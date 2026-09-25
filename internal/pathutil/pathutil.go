package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindBaseDir busca, a partir de cada diretório inicial, um ancestral que
// contenha rules/global-rules.md.
func FindBaseDir(starts []string) (string, bool) {
	for _, start := range starts {
		if start == "" {
			continue
		}
		dir, err := filepath.Abs(start)
		if err != nil {
			continue
		}
		for {
			if _, err := os.Stat(filepath.Join(dir, "rules", "global-rules.md")); err == nil {
				return dir, true
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", false
}

// ResolveBaseDir determina a raiz do repositório na ordem: AGENT_SYNC_HOME,
// diretório do executável e diretório de trabalho atual. Falha com erro
// quando nenhum start contém rules/global-rules.md (fail-fast, postmortem
// 2026-09-23-skills-truncation-and-hook-paths.md P2): antes deste fix,
// o fallback silencioso para cwd permitia que `agent-sync -apply` rodado
// da $HOME gravasse paths /home/<user>/hooks/... em ~/.claude/settings.json.
func ResolveBaseDir(exePath, cwd, envHome string) (string, error) {
	var starts []string
	if envHome != "" {
		starts = append(starts, envHome)
	}
	if exePath != "" {
		starts = append(starts, filepath.Dir(exePath))
	}
	if cwd != "" {
		starts = append(starts, cwd)
	}
	if dir, ok := FindBaseDir(starts); ok {
		return dir, nil
	}
	return "", fmt.Errorf(
		"agent-sync: rules/global-rules.md nao encontrado a partir de %v; execute dentro do repo ou defina AGENT_SYNC_HOME",
		starts)
}

// HookScriptPath resolve hooks/<scriptName> relativo a baseDir e aborta
// com erro se o arquivo nao existir. Single source of truth para evitar
// a regressao de 2026-09-23 (settings.json com /home/matheus_dutra/hooks/...
// fantasma quando apply rodou de cwd sem ser o repo, gravado sem validacao).
func HookScriptPath(baseDir, scriptName string) (string, error) {
	p := filepath.Join(baseDir, "hooks", scriptName)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("hook script nao encontrado: %s", p)
	}
	return p, nil
}

// IsProtectedSkillsDir evita sobrescrever a árvore git de plugins de terceiros
// (qualquer caminho que contenha o par de segmentos "config/plugins") e o
// diretório reservado de skills built-in da Cursor (~/.cursor/skills-cursor).
func IsProtectedSkillsDir(dir string) bool {
	slash := filepath.ToSlash(filepath.Clean(dir))
	if strings.Contains(slash, "/.cursor/skills-cursor") || strings.HasSuffix(slash, "/skills-cursor") {
		return true
	}
	parts := strings.Split(slash, "/")
	for i := 0; i+1 < len(parts); i++ {
		if (parts[i] == "config" || parts[i] == ".config") && parts[i+1] == "plugins" {
			return true
		}
	}
	return false
}

// SessionStateDirName é o diretório canônico onde vive o estado e logs locais da sessão.
const SessionStateDirName = ".agent-sync"

// EnsureStateDir garante que o diretório .agent-sync exista na raiz informada.
func EnsureStateDir(projectRoot string) error {
	dir := filepath.Join(projectRoot, SessionStateDirName)
	return os.MkdirAll(dir, 0o755)
}

// ResolveStateRoot determina a raiz do projeto para comandos de estado/evento/tasks.
func ResolveStateRoot(override string) (string, error) {
	if override != "" {
		if abs, err := filepath.Abs(override); err == nil {
			return abs, nil
		}
	}
	exePath, _ := os.Executable()
	cwd, _ := os.Getwd()
	return ResolveBaseDir(exePath, cwd, os.Getenv("AGENT_SYNC_HOME"))
}


