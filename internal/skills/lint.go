package skills

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
)

// SkillLintIssue representa 1 problema encontrado em uma skill.
type SkillLintIssue struct {
	Skill    string `json:"skill"`
	Check    string `json:"check"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// SkillLintResult agrega issues + contadores.
type SkillLintResult struct {
	SkillsDir string           `json:"skills_dir"`
	Total     int              `json:"total"`
	Errors    int              `json:"errors"`
	Warnings  int              `json:"warnings"`
	Issues    []SkillLintIssue `json:"issues"`
}

var triggerPattern = regexp.MustCompile(`(?i)\b(use when|quando|when |triggered by|if you|para |apply this|use this|implement|design|test|write|build|optimi[sz]e|debug|audit|research|investigat|monitor|manag|review|creat|document|integrat|migrat|moderni[sz]e|refactor|extend|patch|track|teach|learn|explain|map|measur|identif|classif|compar|estimat|plan|execut|orchestrat|coordinat|engin)\b`)

// RunLint implementa `agent-sync skills lint`.
func RunLint(args []string) error {
	return RunLintWithBase(args, "")
}

// RunLintWithBase é a versão injetável de RunLint.
func RunLintWithBase(args []string, baseDirInjected string) error {
	fs := flag.NewFlagSet("skills lint", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Saída em JSON estruturado em vez de tabela texto")
	if err := fs.Parse(args); err != nil {
		return err
	}

	baseDir := baseDirInjected
	if baseDir == "" {
		exePath, _ := os.Executable()
		cwd, _ := os.Getwd()
		var err error
		baseDir, err = pathutil.ResolveBaseDir(exePath, cwd, os.Getenv("AGENT_SYNC_HOME"))
		if err != nil {
			return err
		}
	}

	result, err := CollectLint(baseDir)
	if err != nil {
		return err
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Skills verificadas: %d (errors=%d, warnings=%d)\n",
		result.Total, result.Errors, result.Warnings)
	fmt.Println()
	if len(result.Issues) == 0 {
		fmt.Println("✅ Nenhuma issue encontrada.")
		return nil
	}
	fmt.Println("skill                              check       severity  message")
	fmt.Println(strings.Repeat("-", 80))
	for _, iss := range result.Issues {
		fmt.Printf("%-34s %-10s %-8s %s\n", iss.Skill, iss.Check, iss.Severity, iss.Message)
	}

	if result.Errors > 0 {
		return fmt.Errorf("%d error(s) encontrado(s)", result.Errors)
	}
	return nil
}

// CollectLint executa a verificação estática de skills e retorna o resultado estruturado.
func CollectLint(baseDir string) (SkillLintResult, error) {
	skillsDir := filepath.Join(baseDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return SkillLintResult{}, fmt.Errorf("ler %s: %w", skillsDir, err)
	}

	result := SkillLintResult{SkillsDir: skillsDir, Issues: []SkillLintIssue{}}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "manifest.json" {
			continue
		}
		result.Total++
		skillPath := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		raw, err := os.ReadFile(skillPath)
		if err != nil {
			result.Errors++
			result.Issues = append(result.Issues, SkillLintIssue{
				Skill:    e.Name(),
				Check:    "read",
				Severity: "error",
				Message:  fmt.Sprintf("não conseguiu ler %s: %v", skillPath, err),
			})
			continue
		}
		content := string(raw)
		result.Issues = append(result.Issues, LintSkill(e.Name(), content)...)
	}

	for _, iss := range result.Issues {
		switch iss.Severity {
		case "error":
			result.Errors++
		case "warning":
			result.Warnings++
		}
	}
	sort.SliceStable(result.Issues, func(i, j int) bool {
		if result.Issues[i].Severity != result.Issues[j].Severity {
			return result.Issues[i].Severity < result.Issues[j].Severity
		}
		return result.Issues[i].Skill < result.Issues[j].Skill
	})

	return result, nil
}

// LintSkill aplica os 3 checks a uma skill e devolve a lista de issues.
func LintSkill(id, content string) []SkillLintIssue {
	var issues []SkillLintIssue

	// Check 1: frontmatter YAML.
	if !strings.HasPrefix(content, "---\n") {
		issues = append(issues, SkillLintIssue{
			Skill:    id,
			Check:    "frontmatter",
			Severity: "error",
			Message:  "SKILL.md sem frontmatter YAML (esperado iniciar com '---')",
		})
		return issues
	}

	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		issues = append(issues, SkillLintIssue{
			Skill:    id,
			Check:    "frontmatter",
			Severity: "error",
			Message:  "frontmatter YAML sem fechamento ('---' final ausente)",
		})
		return issues
	}
	fm := content[4 : 4+end]

	// Check 2: name bate com pasta.
	nameRE := regexp.MustCompile(`(?m)^name:\s*(.+?)\s*$`)
	nameMatch := nameRE.FindStringSubmatch(fm)
	if nameMatch == nil {
		issues = append(issues, SkillLintIssue{
			Skill:    id,
			Check:    "name",
			Severity: "error",
			Message:  "campo 'name:' ausente no frontmatter",
		})
	} else {
		got := strings.TrimSpace(nameMatch[1])
		got = strings.Trim(got, "\"'")
		if got != id {
			issues = append(issues, SkillLintIssue{
				Skill:    id,
				Check:    "name",
				Severity: "error",
				Message:  fmt.Sprintf("name %q difere da pasta %q", got, id),
			})
		}
	}

	// Check 3: description com gatilho.
	var desc string
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimRight(line, " \t\r")
		if desc != "" {
			if strings.HasPrefix(trimmed, " ") || strings.HasPrefix(trimmed, "\t") {
				desc += " " + strings.TrimSpace(trimmed)
				continue
			}
			break
		}
		if strings.HasPrefix(trimmed, "description:") {
			desc = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			desc = strings.Trim(desc, "\"'")
		}
	}
	if desc == "" {
		issues = append(issues, SkillLintIssue{
			Skill:    id,
			Check:    "description",
			Severity: "error",
			Message:  "campo 'description:' ausente no frontmatter",
		})
		return issues
	}
	if !triggerPattern.MatchString(desc) {
		issues = append(issues, SkillLintIssue{
			Skill:    id,
			Check:    "description",
			Severity: "error",
			Message:  "description sem gatilho (use when|quando|when |para |verbos de acao)",
		})
	}

	return issues
}
