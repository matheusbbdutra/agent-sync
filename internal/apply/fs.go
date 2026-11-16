package apply

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// apply_fs.go: filesystem helpers para wirar (copy, mkdir, symlink,
// resolveBaseDir, isProtectedSkillsDir).
//
// Migrado de main.go em 2026-09-21 (Fase 6). Funcoes compartilhadas com
// scripts/install em Makefile.

func copyFile(src, dst string) error {
	if shouldDryRun() {
		fmt.Printf("[dry-run] copy %s -> %s\n", src, dst)
		return nil
	}
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

// removeLegacyClaudeRules apaga o ~/.claude/CLAUDE.md legado depois que as
// regras passam a ir para ~/.claude/AGENTS.md. O arquivo legado era cópia
// verbatim de rules/global-rules.md (gerado pelo wirar), então a remoção
// não perde conteúdo — só elimina regra duplicada/stale. Best-effort:
// ausência não é erro; respeita dry-run. Retorna true se o legado existia
// (removido ou a remover no dry-run).
func removeLegacyClaudeRules(rulesPath string) (bool, error) {
	legacy := filepath.Join(filepath.Dir(rulesPath), "CLAUDE.md")
	if legacy == rulesPath {
		return false, nil
	}
	if shouldDryRun() {
		if _, err := os.Stat(legacy); err != nil {
			return false, nil
		}
		fmt.Printf("[dry-run] remove %s\n", legacy)
		return true, nil
	}
	if err := os.Remove(legacy); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// removeLegacyGeminiRules apaga o ~/.gemini/GEMINI.md legado depois que as
// regras passam a ir para ~/.gemini/AGENTS.md (padrao canonico entre as
// 5 CLIs). Mesmo padrao de removeLegacyClaudeRules: copia verbatim
// gerada pelo wirar; remocao elimina duplicata/stale. Best-effort.
func removeLegacyGeminiRules(rulesPath string) (bool, error) {
	legacy := filepath.Join(filepath.Dir(rulesPath), "GEMINI.md")
	if legacy == rulesPath {
		return false, nil
	}
	if shouldDryRun() {
		if _, err := os.Stat(legacy); err != nil {
			return false, nil
		}
		fmt.Printf("[dry-run] remove %s\n", legacy)
		return true, nil
	}
	if err := os.Remove(legacy); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

func syncSkills(srcDir, dstDir string) error {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	dry := shouldDryRun()
	if !dry {
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return err
		}
	}

	managed := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		managed[skillName] = true
		sourceSkill := filepath.Join(srcDir, skillName)
		targetSkill := filepath.Join(dstDir, skillName)

		if dry {
			fmt.Printf("[dry-run] symlink skill %s -> %s\n", targetSkill, sourceSkill)
			continue
		}

		// Conteudo local com o mesmo nome de uma skill do repo nunca e apagado:
		// -apply e aditivo para o que o manifest nao gerencia.
		if fi, lerr := os.Lstat(targetSkill); lerr == nil {
			if fi.Mode()&os.ModeSymlink == 0 {
				fmt.Printf("[skip] %s existe e nao e symlink — preservado\n", targetSkill)
				continue
			}
			_ = os.Remove(targetSkill)
		}

		// Cria symlink relativo/absoluto para sincronização bidirecional
		if err := os.Symlink(sourceSkill, targetSkill); err != nil {
			// Fallback: cópia direta se symlink falhar
			_ = copyDir(sourceSkill, targetSkill)
		}
	}

	reportUnmanagedSkills(dstDir, managed, dry)
	return nil
}

// reportUnmanagedSkills lista o que existe no destino e o repo não gerencia, para
// que fique explícito que -apply não toca nessas entradas.
func reportUnmanagedSkills(dstDir string, managed map[string]bool, dry bool) {
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		return
	}
	var kept []string
	for _, e := range entries {
		if !managed[e.Name()] {
			kept = append(kept, e.Name())
		}
	}
	if len(kept) == 0 {
		return
	}
	prefix := ""
	if dry {
		prefix = "[dry-run] "
	}
	fmt.Printf("%spreservadas (fora do repo): %s\n", prefix, strings.Join(kept, ", "))
}

func findBaseDir(starts []string) (string, bool) {
	return pathutil.FindBaseDir(starts)
}

func resolveBaseDir(exePath, cwd, envHome string) (string, error) {
	return pathutil.ResolveBaseDir(exePath, cwd, envHome)
}

func isProtectedSkillsDir(dir string) bool {
	return pathutil.IsProtectedSkillsDir(dir)
}
