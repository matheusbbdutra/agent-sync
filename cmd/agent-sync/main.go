package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type TargetCLI struct {
	Name              string
	RulesPath         string
	SkillsDir         string
	AgentsDir         string
	AgentKind         string
	PluginDir         string
	HooksSettingsPath string
	HooksEvent        string
	HooksFormat       string // "" (padrão Claude/Codex), "antigravity" ou "cursor"
	OpenCodePluginDir string
}

func getHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao obter Home: %v\n", err)
		os.Exit(1)
	}
	return home
}

func getTargets() []TargetCLI {
	home := getHome()
	return []TargetCLI{
		{
			Name:              "claude",
			RulesPath:         filepath.Join(home, ".claude", "CLAUDE.md"),
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
			// Gemini CLI standalone foi descontinuada (18/06/2026) para contas
			// não-enterprise; sucessora é o Antigravity CLI (compatibilidade de
			// regras mantida em ~/.gemini/GEMINI.md).
			Name:              "antigravity",
			RulesPath:         filepath.Join(home, ".gemini", "GEMINI.md"),
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
			// Cursor IDE / agent CLI: skills e agents em ~/.cursor; regras
			// locais em ~/.cursor/rules/*.mdc (não sincronizam via conta);
			// hooks nativos em ~/.cursor/hooks.json (cwd = ~/.cursor/).
			// Nunca tocar em ~/.cursor/skills-cursor/ (built-ins da Cursor).
			Name:              "cursor",
			RulesPath:         filepath.Join(home, ".cursor", "rules", "agent-sync-global.mdc"),
			SkillsDir:         filepath.Join(home, ".cursor", "skills"),
			AgentsDir:         filepath.Join(home, ".cursor", "agents"),
			AgentKind:         "cursor",
			HooksSettingsPath: filepath.Join(home, ".cursor", "hooks.json"),
			HooksEvent:        "postToolUse",
			HooksFormat:       "cursor",
		},
	}
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func syncSkills(srcDir, dstDir string) error {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		sourceSkill := filepath.Join(srcDir, skillName)
		targetSkill := filepath.Join(dstDir, skillName)

		// Remove link antigo se existir para evitar conflitos
		_ = os.Remove(targetSkill)

		// Cria symlink relativo/absoluto para sincronização bidirecional
		if err := os.Symlink(sourceSkill, targetSkill); err != nil {
			// Fallback: cópia direta se symlink falhar
			_ = copyDir(sourceSkill, targetSkill)
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		return copyFile(path, targetPath)
	})
}

