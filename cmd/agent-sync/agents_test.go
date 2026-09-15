package main

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

	agent, err := parseAgent(path)
	if err != nil {
		t.Fatalf("parseAgent: %v", err)
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
	if _, err := parseAgent(path); err == nil {
		t.Fatal("esperava erro por description ausente")
	}
}

func TestRenderClaude(t *testing.T) {
	agent := agentSource{Name: "x", Description: "d", Body: "corpo", ReadOnly: true}
	filename, content, err := renderAgent("claude", agent)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
	}
	if filename != "x.md" {
		t.Fatalf("filename = %q", filename)
	}
	if strings.Contains(content, "mode:") || strings.Contains(content, "model:") {
		t.Fatalf("claude não deveria ter mode/model: %q", content)
	}
}

func TestRenderCursorReadonly(t *testing.T) {
	agent := agentSource{Name: "x", Description: "d", Body: "corpo", ReadOnly: true}
	filename, content, err := renderAgent("cursor", agent)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
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
	agent := agentSource{Name: "x", Description: "d", Body: "corpo"}
	_, content, err := renderAgent("cursor", agent)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
	}
	if !strings.Contains(content, "model: inherit") {
		t.Fatalf("faltando model: inherit em:\n%s", content)
	}
	if strings.Contains(content, "readonly:") {
		t.Fatalf("não deveria ter readonly: %q", content)
	}
}

func TestRenderOpenCodeReadonly(t *testing.T) {
	agent := agentSource{Name: "x", Description: "d", Body: "corpo", ReadOnly: true}
	_, content, err := renderAgent("opencode", agent)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
	}
	for _, want := range []string{"mode: subagent", "permission:", "edit: deny"} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
}

func TestRenderOpenCodeWritableHasNoPermission(t *testing.T) {
	agent := agentSource{Name: "x", Description: "d", Body: "corpo"}
	_, content, err := renderAgent("opencode", agent)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
	}
	if strings.Contains(content, "permission:") {
		t.Fatalf("não deveria ter permission: %q", content)
	}
}

func TestRenderAntigravity(t *testing.T) {
	agent := agentSource{Name: "x", Description: "d", Body: "corpo"}
	_, content, err := renderAgent("antigravity", agent)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
	}
	for _, want := range []string{"model: inherit", "subagent: true"} {
		if !strings.Contains(content, want) {
			t.Errorf("faltando %q em:\n%s", want, content)
		}
	}
}

func TestRenderCodexReadonlyAndEscaping(t *testing.T) {
	agent := agentSource{Name: "x", Description: `aspas "no" meio`, Body: `regex \d+ e "aspas"`, ReadOnly: true}
	filename, content, err := renderAgent("codex", agent)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
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
	agent := agentSource{Name: "x", Description: "d", Body: "corpo"}
	_, content, err := renderAgent("codex", agent)
	if err != nil {
		t.Fatalf("renderAgent: %v", err)
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
	agents, err := loadAgents(dir)
	if err != nil {
		t.Fatalf("loadAgents: %v", err)
	}
	if len(agents) != 2 || agents[0].Name != "a" || agents[1].Name != "b" {
		t.Fatalf("ordem/quantidade inesperada: %+v", agents)
	}
}
