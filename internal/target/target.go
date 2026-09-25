package target

import (
	"fmt"
	"os"
	"path/filepath"
)

// TargetCLI descreve o schema de instalacao de um CLI alvo (Claude Code,
// Codex, Antigravity, OpenCode, Cursor). Cada campo vira path no filesystem
// do usuario.
type TargetCLI struct {
	Name              string
	RulesPath         string
	SkillsDir         string
	AgentsDir         string
	AgentKind         string
	PluginDir         string
	HooksSettingsPath string
	HooksEvent        string
	HooksFormat       string // "" (padrao Claude/Codex), "antigravity" ou "cursor"
	OpenCodePluginDir string
}

// Target é um alias para TargetCLI.
type Target = TargetCLI

// GetHome retorna o home do usuario, falhando ruidosamente se nao conseguir.
func GetHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao obter Home: %v\n", err)
		os.Exit(1)
	}
	return home
}

// GetTargets retorna os 5 CLI targets wiraveis pelo agent-sync.
func GetTargets() []TargetCLI {
	return GetTargetsForHome(GetHome())
}

// GetTargetsForHome retorna os 5 CLI targets usando o home directory fornecido.
func GetTargetsForHome(home string) []TargetCLI {
	return []TargetCLI{
		{
			Name:              "claude",
			RulesPath:         filepath.Join(home, ".claude", "AGENTS.md"),
			SkillsDir:         filepath.Join(home, ".claude", "skills"),
			AgentsDir:         filepath.Join(home, ".claude", "agents"),
			AgentKind:         "claude",
			HooksSettingsPath: filepath.Join(home, ".claude", "settings.json"),
			HooksEvent:        "PostToolUse",
		},
		{
			Name:              "codex",
			RulesPath:         filepath.Join(home, ".codex", "AGENTS.md"),
			SkillsDir:         filepath.Join(home, ".codex", "skills"),
			AgentsDir:         filepath.Join(home, ".codex", "agents"),
			AgentKind:         "codex",
			HooksSettingsPath: filepath.Join(home, ".codex", "hooks.json"),
			HooksEvent:        "PostToolUse",
		},
		{
			Name:              "antigravity",
			RulesPath:         filepath.Join(home, ".gemini", "AGENTS.md"),
			SkillsDir:         filepath.Join(home, ".gemini", "antigravity-cli", "skills"),
			AgentsDir:         filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "agent-sync", "agents"),
			AgentKind:         "antigravity",
			PluginDir:         filepath.Join(home, ".gemini", "antigravity-cli", "plugins", "agent-sync"),
			HooksSettingsPath: filepath.Join(home, ".gemini", "config", "hooks.json"),
			HooksEvent:        "PreInvocation",
			HooksFormat:       "antigravity",
		},
		{
			Name:              "opencode",
			RulesPath:         filepath.Join(home, ".config", "opencode", "AGENTS.md"),
			SkillsDir:         filepath.Join(home, ".config", "opencode", "skills"),
			AgentsDir:         filepath.Join(home, ".config", "opencode", "agents"),
			AgentKind:         "opencode",
			OpenCodePluginDir: filepath.Join(home, ".config", "opencode", "plugins"),
		},
		{
			Name:              "cursor",
			RulesPath:         filepath.Join(home, ".cursor", "AGENTS.md"),
			SkillsDir:         filepath.Join(home, ".cursor", "skills"),
			AgentsDir:         filepath.Join(home, ".cursor", "agents"),
			AgentKind:         "cursor",
			HooksSettingsPath: filepath.Join(home, ".cursor", "hooks.json"),
			HooksEvent:        "postToolUse",
			HooksFormat:       "cursor",
		},
	}
}