// findBaseDir busca, a partir de cada diretório inicial, um ancestral que
// contenha rules/global-rules.md.
func findBaseDir(starts []string) (string, bool) {
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

// resolveBaseDir determina a raiz do repositório na ordem: AGENT_SYNC_HOME,
// diretório do executável e diretório de trabalho atual.
func resolveBaseDir(exePath, cwd, envHome string) string {
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
	if dir, ok := findBaseDir(starts); ok {
		return dir
	}
	if cwd != "" {
		return cwd
	}
	return "."
}

// isProtectedSkillsDir evita sobrescrever a árvore git de plugins de terceiros
// (qualquer caminho que contenha o par de segmentos "config/plugins") e o
// diretório reservado de skills built-in da Cursor (~/.cursor/skills-cursor).
func isProtectedSkillsDir(dir string) bool {
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

func main() {
	applyFlag := flag.Bool("apply", false, "Aplica as regras e skills para todas as CLIs configuradas")
	targetFlag := flag.String("target", "", "Aplica para uma CLI específica (claude, codex, antigravity, opencode, cursor)")
	statusFlag := flag.Bool("status", false, "Exibe o status de sincronização com as CLIs")
	vendorFlag := flag.Bool("vendor", false, "Importa as skills curadas do catálogo definido em skills/manifest.json")
	observabilityFlag := flag.Bool("observability", false, "Exibe resumo dos erros persistidos pelos hooks")
	sourceFlag := flag.String("source", "", "Diretório de origem das skills para -vendor (default: skillsDir do manifest)")
	flag.Parse()

	exePath, _ := os.Executable()
	cwd, _ := os.Getwd()
	baseDir := resolveBaseDir(exePath, cwd, os.Getenv("AGENT_SYNC_HOME"))

	rulesSource := filepath.Join(baseDir, "rules", "global-rules.md")
	skillsSource := filepath.Join(baseDir, "skills")

	if !*applyFlag && !*statusFlag && !*vendorFlag && !*observabilityFlag && *targetFlag == "" {
		fmt.Println("🚀 Agent-Sync: Gerenciador Unificado de Regras e Skills para Agentes AI")
		fmt.Println("\nUso:")
		fmt.Println("  agent-sync -apply              # Sincroniza em todas as CLIs instaladas")
		fmt.Println("  agent-sync -target <cli>       # Sincroniza apenas para claude, codex, antigravity, opencode ou cursor")
		fmt.Println("  agent-sync -status             # Verifica o status atual de cada CLI")
		fmt.Println("  agent-sync -vendor             # Importa as skills curadas do manifest")
		fmt.Println("  agent-sync -observability      # Resume erros persistidos pelos hooks")
		return
	}

	if *vendorFlag {
		if err := runVendor(baseDir, *sourceFlag); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Falha no vendor: %v\n", err)
			os.Exit(1)
		}
		return
	}

	targets := getTargets()
	if *observabilityFlag {
		if err := printHookObservability(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Falha ao ler observabilidade: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *statusFlag {
		fmt.Println("📊 Status de Sincronização:")
		for _, t := range targets {
			rulesStatus := "❌ Não encontrado"
			if _, err := os.Stat(t.RulesPath); err == nil {
				rulesStatus = "✅ Presente"
			}
			skillsCount := 0
			if entries, err := os.ReadDir(t.SkillsDir); err == nil {
				for _, e := range entries {
					if e.IsDir() || (e.Type()&os.ModeSymlink != 0) {
						skillsCount++
					}
				}
			}
			agentsCount := 0
			if entries, err := os.ReadDir(t.AgentsDir); err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						agentsCount++
					}
				}
			}
			fmt.Printf(" - %-11s | Regras: %-16s | Skills: %d instaladas | Agentes: %d\n", t.Name, rulesStatus, skillsCount, agentsCount)
		}
		return
	}

	fmt.Println("🔄 Iniciando sincronização...")
	count := 0
	for _, t := range targets {
		if *targetFlag != "" && *targetFlag != t.Name {
			continue
		}

		// Copia regras (Cursor usa .mdc com alwaysApply)
		var rulesErr error
		if t.AgentKind == "cursor" {
			rulesErr = syncCursorRules(rulesSource, t.RulesPath)
		} else {
			rulesErr = copyFile(rulesSource, t.RulesPath)
		}
		if rulesErr != nil {
			fmt.Printf("⚠️  [%s] Falha ao atualizar regras: %v\n", t.Name, rulesErr)
		} else {
			fmt.Printf("✅ [%s] Regras atualizadas em: %s\n", t.Name, t.RulesPath)
		}

		// Sincroniza skills (protegendo a árvore git de plugins de terceiros)
		if isProtectedSkillsDir(t.SkillsDir) {
			fmt.Printf("⛔ [%s] Destino de skills protegido, ignorado: %s\n", t.Name, t.SkillsDir)
		} else if err := syncSkills(skillsSource, t.SkillsDir); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar skills: %v\n", t.Name, err)
		} else {
			fmt.Printf("✅ [%s] Skills sincronizadas em: %s\n", t.Name, t.SkillsDir)
		}

		// Gera os agentes especialistas no formato nativo da CLI
		if agents, err := syncAgents(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar agentes: %v\n", t.Name, err)
		} else if agents > 0 {
			fmt.Printf("✅ [%s] %d agentes gerados em: %s\n", t.Name, agents, t.AgentsDir)
		}

		if t.HooksFormat == "cursor" {
			if err := syncCursorAll(baseDir, t); err != nil {
				fmt.Printf("⚠️  [%s] Falha ao sincronizar hooks Cursor: %v\n", t.Name, err)
			} else {
				fmt.Printf("✅ [%s] Hooks (context-guard, memory, agent-react, docs-cache, bash-guardian) em: %s\n", t.Name, t.HooksSettingsPath)
			}
			if err := syncCtxCompactHook(baseDir, t); err != nil {
				fmt.Printf("⚠️  [%s] Falha ao sincronizar tracking ctx-window: %v\n", t.Name, err)
			}
			if err := syncCtxHandoffHook(baseDir, t); err != nil {
				fmt.Printf("⚠️  [%s] Falha ao sincronizar handoff ctx-window: %v\n", t.Name, err)
			}
			count++
			continue
		}

		// Instala o hook de lembrete do context-guard (quando suportado pela CLI)
		if err := syncHooks(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar hooks: %v\n", t.Name, err)
		} else if t.HooksSettingsPath != "" {
			fmt.Printf("✅ [%s] Hook de context-guard instalado em: %s\n", t.Name, t.HooksSettingsPath)
		}

		// Instala o lembrete de memory-mcp (store_memory), hoje dependente só
		// da disciplina do modelo.
		if err := syncMemoryNudgeHook(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar hook de memória: %v\n", t.Name, err)
		} else if t.HooksSettingsPath != "" {
			fmt.Printf("✅ [%s] Hook de lembrete de memória instalado em: %s\n", t.Name, t.HooksSettingsPath)
		}

		// Instala o lembrete de validação de hipóteses (agent-react).
		if err := syncAgentReactNudgeHook(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar hook agent-react: %v\n", t.Name, err)
		} else if t.HooksSettingsPath != "" {
			fmt.Printf("✅ [%s] Hook de agent-react instalado em: %s\n", t.Name, t.HooksSettingsPath)
		}

		// Instala o hook de compactação de contexto (ctx-window on-tool-call).
		if err := syncCtxCompactHook(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar hook ctx-compact: %v\n", t.Name, err)
		} else if t.HooksSettingsPath != "" {
			fmt.Printf("✅ [%s] Hook de ctx-compact instalado em: %s\n", t.Name, t.HooksSettingsPath)
		}
		if err := syncCtxHandoffHook(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar handoff ctx-window: %v\n", t.Name, err)
		}

		// Instala o hook de fim de turno (Stop) no Antigravity CLI
		if err := syncStopHook(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar hook de stop: %v\n", t.Name, err)
		} else if t.HooksFormat == "antigravity" && t.HooksSettingsPath != "" {
			fmt.Printf("✅ [%s] Hook de stop instalado em: %s\n", t.Name, t.HooksSettingsPath)
		}

		// Instala o lembrete just-in-time no PreInvocation do Antigravity CLI
		if err := syncPreInvocationReminderHook(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar lembrete preinvocation: %v\n", t.Name, err)
		} else if t.HooksFormat == "antigravity" && t.HooksSettingsPath != "" {
			fmt.Printf("✅ [%s] Lembrete de preinvocation instalado em: %s\n", t.Name, t.HooksSettingsPath)
		}

		// Instala o hook PreToolUse do shell-validate. É opt-in: só ativa
		// quando AGENT_SYNC_PRETOOLUSE_VALIDATE=1 estiver setado no ambiente.
		if err := syncShellValidateHook(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar hook shell-validate: %v\n", t.Name, err)
		} else if t.HooksSettingsPath != "" {
			fmt.Printf("✅ [%s] Hook de shell-validate instalado em: %s\n", t.Name, t.HooksSettingsPath)
		}

		// OpenCode: plugin TS best-effort (ver limitação documentada no hooks.go)
		if err := syncOpenCodePlugin(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar plugin: %v\n", t.Name, err)
		} else if t.OpenCodePluginDir != "" {
			fmt.Printf("✅ [%s] Plugin de context-guard instalado em: %s (best-effort, ver README)\n", t.Name, t.OpenCodePluginDir)
		}
		if err := syncOpenCodeMemoryNudgePlugin(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar plugin de memória: %v\n", t.Name, err)
		} else if t.OpenCodePluginDir != "" {
			fmt.Printf("✅ [%s] Plugin de lembrete de memória instalado em: %s (best-effort, ver README)\n", t.Name, t.OpenCodePluginDir)
		}
		if err := syncOpenCodeAgentReactNudgePlugin(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar plugin agent-react: %v\n", t.Name, err)
		} else if t.OpenCodePluginDir != "" {
			fmt.Printf("✅ [%s] Plugin de agent-react instalado em: %s (best-effort, ver README)\n", t.Name, t.OpenCodePluginDir)
		}
		if err := syncOpenCodeCtxCompactPlugin(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar plugin ctx-compact: %v\n", t.Name, err)
		} else if t.OpenCodePluginDir != "" {
			fmt.Printf("✅ [%s] Plugin de ctx-compact instalado em: %s (best-effort)\n", t.Name, t.OpenCodePluginDir)
		}

		// docs-cache: cacheia passivamente docs consultadas via WebFetch/
		// read_url_content e context7 (query-docs), sem refazer requisição de rede.
		if err := syncDocsCacheHook(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar docs-cache: %v\n", t.Name, err)
		} else if t.HooksSettingsPath != "" {
			fmt.Printf("✅ [%s] docs-cache instalado em: %s\n", t.Name, t.HooksSettingsPath)
		}
		if err := syncOpenCodeDocsCachePlugin(baseDir, t); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar plugin docs-cache: %v\n", t.Name, err)
		} else if t.OpenCodePluginDir != "" {
			fmt.Printf("✅ [%s] Plugin docs-cache instalado em: %s (best-effort)\n", t.Name, t.OpenCodePluginDir)
		}

		// bash-guardian: pede confirmação em comandos de risco conhecido.
		// Codex fica de fora (PreToolUse não suporta "ask", só allow/deny binário).
		switch t.AgentKind {
		case "claude":
			if err := syncBashGuardianClaude(baseDir, t); err != nil {
				fmt.Printf("⚠️  [%s] Falha ao sincronizar bash-guardian: %v\n", t.Name, err)
			} else {
				fmt.Printf("✅ [%s] bash-guardian instalado em: %s\n", t.Name, t.HooksSettingsPath)
			}
		case "antigravity":
			if err := syncBashGuardianAntigravity(baseDir, t); err != nil {
				fmt.Printf("⚠️  [%s] Falha ao sincronizar bash-guardian: %v\n", t.Name, err)
			} else {
				fmt.Printf("✅ [%s] bash-guardian instalado em: %s\n", t.Name, t.HooksSettingsPath)
			}
		case "opencode":
			if err := syncBashGuardianOpenCode(baseDir, t); err != nil {
				fmt.Printf("⚠️  [%s] Falha ao sincronizar bash-guardian: %v\n", t.Name, err)
			} else {
				fmt.Printf("✅ [%s] bash-guardian instalado em: %s\n", t.Name, filepath.Join(filepath.Dir(t.OpenCodePluginDir), openCodeConfigFile))
			}
		}
		count++
	}

	// Persiste AGENT_SYNC_PRETOOLUSE_VALIDATE=1 no shell rc do usuário, para
	// que o shell-validate hook saia do no-op nas próximas sessões sem precisar
	// exportar manualmente. Idempotente: não duplica a linha em chamadas
	// repetidas de `apply`. Detecta `~/.zshrc` se existir, senão `~/.bashrc`.
	if err := persistShellEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Falha ao persistir env no shell rc: %v\n", err)
	}

	fmt.Printf("\n✨ Concluído! %d CLI(s) sincronizada(s) com sucesso.\n", count)
}

