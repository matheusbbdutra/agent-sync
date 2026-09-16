package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

type agentSource struct {
	Name        string
	Description string
	Body        string
	ReadOnly    bool
}

// parseAgent lê um agente canônico markdown (frontmatter name/description/readonly + corpo).
func parseAgent(path string) (agentSource, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return agentSource{}, err
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
		return agentSource{}, fmt.Errorf("%s: frontmatter inválido", path)
	}

	agent := agentSource{}
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
		}
	}
	if agent.Name == "" {
		agent.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	}
	if agent.Description == "" {
		return agentSource{}, fmt.Errorf("%s: description ausente", path)
	}
	agent.Body = strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	return agent, nil
}

func loadAgents(dir string) ([]agentSource, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var agents []agentSource
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		agent, err := parseAgent(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	return agents, nil
}

func markdownAgent(agent agentSource, extraLines []string) string {
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

func codexAgent(agent agentSource) string {
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

// renderAgent converte o agente canônico para o formato nativo de cada CLI.
func renderAgent(kind string, agent agentSource) (string, string, error) {
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
		return agent.Name + ".md", markdownAgent(agent, extra), nil
	case "antigravity":
		return agent.Name + ".md", markdownAgent(agent, []string{"model: inherit", "subagent: true"}), nil
	case "codex":
		return agent.Name + ".toml", codexAgent(agent), nil
	default:
		return "", "", fmt.Errorf("tipo de agente desconhecido: %s", kind)
	}
}

func writeAgentFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// syncAgents gera os agentes autorais no destino da CLI informada.
func syncAgents(baseDir string, target TargetCLI) (int, error) {
	if target.AgentsDir == "" || target.AgentKind == "" {
		return 0, nil
	}
	if isProtectedSkillsDir(target.AgentsDir) {
		return 0, fmt.Errorf("destino protegido (plugin de terceiros): %s", target.AgentsDir)
	}

	agents, err := loadAgents(filepath.Join(baseDir, "agents"))
	if err != nil {
		return 0, err
	}

	if target.PluginDir != "" {
		if err := writeAgentFile(filepath.Join(target.PluginDir, "plugin.json"), antigravityPluginDoc); err != nil {
			return 0, err
		}
	}

	for _, agent := range agents {
		filename, content, err := renderAgent(target.AgentKind, agent)
		if err != nil {
			return 0, err
		}
		if err := writeAgentFile(filepath.Join(target.AgentsDir, filename), content); err != nil {
			return 0, err
		}
	}
	return len(agents), nil
}
