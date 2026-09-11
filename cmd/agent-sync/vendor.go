package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type skillEntry struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

type manifestSource struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	License    string `json:"license"`
	SkillsDir  string `json:"skillsDir"`
}

type manifest struct {
	Source manifestSource `json:"source"`
	Skills []skillEntry   `json:"skills"`
}

func loadManifest(path string) (*manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("erro ao ler %s: %w", path, err)
	}
	if len(m.Skills) == 0 {
		return nil, fmt.Errorf("%s não define skills", path)
	}
	return &m, nil
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

func vendorSkill(src, dst, id string) error {
	data, err := os.ReadFile(filepath.Join(src, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("SKILL.md ausente na origem: %w", err)
	}
	if err := validateSkill(string(data), id); err != nil {
		return err
	}

	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := copyDir(src, dst); err != nil {
		return err
	}
	normalized := normalizeFrontmatter(string(data))
	return os.WriteFile(filepath.Join(dst, "SKILL.md"), []byte(normalized), 0o644)
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
