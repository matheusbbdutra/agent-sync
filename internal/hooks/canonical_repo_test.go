package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateConfigHome isola ~/.config do teste: o repo canônico mora em
// <config>/agent-sync/config.json. Sem isso a suíte escreveria no config real
// do usuário — que contém segredo (turso.token).
func isolateConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

// repoFake cria um diretório que passa em isAgentSyncRepo (marcador
// rules/global-rules.md), simulando um segundo repo/worktree.
func repoFake(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rules", "global-rules.md"), []byte("# regras\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readConfigMap(t *testing.T) map[string]any {
	t.Helper()
	path, err := CanonicalRepoConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler config: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("config inválido: %v", err)
	}
	return doc
}

func TestAdoptCanonicalRegistraPrimeiroApply(t *testing.T) {
	isolateConfigHome(t)
	baseDir := repoBaseDir(t)

	if got := readCanonicalRepo(); got != "" {
		t.Fatalf("canônico deveria começar vazio, obteve %q", got)
	}
	adoption, err := adoptCanonicalBaseDir(baseDir)
	if err != nil {
		t.Fatalf("adoptCanonicalBaseDir: %v", err)
	}
	if !adoption.Adopted {
		t.Fatalf("primeiro apply deveria adotar o baseDir (note=%q)", adoption.Note)
	}
	if adoption.Caveat {
		t.Errorf("primeiro apply não é caveat: %q", adoption.Note)
	}
	if got := readCanonicalRepo(); got != baseDir {
		t.Errorf("canônico registrado = %q, esperado %q", got, baseDir)
	}
	if !strings.Contains(adoption.Note, "primeiro apply") {
		t.Errorf("note deveria mencionar o primeiro apply: %q", adoption.Note)
	}
}

func TestAdoptCanonicalIdempotenteParaMesmoBaseDir(t *testing.T) {
	isolateConfigHome(t)
	baseDir := repoBaseDir(t)

	if _, err := adoptCanonicalBaseDir(baseDir); err != nil {
		t.Fatal(err)
	}
	adoption, err := adoptCanonicalBaseDir(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if !adoption.Adopted {
		t.Fatal("mesmo baseDir deveria continuar adotado")
	}
	if adoption.Note != "" {
		t.Errorf("reapply do canônico não deveria emitir nota: %q", adoption.Note)
	}
}

func TestAdoptCanonicalRecusaOutroRepo(t *testing.T) {
	isolateConfigHome(t)
	canonical := repoBaseDir(t)
	if _, err := adoptCanonicalBaseDir(canonical); err != nil {
		t.Fatal(err)
	}

	outro := repoFake(t)
	adoption, err := adoptCanonicalBaseDir(outro)
	if err != nil {
		t.Fatalf("adoptCanonicalBaseDir: %v", err)
	}
	if adoption.Adopted {
		t.Fatal("não deveria adotar um segundo repo quando já há canônico")
	}
	if !strings.Contains(adoption.Note, canonical) || !strings.Contains(adoption.Note, AllowBasedirEnv) {
		t.Errorf("note deveria citar o canônico e o escape hatch: %q", adoption.Note)
	}
	if got := readCanonicalRepo(); got != canonical {
		t.Errorf("canônico não deveria mudar: %q", got)
	}
}

func TestAdoptCanonicalAutoCuraQuandoObsoleto(t *testing.T) {
	isolateConfigHome(t)
	// Canônico registrado aponta para um diretório que não existe mais
	// (cenário real: worktree removido).
	obsoleto := filepath.Join(t.TempDir(), "worktree-removido")
	if err := writeCanonicalRepo(obsoleto); err != nil {
		t.Fatal(err)
	}

	baseDir := repoBaseDir(t)
	adoption, err := adoptCanonicalBaseDir(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if !adoption.Adopted {
		t.Fatalf("canônico obsoleto deveria ser substituído (note=%q)", adoption.Note)
	}
	if !adoption.Caveat {
		t.Error("substituição de canônico obsoleto deveria ser caveat (⚠️)")
	}
	if got := readCanonicalRepo(); got != baseDir {
		t.Errorf("canônico = %q, esperado %q", got, baseDir)
	}
}

func TestAdoptCanonicalEscapeHatch(t *testing.T) {
	isolateConfigHome(t)
	canonical := repoBaseDir(t)
	if _, err := adoptCanonicalBaseDir(canonical); err != nil {
		t.Fatal(err)
	}

	outro := repoFake(t)
	t.Setenv(AllowBasedirEnv, "1")
	adoption, err := adoptCanonicalBaseDir(outro)
	if err != nil {
		t.Fatal(err)
	}
	if !adoption.Adopted {
		t.Fatal("escape hatch deveria permitir baseDir não canônico")
	}
	if !adoption.Caveat {
		t.Error("escape hatch é caveat (hooks podem rodar 2×)")
	}
	if got := readCanonicalRepo(); got != canonical {
		t.Errorf("escape hatch não deveria re-registrar o canônico: %q", got)
	}
}

func TestWriteCanonicalRepoPreservaCamposExistentes(t *testing.T) {
	configHome := isolateConfigHome(t)
	path := filepath.Join(configHome, "agent-sync", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{"turso":{"url":"libsql://exemplo","token":"segredo"},"summarizer":"auto"}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	baseDir := repoBaseDir(t)
	if err := writeCanonicalRepo(baseDir); err != nil {
		t.Fatal(err)
	}

	doc := readConfigMap(t)
	if got, _ := doc["repo"].(string); got != baseDir {
		t.Errorf("repo = %q, esperado %q", got, baseDir)
	}
	if got, _ := doc["summarizer"].(string); got != "auto" {
		t.Errorf("summarizer perdido no merge: %v", doc["summarizer"])
	}
	turso, ok := doc["turso"].(map[string]any)
	if !ok {
		t.Fatalf("turso perdido no merge: %v", doc)
	}
	if turso["token"] != "segredo" {
		t.Errorf("token perdido no merge: %v", turso)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("modo do config = %o, esperado 600 (contém segredo)", perm)
	}
}

func TestReadCanonicalRepoConfigAusenteECorrompido(t *testing.T) {
	configHome := isolateConfigHome(t)
	if got := readCanonicalRepo(); got != "" {
		t.Errorf("config ausente deveria devolver \"\", obteve %q", got)
	}

	path := filepath.Join(configHome, "agent-sync", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{nao e json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readCanonicalRepo(); got != "" {
		t.Errorf("config corrompido deveria devolver \"\", obteve %q", got)
	}
}

func TestIsAgentSyncRepo(t *testing.T) {
	if !isAgentSyncRepo(repoBaseDir(t)) {
		t.Error("repo real deveria ser reconhecido")
	}
	if isAgentSyncRepo(t.TempDir()) {
		t.Error("diretório vazio não é repo agent-sync")
	}
	if isAgentSyncRepo("") {
		t.Error("vazio não é repo agent-sync")
	}
	// Diretório dentro do repo também conta (FindBaseDir sobe a árvore).
	if !isAgentSyncRepo(filepath.Join(repoBaseDir(t), "internal")) {
		t.Error("subdiretório do repo deveria ser reconhecido")
	}
}
