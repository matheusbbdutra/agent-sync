package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeSkill(t *testing.T, baseDir, id, content string) {
	t.Helper()
	dir := filepath.Join(baseDir, "skills", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSkillsLintAllClean(t *testing.T) {
	baseDir := t.TempDir()
	makeSkill(t, baseDir, "foo", "---\nname: foo\ndescription: \"Use when fooing.\"\n---\n\n# Foo\n")
	makeSkill(t, baseDir, "bar", "---\nname: bar\ndescription: \"Design bars with baz.\"\n---\n\n# Bar\n")

	if err := RunLintWithBase(nil, baseDir); err != nil {
		t.Fatalf("esperava err=nil, got %v", err)
	}
}

func TestSkillsLintMissingFrontmatter(t *testing.T) {
	baseDir := t.TempDir()
	makeSkill(t, baseDir, "broken", "# No frontmatter here\n")
	err := RunLintWithBase(nil, baseDir)
	if err == nil {
		t.Fatal("esperava err por falta de frontmatter")
	}
	if !strings.Contains(err.Error(), "error") {
		t.Fatalf("mensagem deveria mencionar error: %v", err)
	}
}

func TestSkillsLintNameMismatch(t *testing.T) {
	baseDir := t.TempDir()
	makeSkill(t, baseDir, "alpha", "---\nname: beta\ndescription: \"Use when test.\"\n---\n\n# X\n")
	err := RunLintWithBase(nil, baseDir)
	if err == nil {
		t.Fatal("esperava err por name != pasta")
	}
}

func TestSkillsLintDescriptionMissingTrigger(t *testing.T) {
	baseDir := t.TempDir()
	makeSkill(t, baseDir, "notrigger", "---\nname: notrigger\ndescription: \"coisas aleatorias sem verbo nem when\"\n---\n\n# X\n")
	err := RunLintWithBase(nil, baseDir)
	if err == nil {
		t.Fatal("esperava err por description sem gatilho")
	}
}

func TestSkillsLintJSONOutput(t *testing.T) {
	baseDir := t.TempDir()
	makeSkill(t, baseDir, "clean", "---\nname: clean\ndescription: \"Use when clean.\"\n---\n")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunLintWithBase([]string{"-json"}, baseDir)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("clean nao deveria falhar: %v", err)
	}
	var buf [4096]byte
	n, _ := r.Read(buf[:])
	out := string(buf[:n])

	var res SkillLintResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("JSON invalido: %v\noutput: %s", err, out)
	}
	if res.Total != 1 {
		t.Errorf("esperava total=1, got %d", res.Total)
	}
	if res.Errors != 0 {
		t.Errorf("esperava errors=0, got %d (issues=%v)", res.Errors, res.Issues)
	}
}

func TestSkillsLintAggregatesMultipleIssues(t *testing.T) {
	baseDir := t.TempDir()
	makeSkill(t, baseDir, "twoerrs", "---\nname: wrongname\ndescription: \"lorem ipsum sem verbo\"\n---\n")
	err := RunLintWithBase(nil, baseDir)
	if err == nil {
		t.Fatal("esperava err com multiplos problemas")
	}
}

func TestSkillsLintRealManifestPasses(t *testing.T) {
	if err := RunLintWithBase(nil, ""); err != nil {
		t.Fatalf("repo real deveria passar lint: %v", err)
	}
}

// Colisão de gatilhos: descriptions praticamente idênticas competem pelo mesmo
// prompt, então o lint sinaliza warning (sem falhar o gate: Errors continua 0).
func TestSkillsLintCollisionWarning(t *testing.T) {
	baseDir := t.TempDir()
	desc := "Use when reviewing authentication flows, validating input and auditing security sensitive code."
	makeSkill(t, baseDir, "alpha-security", "---\nname: alpha-security\ndescription: \""+desc+"\"\n---\n\n# A\n")
	makeSkill(t, baseDir, "beta-security", "---\nname: beta-security\ndescription: \""+desc+"\"\n---\n\n# B\n")

	res, err := CollectLint(baseDir)
	if err != nil {
		t.Fatalf("CollectLint falhou: %v", err)
	}
	if res.Errors != 0 {
		t.Fatalf("colisão deveria ser warning, nao error (issues=%+v)", res.Issues)
	}
	found := false
	for _, iss := range res.Issues {
		if iss.Check == "collision" && iss.Severity == "warning" {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperava warning de collision, issues=%+v", res.Issues)
	}
	if err := RunLintWithBase(nil, baseDir); err != nil {
		t.Fatalf("warning de colisao nao pode quebrar o gate: %v", err)
	}
}

func TestSkillsLintNoCollisionForDistinctDescriptions(t *testing.T) {
	baseDir := t.TempDir()
	makeSkill(t, baseDir, "foo", "---\nname: foo\ndescription: \"Use when fooing a quux.\"\n---\n")
	makeSkill(t, baseDir, "bar", "---\nname: bar\ndescription: \"Design bars with baz.\"\n---\n")

	res, err := CollectLint(baseDir)
	if err != nil {
		t.Fatalf("CollectLint falhou: %v", err)
	}
	if res.Warnings != 0 {
		t.Fatalf("nao esperava warnings, issues=%+v", res.Issues)
	}
}
