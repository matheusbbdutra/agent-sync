package hooks

// canonical_repo.go: registro do repo canônico do agent-sync (A-84).
//
// Contexto (Pendência 4): o diretório do plugin Cline é
// <configDir>/plugins/_installed/local/agent-sync-hooks-<sha256(baseDir)[:12]>.
// Como o nome deriva do baseDir, `agent-sync -apply` rodado de OUTRO baseDir
// (ex.: um git worktree) não substitui o plugin — cria um segundo. O CLI do
// Cline carrega todos os diretórios de _installed/local (u9/i9 do binário
// v3.0.65, sem dedup por nome), então cada hook passa a rodar 2×; e o plugin do
// worktree aponta para um baseDir que pode deixar de existir, caso em que
// cline-plugin/index.js engole o erro e vira no-op silencioso.
//
// Regra: o primeiro apply registra o baseDir canônico em
// ~/.config/agent-sync/config.json (campo "repo"). Applies subsequentes de
// outro baseDir avisam e NÃO adotam (nem criam nem podam), para não duplicar
// hooks. Escape hatch explícito: AGENT_SYNC_ALLOW_BASEDIR=1.
//
// O campo é gravado por merge, preservando os demais (turso.token, summarizer,
// projects) — mesmo cuidado de agentmemory.EnsureConfig, já que o arquivo
// contém segredo (por isso o modo 0600).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

const (
	// canonicalRepoKey é o campo gravado no config global do agent-sync.
	canonicalRepoKey = "repo"
	// AllowBasedirEnv é o escape hatch para wiramento de um baseDir não canônico.
	AllowBasedirEnv = "AGENT_SYNC_ALLOW_BASEDIR"
)

// CanonicalRepoConfigPath devolve ~/.config/agent-sync/config.json. Respeita
// XDG_CONFIG_HOME (os.UserConfigDir), o que mantém os testes herméticos.
func CanonicalRepoConfigPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("repo canônico: localizar diretório de configuração: %w", err)
	}
	return filepath.Join(root, "agent-sync", "config.json"), nil
}

// allowNonCanonicalBaseDir informa se o escape hatch está ligado.
func allowNonCanonicalBaseDir() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(AllowBasedirEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// readCanonicalRepo lê o repo canônico registrado. Devolve "" quando ausente,
// ilegível ou corrompido (nunca falha: o chamador decide o que fazer).
func readCanonicalRepo() string {
	path, err := CanonicalRepoConfigPath()
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	repo, _ := doc[canonicalRepoKey].(string)
	return strings.TrimSpace(repo)
}

// writeCanonicalRepo grava o repo canônico preservando os campos existentes
// (merge via map, como agentmemory.EnsureConfig). O arquivo é escrito com 0600
// por conter segredo (turso.token).
func writeCanonicalRepo(repo string) error {
	path, err := CanonicalRepoConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("repo canônico: criar diretório de configuração: %w", err)
	}

	doc := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		// JSON corrompido vira mapa vazio (não propagamos o lixo); o arquivo
		// anterior só é substituído no rename atômico abaixo.
		_ = json.Unmarshal(raw, &doc)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	doc[canonicalRepoKey] = repo

	merged, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("repo canônico: serializar configuração: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(merged, '\n'), 0o600); err != nil {
		return fmt.Errorf("repo canônico: escrever configuração: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("repo canônico: gravar configuração: %w", err)
	}
	return nil
}

// isAgentSyncRepo confirma que dir é um repo agent-sync (ou está dentro de um),
// usando o mesmo marcador de pathutil.FindBaseDir: rules/global-rules.md.
func isAgentSyncRepo(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "rules", "global-rules.md")); err == nil {
		return true
	}
	_, ok := pathutil.FindBaseDir([]string{dir})
	return ok
}

// canonicalAdoption é o resultado da decisão de adoção do baseDir.
type canonicalAdoption struct {
	Adopted bool   // este baseDir pode virar o repo do plugin
	Note    string // mensagem para o usuário ("" = nada a dizer)
	Caveat  bool   // true = imprimir como ⚠️; false = informativo (ℹ️)
}

// adoptCanonicalBaseDir decide se este apply pode adotar baseDir como o repo do
// plugin. No primeiro apply registra o canônico; se o canônico registrado já
// não existe (ex.: worktree removido), re-registra (auto-cura). Adopted=false
// quando outro repo canônico está ativo.
func adoptCanonicalBaseDir(baseDir string) (canonicalAdoption, error) {
	if allowNonCanonicalBaseDir() {
		return canonicalAdoption{
			Adopted: true,
			Note:    AllowBasedirEnv + "=1: baseDir não canônico permitido; plugins coexistindo rodam cada hook 2×",
			Caveat:  true,
		}, nil
	}

	canonical := readCanonicalRepo()
	switch {
	case canonical == "":
		if err := writeCanonicalRepo(baseDir); err != nil {
			return canonicalAdoption{}, err
		}
		return canonicalAdoption{
			Adopted: true,
			Note:    "primeiro apply: " + baseDir + " registrado como repo canônico",
		}, nil
	case canonical == baseDir:
		return canonicalAdoption{Adopted: true}, nil
	case !isAgentSyncRepo(canonical):
		// Canônico obsoleto (diretório removido/renomeado): assume o atual.
		if err := writeCanonicalRepo(baseDir); err != nil {
			return canonicalAdoption{}, err
		}
		return canonicalAdoption{
			Adopted: true,
			Note:    fmt.Sprintf("canônico anterior obsoleto (%s): re-registrado para %s", canonical, baseDir),
			Caveat:  true,
		}, nil
	default:
		return canonicalAdoption{
			Note: fmt.Sprintf("repo canônico é %s; este apply roda de %s — use %s=1 para forçar",
				canonical, baseDir, AllowBasedirEnv),
		}, nil
	}
}
