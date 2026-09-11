package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type TargetCLI struct {
	Name      string
	RulesPath string
	SkillsDir string
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
			Name:      "claude",
			RulesPath: filepath.Join(home, ".claude", "CLAUDE.md"),
			SkillsDir: filepath.Join(home, ".claude", "skills"),
		},
		{
			Name:      "codex",
			RulesPath: filepath.Join(home, ".codex", "AGENTS.md"),
			SkillsDir: filepath.Join(home, ".codex", "skills"),
		},
		{
			Name:      "gemini",
			RulesPath: filepath.Join(home, ".gemini", "GEMINI.md"),
			SkillsDir: filepath.Join(home, ".gemini", "config", "plugins", "antigravity-skills-manager", "skills"),
		},
		{
			Name:      "opencode",
			RulesPath: filepath.Join(home, ".config", "opencode", "AGENTS.md"),
			SkillsDir: filepath.Join(home, ".config", "opencode", "skills"),
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

func main() {
	applyFlag := flag.Bool("apply", false, "Aplica as regras e skills para todas as CLIs configuradas")
	targetFlag := flag.String("target", "", "Aplica para uma CLI específica (claude, codex, gemini, opencode)")
	statusFlag := flag.Bool("status", false, "Exibe o status de sincronização com as CLIs")
	flag.Parse()

	exePath, err := os.Executable()
	baseDir := "."
	if err == nil {
		baseDir = filepath.Dir(filepath.Dir(filepath.Dir(exePath)))
	}
	// Fallback para diretório de trabalho se estiver rodando via go run
	if _, err := os.Stat(filepath.Join(baseDir, "rules", "global-rules.md")); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		baseDir = cwd
	}

	rulesSource := filepath.Join(baseDir, "rules", "global-rules.md")
	skillsSource := filepath.Join(baseDir, "skills")

	if !*applyFlag && !*statusFlag && *targetFlag == "" {
		fmt.Println("🚀 Agent-Sync: Gerenciador Unificado de Regras e Skills para Agentes AI")
		fmt.Println("\nUso:")
		fmt.Println("  agent-sync -apply              # Sincroniza em todas as CLIs instaladas")
		fmt.Println("  agent-sync -target <cli>       # Sincroniza apenas para claude, codex, gemini ou opencode")
		fmt.Println("  agent-sync -status             # Verifica o status atual de cada CLI")
		return
	}

	targets := getTargets()

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
			fmt.Printf(" - %-10s | Regras: %-16s | Skills: %d instaladas\n", t.Name, rulesStatus, skillsCount)
		}
		return
	}

	fmt.Println("🔄 Iniciando sincronização...")
	count := 0
	for _, t := range targets {
		if *targetFlag != "" && *targetFlag != t.Name {
			continue
		}

		// Copia regras
		if err := copyFile(rulesSource, t.RulesPath); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao atualizar regras: %v\n", t.Name, err)
		} else {
			fmt.Printf("✅ [%s] Regras atualizadas em: %s\n", t.Name, t.RulesPath)
		}

		// Sincroniza skills
		if err := syncSkills(skillsSource, t.SkillsDir); err != nil {
			fmt.Printf("⚠️  [%s] Falha ao sincronizar skills: %v\n", t.Name, err)
		} else {
			fmt.Printf("✅ [%s] Skills sincronizadas em: %s\n", t.Name, t.SkillsDir)
		}
		count++
	}

	fmt.Printf("\n✨ Concluído! %d CLI(s) sincronizada(s) com sucesso.\n", count)
}
