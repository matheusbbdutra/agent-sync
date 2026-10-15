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

// splitFrontmatter devolve o bloco YAML inicial do SKILL.md, sem os
// delimitadores. ok=false quando o arquivo não abre com "---" ou não fecha.
func splitFrontmatter(content string) (string, bool) {
	if !strings.HasPrefix(content, "---\n") {
		return "", false
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return "", false
	}
	return content[4 : 4+end], true
}

// frontmatterField extrai o valor de um campo de topo, concatenando linhas de
// continuação indentadas. Devolve "" quando o campo não existe.
func frontmatterField(fm, key string) string {
	prefix := key + ":"
	var value string
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimRight(line, " \t\r")
		if value != "" {
			if strings.HasPrefix(trimmed, " ") || strings.HasPrefix(trimmed, "\t") {
				value += " " + strings.TrimSpace(trimmed)
				continue
			}
			break
		}
		if strings.HasPrefix(trimmed, prefix) {
			value = strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			value = strings.Trim(value, "\"'")
		}
	}
	return value
}

// collisionThreshold é o Jaccard mínimo entre os termos de duas descriptions
// para tratá-las como concorrentes pelo mesmo prompt (misrouting). Calibrado no
// catálogo real de 54 skills, onde o maior valor observado é 0.295
// (backend-security-coder x frontend-security-coder); 0.25 sinaliza só esse par.
const collisionThreshold = 0.25

// descTokenPattern extrai termos com 3+ caracteres preservando hífen, ponto e
// mais (ex.: node.js, clean-code).
var descTokenPattern = regexp.MustCompile(`[a-z0-9][a-z0-9\-.+]{2,}`)

// descStopwords remove palavras funcionais (PT-BR e EN) presentes em quase toda
// description — sem isso o Jaccard mediria estilo de escrita, não sobreposição.
var descStopwords = map[string]bool{
	"agente": true, "agent": true, "codigo": true, "projeto": true,
	"skill": true, "skills": true, "tarefa": true, "tarefas": true, "usar": true,
	"de": true, "da": true, "do": true, "das": true, "dos": true, "em": true,
	"no": true, "na": true, "nos": true, "nas": true, "um": true, "uma": true,
	"para": true, "por": true, "com": true, "sem": true, "que": true,
	"qual": true, "quais": true, "como": true, "quando": true,
	"use": true, "when": true, "this": true, "the": true, "and": true,
	"or": true, "for": true, "with": true,
}

func descTokens(description string) map[string]bool {
	set := map[string]bool{}
	for _, tok := range descTokenPattern.FindAllString(strings.ToLower(description), -1) {
		if descStopwords[tok] {
			continue
		}
		set[tok] = true
	}
	return set
}

// jaccard mede a sobreposição entre dois conjuntos de termos (0..1).
func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for tok := range a {
		if b[tok] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

// collisionIssues sinaliza (warning) pares de skills que competem pelo mesmo
// prompt — sintoma de gatilho genérico, não de erro no formato do SKILL.md.
func collisionIssues(descriptions map[string]string) []SkillLintIssue {
	ids := make([]string, 0, len(descriptions))
	for id := range descriptions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	tokens := make(map[string]map[string]bool, len(ids))
	for _, id := range ids {
		tokens[id] = descTokens(descriptions[id])
	}

	var issues []SkillLintIssue
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			score := jaccard(tokens[ids[i]], tokens[ids[j]])
			if score < collisionThreshold {
				continue
			}
			issues = append(issues, SkillLintIssue{
				Skill:    ids[i] + " × " + ids[j],
				Check:    "collision",
				Severity: "warning",
				Message: fmt.Sprintf("descriptions se sobrepõem (Jaccard %.2f): "+
					"diferencie os gatilhos para evitar misrouting", score),
			})
		}
	}
	return issues
}

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
	descriptions := map[string]string{}
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
		if fm, ok := splitFrontmatter(content); ok {
			if desc := frontmatterField(fm, "description"); desc != "" {
				descriptions[e.Name()] = desc
			}
		}
		result.Issues = append(result.Issues, LintSkill(e.Name(), content)...)
	}
	result.Issues = append(result.Issues, collisionIssues(descriptions)...)

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
	fm, ok := splitFrontmatter(content)
	if !ok {
		message := "SKILL.md sem frontmatter YAML (esperado iniciar com '---')"
		if strings.HasPrefix(content, "---\n") {
			message = "frontmatter YAML sem fechamento ('---' final ausente)"
		}
		issues = append(issues, SkillLintIssue{
			Skill:    id,
			Check:    "frontmatter",
			Severity: "error",
			Message:  message,
		})
		return issues
	}

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
	desc := frontmatterField(fm, "description")
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
