// cmdcache.go — estágio "overwrite + cache TTL" (ADR-reuso-bash-sudo-agents §1).
//
// Para um allowlist CONSERVADORA de comandos read-only determinísticos
// (lscpu, uname, cat /etc/os-release, git log), guarda o stdout em cache com
// TTL e, no próximo PreToolUse DO MESMO comando no MESMO cwd/uid, injeta o
// resultado já obtido como additionalContext — o agente não precisa executar
// de novo (economia de tokens no re-run).
//
// Invariantes (§1 da ADR, as 6 correções obrigatórias):
//  1. diretório 0700 / arquivo 0600; NUNCA grava saída que o secret-scan
//     sinaliza (a deny-list dele é a fonte única — aqui não há cópia).
//  2. chave = sha256(uid + cwd + cmd), com cap de tamanho por entrada.
//  3. detecção de redirect/pipe distingue `>` de `&&`: meta de shell real
//     (`>`, `|`, `<`, `;`, `&`, crase, `$(`) bloqueia; `git log A && lscpu`
//     não bloqueia por causa do `&&`. (O compressor do original usava
//     `[>|&]` e desistia em `a && b`.)
//  4. allowlist cross-platform/read-only: sem `yay/paru -Ss`, sem
//     `journalctl`/`dmesg` como default cego.
//  5. TTL obrigatório, expiração por mtime; nada de cache com pipe/redirect.
//  6. o compressor não reescreve o comando do usuário (fail-open): o hook só
//     ADICIONA contexto ou passa; nunca muda o que o agente vai executar.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// cap por entrada (§1.2): saída maior não é cacheada.
	cacheMaxEntryBytes = 64 << 10 // 64 KiB
	// teto do que volta ao contexto ("compressor" determinístico, sem LLM).
	cacheMaxInjectBytes = 8 << 10 // 8 KiB
	cacheMaxInjectLines = 120
	// TTLs default (segundos): estado de máquina muda pouco, estado de repo não.
	cacheDefaultTTLStatic = 3600
	cacheDefaultTTLRepo   = 60
)

// cachePolicy é uma entrada do allowlist + o TTL aplicado a ela.
type cachePolicy struct {
	kind string
	ttl  time.Duration
}

// shellMetaChars: meta de shell que muda o significado do comando (redirect,
// pipe, encadeamento, substituição, aspas, barra invertida).
var shellMetaChars = ">|<;&`\n\\\"'"

// validFlagToken valida cada token depois do binário: sem aspas, sem `$`/`*`,
// sem espaço. Evita cachear comando com expansão/globbing (o output mudaria
// sem o texto do comando mudar).
func validFlagToken(tok string) bool {
	if tok == "" {
		return false
	}
	for _, r := range tok {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.', r == ',', r == ':', r == '/', r == '=', r == '@', r == '+', r == '%', r == '~', r == '^':
		default:
			return false
		}
	}
	return true
}

func hasShellMeta(cmd string) bool {
	return strings.ContainsAny(cmd, shellMetaChars)
}

// classifyCacheable devolve a política de cache quando o comando é 1 comando
// exato do allowlist; caso contrário ok=false (fail-open: segue sem cache).
func classifyCacheable(cmd string) (cachePolicy, bool) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" || hasShellMeta(trimmed) {
		return cachePolicy{}, false
	}
	tokens := strings.Fields(trimmed)
	for _, tok := range tokens {
		if !validFlagToken(tok) {
			return cachePolicy{}, false
		}
	}
	static := cachePolicy{kind: "static", ttl: ttlFromEnv(cacheDefaultTTLStatic)}
	repo := cachePolicy{kind: "repo", ttl: ttlFromEnv(cacheDefaultTTLRepo)}

	switch tokens[0] {
	case "lscpu", "uname":
		return static, true
	case "cat":
		// Somente o arquivo canônico de identidade da distro (read-only, texto).
		if len(tokens) == 2 && tokens[1] == "/etc/os-release" {
			return static, true
		}
	case "git":
		if len(tokens) >= 2 && tokens[1] == "log" {
			return repo, true
		}
	}
	return cachePolicy{}, false
}

// ttlFromEnv permite calibrar o TTL sem recompilar.
func ttlFromEnv(def int) time.Duration {
	if v := strings.TrimSpace(os.Getenv("AGENT_SYNC_CMD_CACHE_TTL")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return time.Duration(def) * time.Second
}

// cmdCacheEnabled: o estágio é opt-in (mesmo padrão do
// AGENT_SYNC_PRETOOLUSE_VALIDATE). Sem a flag: zero custo e zero escrita.
func cmdCacheEnabled() bool {
	return strings.TrimSpace(os.Getenv("AGENT_SYNC_CMD_CACHE")) == "1"
}

func cmdCacheDir() string {
	if d := strings.TrimSpace(os.Getenv("AGENT_SYNC_CMD_CACHE_DIR")); d != "" {
		return d
	}
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "agent-sync", "cmd-cache")
}

