package apply

// vendor.go: copia de dependencias externas (Go modules, tools/) para
// build reproduzivel. Migrado sem renomear em 2026-09-21 (Fase 7b).
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/skills"
)

func loadManifest(path string) (*skills.Manifest, error) {
	return skills.LoadManifest(path)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// parseFrontmatter devolve as chaves de topo do bloco YAML inicial.
func parseFrontmatter(content string) (map[string]string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, false
	}
	fields := map[string]string{}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			return fields, true
		}
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return nil, false
}

// normalizeFrontmatter remove chaves aninhadas de modelo (metadata.model) para
// manter as skills neutras entre CLIs, preservando os demais metadados.
func normalizeFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	start, end := -1, -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if start == -1 {
				start = i
			} else {
				end = i
				break
			}
		}
	}
	if start == -1 || end == -1 {
		return content
	}

	fm := lines[start+1 : end]
	var out []string
	for i := 0; i < len(fm); i++ {
		line := fm[i]
		if strings.TrimSpace(line) != "metadata:" {
			out = append(out, line)
			continue
		}
		var kept []string
		j := i + 1
		for j < len(fm) && (strings.HasPrefix(fm[j], " ") || strings.HasPrefix(fm[j], "\t")) {
			if !strings.HasPrefix(strings.TrimSpace(fm[j]), "model:") {
				kept = append(kept, fm[j])
			}
			j++
		}
		if len(kept) > 0 {
			out = append(out, line)
			out = append(out, kept...)
		}
		i = j - 1
	}

	result := append([]string{}, lines[:start+1]...)
	result = append(result, out...)
	result = append(result, lines[end:]...)
	return strings.Join(result, "\n")
}

func validateSkill(content, id string) error {
	fields, ok := parseFrontmatter(content)
	if !ok {
		return fmt.Errorf("frontmatter ausente")
	}
	if name := fields["name"]; name != id {
		return fmt.Errorf("name %q não corresponde à pasta %q", name, id)
	}
	if strings.TrimSpace(fields["description"]) == "" {
		return fmt.Errorf("description ausente")
	}
	if !strings.Contains(content, "## Use this skill when") {
		return fmt.Errorf("seção '## Use this skill when' ausente")
	}
	return nil
}

// vendorSkill substitui dst pelo conteúdo de src usando o padrão
// "copiar para temp + rename atômico": dst só é removido depois que o
// source passa TODA validação e a cópia para um diretório temporário é
// confirmada. Isso evita o foot-gun anterior onde um source com SKILL.md
// vazio/inexistente sobrescrevia dst com arquivos vazios (bug que zerou
// 54 skills no projeto em 2026-09-23 — ver Postmortem correlato).
func vendorSkill(src, dst, id string) error {
	data, err := os.ReadFile(filepath.Join(src, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("SKILL.md ausente na origem: %w", err)
	}
	if err := validateSkill(string(data), id); err != nil {
		return err
	}

	// Fase 1: copiar para diretório temporário SIBLING (sem tocar em dst).
	// Se src estiver vazio/quebrado, copyDir falhará e dst fica intacto.
	tmp := dst + ".new-" + id
	if err := os.RemoveAll(tmp); err != nil {
		return fmt.Errorf("limpar tmp anterior: %w", err)
	}
	if err := copyDir(src, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("copiar %s -> %s: %w", src, tmp, err)
	}

	// Fase 2: revalidar SKILL.md do temp (defesa em profundidade: copyDir
	// pode ter sucesso mas o SKILL.md do temp pode estar corrompido).
	tmpSkill, err := os.ReadFile(filepath.Join(tmp, "SKILL.md"))
	if err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("SKILL.md ausente no temp após copy: %w", err)
	}
	if err := validateSkill(string(tmpSkill), id); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("temp inválido após copy: %w", err)
	}

	// Fase 3: escrita final do SKILL.md normalizado dentro do temp.
	normalized := normalizeFrontmatter(string(data))
	if err := os.WriteFile(filepath.Join(tmp, "SKILL.md"), []byte(normalized), 0o644); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}

	// Fase 4: trocar. Apenas aqui dst é modificado.
	if err := os.RemoveAll(dst); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("remover dst antigo: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		// rollback: tentar restaurar dst se possível; tmp fica como evidência
		return fmt.Errorf("rename tmp -> dst: %w (dst pode estar removido)", err)
	}
	return nil
}

func runVendor(baseDir, sourceDir string) error {
	m, err := loadManifest(filepath.Join(baseDir, "skills", "manifest.json"))
	if err != nil {
		return err
	}
	if sourceDir == "" {
		sourceDir = expandHome(m.Source.SkillsDir)
	}
	if sourceDir == "" {
		return fmt.Errorf("diretório de origem não informado (use -source)")
	}
	if _, err := os.Stat(sourceDir); err != nil {
		return fmt.Errorf("diretório de origem inacessível (%s): %w", sourceDir, err)
	}

	fmt.Printf("📦 Vendorizando %d skill(s) de %s\n", len(m.Skills), sourceDir)
	// Precheck: se TODAS as skills do source estão com SKILL.md ausente/vazio,
	// abortar ANTES de tocar em dst. Evita cenário onde um source dir recém-clonado
	// (ou symlink quebrado) deixa N skills zeradas no projeto (bug 2026-09-23).
	validSourceCount := 0
	for _, s := range m.Skills {
		data, err := os.ReadFile(filepath.Join(sourceDir, s.ID, "SKILL.md"))
		if err != nil || len(data) == 0 {
			continue
		}
		if validateSkill(string(data), s.ID) == nil {
			validSourceCount++
		}
	}
	if validSourceCount == 0 && len(m.Skills) > 0 {
		return fmt.Errorf("source íntegro: 0/%d skills com SKILL.md válido em %s (source vazio, corrompido ou apontando para diretório errado)", len(m.Skills), sourceDir)
	}

	vendored, failed := 0, 0
	for _, s := range m.Skills {
		src := filepath.Join(sourceDir, s.ID)
		dst := filepath.Join(baseDir, "skills", s.ID)
		if err := vendorSkill(src, dst, s.ID); err != nil {
			fmt.Printf("⚠️  [%s] %v\n", s.ID, err)
			failed++
			continue
		}
		fmt.Printf("✅ [%s] vendorizada (%s)\n", s.ID, s.Domain)
		vendored++
	}

	fmt.Printf("\n✨ %d vendorizada(s), %d falha(s).\n", vendored, failed)
	if failed > 0 {
		return fmt.Errorf("%d skill(s) falharam", failed)
	}
	return nil
}