// shellEnvMarker é a linha que marca o bloco gerenciado por este agente.
// Usada para tornar a escrita idempotente: se já existir um bloco com este
// marcador, substitui em vez de duplicar; se não existir, anexa.
const shellEnvMarker = "# agent-sync: shell-validate hook (gerenciado por `agent-sync -apply`)"

// persistShellEnv escreve `export AGENT_SYNC_PRETOOLUSE_VALIDATE=1` no shell
// rc do usuário. Idempotente: detecta bloco anterior pelo marcador, substitui
// se já existe, anexa se não. Tenta ~/.zshrc primeiro, depois ~/.bashrc.
//
// Falhas são reportadas mas não abortam o `apply` — o usuário ainda fica com
// o hook instalado no settings.json, só fica no-op até setar a env manualmente.
func persistShellEnv() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("encontrar HOME: %w", err)
	}
	rcPath := filepath.Join(home, ".zshrc")
	if _, err := os.Stat(rcPath); err != nil {
		rcPath = filepath.Join(home, ".bashrc")
	}

	existing, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ler %s: %w", rcPath, err)
	}

	block := shellEnvMarker + "\nexport AGENT_SYNC_PRETOOLUSE_VALIDATE=1\n"
	var out []byte
	if bytes.Contains(existing, []byte(shellEnvMarker)) {
		// Substitui o bloco existente (marcador + 1 linha) por uma versão nova.
		out = replaceBlock(existing, shellEnvMarker, block)
		fmt.Printf("✅ env atualizada em %s\n", rcPath)
	} else {
		// Anexa novo bloco com separador para legibilidade.
		sep := []byte("\n")
		if len(existing) > 0 && existing[len(existing)-1] != '\n' {
			sep = []byte("\n\n")
		}
		out = append(existing, sep...)
		out = append(out, []byte(block)...)
		fmt.Printf("✅ env persistida em %s (próxima sessão já ativa)\n", rcPath)
	}

	info, err := os.Stat(rcPath)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(rcPath, out, mode)
}

// replaceBlock substitui em data o trecho que começa com markerStart e vai
// até o próximo "\n\n" ou fim do arquivo, pelo novo conteúdo newBlock. Usado
// para reescrever o bloco gerenciado quando o `apply` roda de novo.
func replaceBlock(data []byte, markerStart string, newBlock string) []byte {
	idx := bytes.Index(data, []byte(markerStart))
	if idx < 0 {
		return data
	}
	end := idx + len(markerStart)
	// Procura fim do bloco: próxima linha em branco dupla ou fim do arquivo.
	rest := data[end:]
	endOffset := len(data)
	for i := 0; i < len(rest); i++ {
		if i+1 < len(rest) && rest[i] == '\n' && rest[i+1] == '\n' {
			endOffset = end + i + 1
			break
		}
	}
	out := make([]byte, 0, len(data))
	out = append(out, data[:idx]...)
	out = append(out, []byte(newBlock)...)
	out = append(out, data[endOffset:]...)
	return out
}
