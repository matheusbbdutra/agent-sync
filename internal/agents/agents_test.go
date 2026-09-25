package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempAgent(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestParseAgent(t *testing.T) {
	path := writeTempAgent(t, "---\nname: sample\ndescription: Faz algo util\nreadonly: true\n---\n\nCorpo do agente.\n")

	agent, err := ParseAgent(path)
	if err != nil {
		t.Fatalf("ParseAgent: %v", err)
	}
	if agent.Name != "sample" || agent.Description != "Faz algo util" {
		t.Fatalf("frontmatter inesperado: %+v", agent)
	}
	if !agent.ReadOnly {
		t.Fatal("esperava readonly=true")
	}
	if agent.Body != "Corpo do agente." {
		t.Fatalf("corpo inesperado: %q", agent.Body)
	}
}

func TestParseAgentMissingDescription(t *testing.T) {
	path := writeTempAgent(t, "---\nname: sample\n---\ncorpo\n")
	if _, err := ParseAgent(path); err == nil {
		t.Fatal("esperava erro por description ausente")
	}
}

func TestRenderClaude(t *testing.T) {
	agent := Source{Name: "x", Description: "d", Body: "corpo", ReadOnly: true}
	filename, content, err := RenderAgent("claude", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if filename != "x.md" {
		t.Fatalf("filename = %q", filename)
	}
	if strings.Contains(content, "mode:") || strings.Contains(content, "model:") {
		t.Fatalf("claude não deveria ter mode/model: %q", content)
	}
}

func TestRenderCursorReadonly(t *testing.T) {
	agent := Source{Name: "x", Description: "d", Body: "corpo", ReadOnly: true}
	filename, content, err := RenderAgent("cursor", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if filename != "x.md" {
		t.Fatalf("filename = %q", filename)
	}
	for _, want := range []string{"model: inherit", "readonly: true"} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
}

func TestRenderCursorWritable(t *testing.T) {
	agent := Source{Name: "x", Description: "d", Body: "corpo"}
	_, content, err := RenderAgent("cursor", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if !strings.Contains(content, "model: inherit") {
		t.Fatalf("faltando model: inherit em:\n%s", content)
	}
	if strings.Contains(content, "readonly:") {
		t.Fatalf("não deveria ter readonly: %q", content)
	}
}

func TestRenderOpenCodeReadonly(t *testing.T) {
	agent := Source{Name: "x", Description: "d", Body: "corpo", ReadOnly: true}
	_, content, err := RenderAgent("opencode", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	for _, want := range []string{"mode: subagent", "permission:", "edit: deny"} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
}

func TestRenderOpenCodeWritableHasNoPermission(t *testing.T) {
	agent := Source{Name: "x", Description: "d", Body: "corpo"}
	_, content, err := RenderAgent("opencode", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if strings.Contains(content, "permission:") {
		t.Fatalf("não deveria ter permission: %q", content)
	}
}

func TestRenderAntigravity(t *testing.T) {
	agent := Source{Name: "x", Description: "d", Body: "corpo"}
	_, content, err := RenderAgent("antigravity", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	for _, want := range []string{"model: inherit", "subagent: true"} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
}

func TestRenderCodexReadonlyAndEscaping(t *testing.T) {
	agent := Source{Name: "x", Description: `aspas "no" meio`, Body: `regex \d+ e "aspas"`, ReadOnly: true}
	filename, content, err := RenderAgent("codex", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if filename != "x.toml" {
		t.Fatalf("filename = %q", filename)
	}
	for _, want := range []string{
		`sandbox_mode = "read-only"`,
		`model = "` + codexInheritModel + `"`,
		`developer_instructions = """`,
		`regex \\d+ e "aspas"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
}

func TestRenderCodexWritable(t *testing.T) {
	agent := Source{Name: "x", Description: "d", Body: "corpo"}
	_, content, err := RenderAgent("codex", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if !strings.Contains(content, `sandbox_mode = "workspace-write"`) {
		t.Fatalf("esperava workspace-write: %s", content)
	}
}

func TestLoadAgentsReadsRepoAgents(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\nname: b\ndescription: B\n---\ncorpo B\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\nname: a\ndescription: A\n---\ncorpo A\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	agents, err := LoadAgents(dir)
	if err != nil {
		t.Fatalf("LoadAgents: %v", err)
	}
	if len(agents) != 2 || agents[0].Name != "a" || agents[1].Name != "b" {
		t.Fatalf("ordem/quantidade inesperada: %+v", agents)
	}
}

func TestMRReviewerRendersForAllCLIs(t *testing.T) {
	path := filepath.Join("..", "..", "agents", "mr-reviewer.md")
	agent, err := ParseAgent(path)
	if err != nil {
		t.Fatal(err)
	}
	if !agent.ReadOnly || !strings.Contains(agent.Body, "mr-review-local") {
		t.Fatal("agente de MR deve ser somente leitura e usar o coletor comum")
	}
	for _, kind := range []string{"claude", "codex", "antigravity", "opencode", "cursor"} {
		name, content, err := RenderAgent(kind, agent)
		if err != nil || !strings.Contains(name, "mr-reviewer") || !strings.Contains(content, "mr-review-local") {
			t.Fatalf("renderização %s: %s, %v", kind, name, err)
		}
	}
}

func TestParseYAMLStringList(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty brackets", "[]", nil},
		{"whitespace only", "[ ]", nil},
		{"single unquoted", "[code-reviewer]", []string{"code-reviewer"}},
		{"multiple unquoted", "[code-reviewer, mr-reviewer]", []string{"code-reviewer", "mr-reviewer"}},
		{"quoted", `["code-reviewer", "mr-reviewer"]`, []string{"code-reviewer", "mr-reviewer"}},
		{"trailing comma", "[a, b, ]", []string{"a", "b"}},
		{"not a list", "code-reviewer", nil},
		{"missing bracket", "[a, b", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseYAMLStringList(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len: obtido %d (%v), esperado %d (%v)", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("idx %d: obtido %q, esperado %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseAgentReadsInvokes(t *testing.T) {
	path := writeTempAgent(t, "---\nname: orchestrator\ndescription: Coordena revisoes\nreadonly: true\ninvokes: [code-reviewer, mr-reviewer]\n---\ncorpo\n")
	agent, err := ParseAgent(path)
	if err != nil {
		t.Fatalf("ParseAgent: %v", err)
	}
	if !agent.ReadOnly {
		t.Error("esperava readonly=true")
	}
	if len(agent.Invokes) != 2 || agent.Invokes[0] != "code-reviewer" || agent.Invokes[1] != "mr-reviewer" {
		t.Errorf("invokes divergente: %v", agent.Invokes)
	}
}

func TestRenderOpenCodeReadonlyHasTaskDenyStar(t *testing.T) {
	t.Setenv("AGENT_SYNC_OPENCODE_GRANULAR", "1")
	agent := Source{Name: "x", Description: "d", Body: "corpo", ReadOnly: true}
	_, content, err := RenderAgent("opencode", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	for _, want := range []string{"permission:", "edit: deny", "bash: ask", "task:", `"*": deny`} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
}

func TestRenderOpenCodeWithInvokesEmitsAllowlist(t *testing.T) {
	t.Setenv("AGENT_SYNC_OPENCODE_GRANULAR", "1")
	agent := Source{
		Name:        "orchestrator",
		Description: "d",
		Body:        "corpo",
		ReadOnly:    true,
		Invokes:     []string{"code-reviewer", "mr-reviewer"},
	}
	_, content, err := RenderAgent("opencode", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	for _, want := range []string{"task:", `"*": deny`, `"code-reviewer": allow`, `"mr-reviewer": allow`} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
	idxStar := strings.Index(content, `"*": deny`)
	idxCode := strings.Index(content, `"code-reviewer": allow`)
	if idxStar >= idxCode {
		t.Errorf("ordem invertida: deny global deve vir antes dos allows especificos")
	}
}

func TestRenderOpenCodeGranularDisabledKeepsLegacyOutput(t *testing.T) {
	t.Setenv("AGENT_SYNC_OPENCODE_GRANULAR", "0")
	agent := Source{
		Name:        "x",
		Description: "d",
		Body:        "corpo",
		ReadOnly:    true,
		Invokes:     []string{"code-reviewer"},
	}
	_, content, err := RenderAgent("opencode", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if strings.Contains(content, "task:") {
		t.Errorf("granular=0 nao deveria gerar task:, obteve:\n%s", content)
	}
}

func TestRenderClaudeIgnoresInvokesField(t *testing.T) {
	agent := Source{
		Name:        "x",
		Description: "d",
		Body:        "corpo",
		Invokes:     []string{"code-reviewer"},
	}
	_, content, err := RenderAgent("claude", agent)
	if err != nil {
		t.Fatalf("RenderAgent: %v", err)
	}
	if strings.Contains(content, "invokes:") {
		t.Errorf("claude nao deveria emitir invokes: %q", content)
	}
}

func TestIsOpenCodeGranular(t *testing.T) {
	cases := []struct {
		envVal string
		want   bool
	}{
		{"", true},
		{"1", true},
		{"true", true},
		{"on", true},
		{"0", false},
		{"false", false},
		{"off", false},
		{"yes", true},
		{"no", false},
	}
	for _, tc := range cases {
		t.Run("env="+tc.envVal, func(t *testing.T) {
			t.Setenv("AGENT_SYNC_OPENCODE_GRANULAR", tc.envVal)
			if got := isOpenCodeGranular(); got != tc.want {
				t.Errorf("isOpenCodeGranular com %q: obtido %v, esperado %v", tc.envVal, got, tc.want)
			}
		})
	}
}
