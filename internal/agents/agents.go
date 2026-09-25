package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/agent-sync/internal/target"
)

const (
	codexInheritModel    = "gpt-5.5" // alias "inherit" mapeado para o Codex
	antigravityAgentDir  = "agents"
	antigravityPluginDoc = `{
  "name": "agent-sync",
  "description": "Agentes especialistas autorais do agent-sync."
}
`
)

type Source struct {
	Name        string
	Description string
	Body        string
	ReadOnly    bool
	Invokes     []string
}

// ParseAgent lê um agente canônico markdown (frontmatter name/description/readonly + corpo).
func ParseAgent(path string) (Source, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Source{}, err
	}
	lines := strings.Split(string(raw), "\n")

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
	if start != 0 || end == -1 {
		return Source{}, fmt.Errorf("%s: frontmatter inválido", path)
	}

	agent := Source{}
	for _, line := range lines[start+1 : end] {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			agent.Name = strings.TrimSpace(value)
		case "description":
			agent.Description = strings.TrimSpace(value)
		case "readonly":
			agent.ReadOnly = strings.EqualFold(strings.TrimSpace(value), "true")
		case "invokes":
			agent.Invokes = parseYAMLStringList(value)
		}
	}
	if agent.Name == "" {
		agent.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	if agent.Description == "" {
		return Source{}, fmt.Errorf("%s: description ausente", path)
	}
	agent.Body = strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	return agent, nil
}

func LoadAgents(dir string) ([]Source, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var agentList []Source
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		agent, err := ParseAgent(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		agentList = append(agentList, agent)
	}
	sort.Slice(agentList, func(i, j int) bool { return agentList[i].Name < agentList[j].Name })
	return agentList, nil
}

func markdownAgent(agent Source, extraLines []string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", agent.Name)
	fmt.Fprintf(&b, "description: %s\n", agent.Description)
	for _, line := range extraLines {
		b.WriteString(line + "\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(agent.Body)
	b.WriteString("\n")
	return b.String()
}

func tomlBasic(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}

func tomlMultiline(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"\"\"", "\\\"\\\"\\\"")
	return value
}

// parseYAMLStringList parseia uma lista YAML inline no formato [a, b, "c"].
func parseYAMLStringList(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return nil
	}
	parts := strings.Split(inner, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"")
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isOpenCodeGranular() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_SYNC_OPENCODE_GRANULAR")))
	switch v {
	case "", "1", "true", "yes", "on":
		return true
	}
	return false
}

func openCodeGranularExtra(agent Source, extra []string) []string {
	if !agent.ReadOnly && len(agent.Invokes) == 0 {
		return extra
	}
	extra = append(extra, "  task:")
	extra = append(extra, "    \"*\": deny")
	for _, target := range agent.Invokes {
		extra = append(extra, fmt.Sprintf("    %q: allow", target))
	}
	return extra
}

func codexAgent(agent Source) string {
	sandbox := "workspace-write"
	if agent.ReadOnly {
		sandbox = "read-only"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "name = \"%s\"\n", tomlBasic(agent.Name))
	fmt.Fprintf(&b, "description = \"%s\"\n", tomlBasic(agent.Description))
	fmt.Fprintf(&b, "model = \"%s\"\n", codexInheritModel)
	fmt.Fprintf(&b, "sandbox_mode = \"%s\"\n", sandbox)
	fmt.Fprintf(&b, "developer_instructions = \"\"\"\n%s\n\"\"\"\n", tomlMultiline(agent.Body))
	return b.String()
}

// RenderAgent converte o agente canônico para o formato nativo de cada CLI.
func RenderAgent(kind string, agent Source) (string, string, error) {
	switch kind {
	case "claude":
		return agent.Name + ".md", markdownAgent(agent, nil), nil
	case "cursor":
		extra := []string{"model: inherit"}
		if agent.ReadOnly {
			extra = append(extra, "readonly: true")
		}
		return agent.Name + ".md", markdownAgent(agent, extra), nil
	case "opencode":
		extra := []string{"mode: subagent"}
		if agent.ReadOnly {
			extra = append(extra, "permission:", "  edit: deny", "  bash: ask")
		}
		if len(agent.Invokes) > 0 {
			extra = append(extra, fmt.Sprintf("invokes: [%s]", strings.Join(agent.Invokes, ", ")))
		}
		if isOpenCodeGranular() {
			extra = openCodeGranularExtra(agent, extra)
		}
		return agent.Name + ".md", markdownAgent(agent, extra), nil
	case "antigravity":
		return agent.Name + ".md", markdownAgent(agent, []string{"model: inherit", "subagent: true"}), nil
	case "codex":
		return agent.Name + ".toml", codexAgent(agent), nil
	default:
		return "", "", fmt.Errorf("tipo de agente desconhecido: %s", kind)
	}
}

func writeAgentFile(path, content string, dryRun bool) error {
	if dryRun {
		fmt.Printf("[dry-run] write agent %s\n", path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// SyncAgents gera os agentes autorais no destino da CLI informada.
func SyncAgents(baseDir string, target target.Target, dryRun bool) (int, error) {
	if target.AgentsDir == "" || target.AgentKind == "" {
		return 0, nil
	}
	if pathutil.IsProtectedSkillsDir(target.AgentsDir) {
		return 0, fmt.Errorf("destino protegido (plugin de terceiros): %s", target.AgentsDir)
	}

	agentList, err := LoadAgents(filepath.Join(baseDir, "agents"))
	if err != nil {
		return 0, err
	}

	if target.PluginDir != "" {
		if err := writeAgentFile(filepath.Join(target.PluginDir, "plugin.json"), antigravityPluginDoc, dryRun); err != nil {
			return 0, err
		}
	}

	for _, agent := range agentList {
		filename, content, err := RenderAgent(target.AgentKind, agent)
		if err != nil {
			return 0, err
		}
		if err := writeAgentFile(filepath.Join(target.AgentsDir, filename), content, dryRun); err != nil {
			return 0, err
		}
	}
	return len(agentList), nil
}
