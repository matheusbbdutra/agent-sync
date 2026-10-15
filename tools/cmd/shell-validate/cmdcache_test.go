package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeSecretScan escreve um `secret-scan` de mentira em disco e devolve o
// caminho. Saídas controladas por FAKE_REDACT/FAKE_CHECK; FAKE_EXIT != 0
// simula binário quebrado (para provar o fail-closed da escrita).
func fakeSecretScan(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "secret-scan")
	script := `#!/usr/bin/env bash
cat >/dev/null
if [ "${FAKE_EXIT:-0}" != "0" ]; then exit "${FAKE_EXIT}"; fi
case "$1" in
  redact) out="${FAKE_REDACT}"; [ -z "$out" ] && out='{"action":"none"}';;
  check) out="${FAKE_CHECK}"; [ -z "$out" ] && out='{"decision":"allow"}';;
  *) out='{}';;
esac
printf '%s\n' "$out"
`
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("fake secret-scan: %v", err)
	}
	return p
}

// setupCache aponta o cache para um dir temporário e instala o scanner falso.
func setupCache(t *testing.T) (cacheDir string, scanner string) {
	t.Helper()
	cacheDir = t.TempDir()
	t.Setenv("AGENT_SYNC_CMD_CACHE_DIR", cacheDir)
	scanner = fakeSecretScan(t)
	t.Setenv("AGENT_SYNC_SECRET_SCAN", scanner)
	t.Setenv("AGENT_SYNC_CMD_CACHE", "1")
	return cacheDir, scanner
}

func TestClassifyCacheableAllowlist(t *testing.T) {
	// cross-platform/read-only determinístico (§1.4)
	for _, cmd := range []string{
		"lscpu",
		"lscpu -e",
		"uname -a",
		"uname -r",
		"cat /etc/os-release",
		"git log -5",
		"git log --oneline -10",
		"git log --pretty=%h -3",
		"git log HEAD~2..HEAD",
	} {
		if _, ok := classifyCacheable(cmd); !ok {
			t.Errorf("deveria ser cacheável: %q", cmd)
		}
	}
	// meta de shell real (§1.3) + fora do allowlist (§1.4)
	for _, cmd := range []string{
		"",
		"   ",
		"git log | head",
		"git log > /tmp/out.txt",
		"git log < in.txt",
		"lscpu && rm -rf /tmp/x",
		"cat /etc/os-release; rm -rf /tmp/x",
		"lscpu &",
		"lscpu $(hostname)",
		"lscpu `id`",
		"git log --format=\"%H\"",
		"cat /etc/os-release\nrm -rf /tmp/x",
		"env",
		"printenv",
		"journalctl -n 35",
		"dmesg",
		"yay -Ss foo",
		"paru -Ss foo",
		"git status",
		"cat /etc/passwd",
		"curl https://example.com",
	} {
		if p, ok := classifyCacheable(cmd); ok {
			t.Errorf("NÃO deveria ser cacheável: %q (kind=%s)", cmd, p.kind)
		}
	}
}

func TestClassifyCacheableTTLByKind(t *testing.T) {
	static, ok := classifyCacheable("uname -a")
	if !ok || static.kind != "static" || static.ttl != time.Duration(cacheDefaultTTLStatic)*time.Second {
		t.Fatalf("kind/ttl de uname errado: %+v ok=%v", static, ok)
	}
	repo, ok := classifyCacheable("git log -1")
	if !ok || repo.kind != "repo" || repo.ttl != time.Duration(cacheDefaultTTLRepo)*time.Second {
		t.Fatalf("kind/ttl de git log errado: %+v ok=%v", repo, ok)
	}
}

func TestCacheKeyDependsOnCmdAndCwd(t *testing.T) {
	setupCache(t)
	a := cacheKey("git log -5")
	if len(a) != 32 {
		t.Fatalf("chave deveria ter 32 hex chars, tem %d", len(a))
	}
	if b := cacheKey("git log -6"); a == b {
		t.Fatal("comandos diferentes não podem colidir na chave")
	}
	dirA := t.TempDir()
	dirB := t.TempDir()
	t.Chdir(dirA)
	ka := cacheKey("git log -5")
	t.Chdir(dirB)
	kb := cacheKey("git log -5")
	if ka == kb {
		t.Fatal("cwd diferente deveria gerar chave diferente (uid+cwd+cmd)")
	}
}