// ensureCacheDir cria o diretório 0700 e reaplica o modo se já existir mais
// permissivo — correção obrigatória §1.1.
func ensureCacheDir() (string, error) {
	dir := cmdCacheDir()
	if dir == "" {
		return "", fmt.Errorf("cmd-cache: diretório indisponível")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// cacheKey = sha256(uid + cwd + cmd) (§1.2: uid entra na chave para que caches
// de usuários diferentes no mesmo cwd não colidam).
func cacheKey(cmd string) string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "?"
	}
	sum := sha256.Sum256([]byte(strconv.Itoa(os.Getuid()) + "\x00" + cwd + "\x00" + strings.TrimSpace(cmd)))
	return hex.EncodeToString(sum[:16])
}

// cmdCacheGet devolve o conteúdo se estiver dentro do TTL (mtime). Entrada
// expirada é removida (higiene) e tratada como miss.
func cmdCacheGet(p cachePolicy, cmd string) (string, time.Duration, bool) {
	dir, err := ensureCacheDir()
	if err != nil {
		return "", 0, false
	}
	path := filepath.Join(dir, cacheKey(cmd)+".out")
	fi, err := os.Stat(path)
	if err != nil {
		return "", 0, false
	}
	age := time.Since(fi.ModTime())
	if age > p.ttl {
		_ = os.Remove(path)
		return "", 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return "", 0, false
	}
	return string(data), age, true
}

// cmdCachePut grava a saída. Fail-closed na escrita: sem secret-scan
// disponível NÃO grava (melhor perder o cache que vazar segredo em disco,
// §1.1). Entrada acima do cap também não é gravada.
func cmdCachePut(p cachePolicy, cmd, out string) error {
	if strings.TrimSpace(out) == "" {
		return fmt.Errorf("cmd-cache: saída vazia")
	}
	if len(out) > cacheMaxEntryBytes {
		return fmt.Errorf("cmd-cache: saída acima do cap (%d > %d bytes)", len(out), cacheMaxEntryBytes)
	}
	if flagged, why := secretScanFlags(out); flagged {
		return fmt.Errorf("cmd-cache: não cacheada — %s", why)
	}
	dir, err := ensureCacheDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, cacheKey(cmd)+".out")
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(out); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}

// secretScanFlags reaproveita o secret-scan (fonte única da deny-list) para
// decidir se a saída pode ir para o disco: `redact` pega segredo literal no
// texto, `check` pega path da deny-list presente na saída. Qualquer um
// sinalizando = não cacheia. Binário ausente = não cacheia (fail-closed).
func secretScanFlags(out string) (bool, string) {
	bin := secretScanBinary()
	if bin == "" {
		return true, "secret-scan indisponível (fail-closed)"
	}
	redactPayload := jsonPayload(map[string]any{"tool_name": "Bash", "tool_response": out})
	res, err := runSecretScan(bin, "redact", redactPayload)
	if err != nil {
		return true, "secret-scan redact falhou (" + err.Error() + ") — fail-closed"
	}
	if res != "" && res != "none" {
		return true, "secret-scan redact=" + res
	}
	checkPayload := jsonPayload(map[string]any{"tool_name": "Bash", "tool_input": map[string]string{"command": out}})
	res, err = runSecretScan(bin, "check", checkPayload)
	if err != nil {
		return true, "secret-scan check falhou (" + err.Error() + ") — fail-closed"
	}
	if res == "deny" {
		return true, "secret-scan check=deny"
	}
	return false, ""
}

// jsonPayload serializa o payload sintético enviado ao secret-scan.
func jsonPayload(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// runSecretScan roda `secret-scan <sub> --format decision` (envelope neutro) e
// devolve action|decision. Erro = o chamador decide (aqui, fail-closed).
func runSecretScan(bin, sub, payload string) (string, error) {
	c := exec.Command(bin, sub, "--format", "decision")
	c.Stdin = strings.NewReader(payload)
	raw, err := c.Output()
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return "", err
	}
	var env struct {
		Action   string `json:"action"`
		Decision string `json:"decision"`
	}
	if jerr := json.Unmarshal([]byte(body), &env); jerr != nil {
		return "", jerr
	}
	if env.Action != "" {
		return env.Action, nil
	}
	if env.Decision != "" {
		return env.Decision, nil
	}
	return "", err
}

// secretScanBinary resolve o binário irmão (bin/) ou o do PATH.
func secretScanBinary() string {
	if p := strings.TrimSpace(os.Getenv("AGENT_SYNC_SECRET_SCAN")); p != "" {
		return p
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "secret-scan")
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
	}
	if p, err := exec.LookPath("secret-scan"); err == nil {
		return p
	}
	return ""
}

// compressForContext limita o que volta ao contexto (linhas + bytes).
func compressForContext(out string) (string, bool) {
	truncated := false
	lines := strings.Split(out, "\n")
	if len(lines) > cacheMaxInjectLines {
		lines = lines[:cacheMaxInjectLines]
		truncated = true
	}
	text := strings.Join(lines, "\n")
	if len(text) > cacheMaxInjectBytes {
		text = text[:cacheMaxInjectBytes]
		truncated = true
	}
	return text, truncated
}