func TestCachePutGetAndTTLExpiry(t *testing.T) {
	cacheDir, _ := setupCache(t)
	cmd := "git log -5"
	policy, ok := classifyCacheable(cmd)
	if !ok {
		t.Fatal("comando de teste fora da allowlist")
	}
	if err := cmdCachePut(policy, cmd, "abc123\n"); err != nil {
		t.Fatalf("put: %v", err)
	}
	out, age, hit := cmdCacheGet(policy, cmd)
	if !hit || out != "abc123\n" {
		t.Fatalf("esperava hit com a saída gravada; hit=%v out=%q", hit, out)
	}
	if age < 0 || age > time.Minute {
		t.Fatalf("idade inesperada: %v", age)
	}
	// expiração por mtime (§1.5)
	path := filepath.Join(cacheDir, cacheKey(cmd)+".out")
	old := time.Now().Add(-2 * policy.ttl)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if _, _, hit := cmdCacheGet(policy, cmd); hit {
		t.Fatal("entrada expirada não pode dar hit")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("entrada expirada deveria ter sido removida")
	}
}

func TestCachePutRefusesEmptyAndOversize(t *testing.T) {
	setupCache(t)
	policy, _ := classifyCacheable("uname -a")
	if err := cmdCachePut(policy, "uname -a", "   \n"); err == nil {
		t.Fatal("saída vazia não pode ser cacheada")
	}
	big := strings.Repeat("x", cacheMaxEntryBytes+1)
	if err := cmdCachePut(policy, "uname -a", big); err == nil {
		t.Fatal("saída acima do cap não pode ser cacheada")
	}
}

func TestCachePutRefusesSecretAndFailsClosed(t *testing.T) {
	cacheDir, _ := setupCache(t)
	policy, _ := classifyCacheable("uname -a")

	// scanner sinaliza segredo literal na saída
	t.Setenv("FAKE_REDACT", `{"action":"redact-inline","text":"<REDACTED:JWT>"}`)
	err := cmdCachePut(policy, "uname -a", "linha com segredo")
	if err == nil || !strings.Contains(err.Error(), "secret-scan") {
		t.Fatalf("esperava recusa por segredo; err=%v", err)
	}
	// scanner sinaliza path da deny-list via check
	t.Setenv("FAKE_REDACT", `{"action":"none"}`)
	t.Setenv("FAKE_CHECK", `{"decision":"deny","reason":"path denylist"}`)
	if err := cmdCachePut(policy, "uname -a", "conteudo qualquer"); err == nil {
		t.Fatal("esperava recusa quando check=deny")
	}
	// scanner quebrado = fail-closed
	t.Setenv("FAKE_CHECK", `{"decision":"allow"}`)
	t.Setenv("FAKE_EXIT", "3")
	if err := cmdCachePut(policy, "uname -a", "conteudo"); err == nil {
		t.Fatal("scanner com erro deveria bloquear a escrita (fail-closed)")
	}
	t.Setenv("FAKE_EXIT", "0")
	// nenhum scanner disponível = fail-closed
	t.Setenv("AGENT_SYNC_SECRET_SCAN", "")
	t.Setenv("PATH", "")
	if err := cmdCachePut(policy, "uname -a", "conteudo"); err == nil {
		t.Fatal("sem secret-scan não deve gravar nada")
	}
	if entries, _ := os.ReadDir(cacheDir); len(entries) != 0 {
		t.Fatalf("nada deveria ter sido gravado; dir tem %d entradas", len(entries))
	}
}

func TestCacheDirIs0700AndFixedWhenLoose(t *testing.T) {
	cacheDir, _ := setupCache(t)
	// simula diretório pré-existente permissivo
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(cacheDir, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	policy, _ := classifyCacheable("uname -a")
	if err := cmdCachePut(policy, "uname -a", "Linux\n"); err != nil {
		t.Fatalf("put: %v", err)
	}
	fi, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("dir do cache deveria ser 0700, é %o", fi.Mode().Perm())
	}
	entry, err := os.ReadFile(filepath.Join(cacheDir, cacheKey("uname -a")+".out"))
	if err != nil || string(entry) != "Linux\n" {
		t.Fatalf("entrada do cache inesperada: %q err=%v", entry, err)
	}
}

func TestCompressForContextTruncates(t *testing.T) {
	long := strings.Repeat("linha\n", cacheMaxInjectLines+20)
	text, truncated := compressForContext(long)
	if !truncated {
		t.Fatal("esperava truncamento por linhas")
	}
	if got := strings.Count(text, "\n"); got > cacheMaxInjectLines {
		t.Fatalf("linhas acima do teto: %d", got)
	}
	huge := strings.Repeat("x", cacheMaxInjectBytes+100)
	text, truncated = compressForContext(huge)
	if !truncated || len(text) != cacheMaxInjectBytes {
		t.Fatalf("esperava corte por bytes; len=%d truncated=%v", len(text), truncated)
	}
}

func TestServeCacheHitHookResponses(t *testing.T) {
	setupCache(t)
	policy, _ := classifyCacheable("uname -a")
	if err := cmdCachePut(policy, "uname -a", "Linux host 6.1\n"); err != nil {
		t.Fatalf("put: %v", err)
	}
	payload := `{"tool_name":"Bash","tool_input":{"command":"uname -a"},"session_id":"s1"}`
	var out bytes.Buffer
	if err := runHook(strings.NewReader(payload), &out, os.Stderr); err != nil {
		t.Fatalf("runHook: %v", err)
	}
	if !strings.Contains(out.String(), "cmd-cache HIT") || !strings.Contains(out.String(), "Linux host 6.1") {
		t.Fatalf("esperava injeção do cache; saída=%s", out.String())
	}
	// payload Antigravity: envelope decision, sem additionalContext
	agy := `{"toolCall":{"name":"Bash","args":{"command":"uname -a"}},"invocationNum":7}`
	out.Reset()
	if err := runHook(strings.NewReader(agy), &out, os.Stderr); err != nil {
		t.Fatalf("runHook agy: %v", err)
	}
	if out.String() != `{"decision":"allow"}` {
		t.Fatalf("Antigravity deveria receber decision=allow; got=%s", out.String())
	}
	// comando fora da allowlist segue o caminho normal (sem cache)
	out.Reset()
	if err := runHook(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls -la"}}`), &out, os.Stderr); err != nil {
		t.Fatalf("runHook: %v", err)
	}
	if out.String() != "{}" {
		t.Fatalf("comando fora da allowlist deveria passar; got=%s", out.String())
	}
}

func TestCacheDisabledByDefault(t *testing.T) {
	setupCache(t)
	t.Setenv("AGENT_SYNC_CMD_CACHE", "0")
	policy, _ := classifyCacheable("uname -a")
	if err := cmdCachePut(policy, "uname -a", "Linux\n"); err != nil {
		t.Fatalf("put: %v", err)
	}
	var out bytes.Buffer
	if err := runHook(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"uname -a"}}`), &out, os.Stderr); err != nil {
		t.Fatalf("runHook: %v", err)
	}
	if out.String() != "{}" {
		t.Fatalf("sem AGENT_SYNC_CMD_CACHE=1 o hook não pode mexer no fluxo; got=%s", out.String())
	}
}

func TestCachePutFromPostToolUsePayload(t *testing.T) {
	setupCache(t)
	payload := `{"tool_name":"Bash","tool_input":{"command":"git log -3"},"tool_response":{"stdout":"commit aaa\ncommit bbb\n"}}`
	var out bytes.Buffer
	if err := runCachePut([]string{"--from-hook"}, strings.NewReader(payload), &out, os.Stderr); err != nil {
		t.Fatalf("cache-put --from-hook: %v", err)
	}
	policy, _ := classifyCacheable("git log -3")
	got, _, hit := cmdCacheGet(policy, "git log -3")
	if !hit || got != "commit aaa\ncommit bbb\n" {
		t.Fatalf("esperava a saída do payload no cache; hit=%v got=%q", hit, got)
	}
	// comando fora da allowlist = erro, nada gravado
	var out2 bytes.Buffer
	if err := runCachePut([]string{"--from-hook"}, strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/x"},"tool_response":"x"}`), &out2, os.Stderr); err == nil {
		t.Fatal("comando fora da allowlist deveria falhar no cache-put")
	}
}
