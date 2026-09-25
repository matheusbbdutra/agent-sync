package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSkillsNew_Success(t *testing.T) {
	tmpDir := t.TempDir()

	args := []string{"custom-worker", "--description", "Use when executing custom worker jobs in background", "--title", "Custom Worker Specialist"}
	if err := RunNewWithBase(args, tmpDir); err != nil {
		t.Fatalf("RunNewWithBase falhou: %v", err)
	}

	skillFile := filepath.Join(tmpDir, "skills", "custom-worker", "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("erro ao ler SKILL.md gerado: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "name: custom-worker") {
		t.Errorf("esperava 'name: custom-worker', obteve:\n%s", text)
	}
	if !strings.Contains(text, "description: \"Use when executing custom worker jobs in background\"") {
		t.Errorf("esperava description configurada, obteve:\n%s", text)
	}
	if !strings.Contains(text, "# Custom Worker Specialist") {
		t.Errorf("esperava '# Custom Worker Specialist', obteve:\n%s", text)
	}

	if err := RunLintWithBase(nil, tmpDir); err != nil {
		t.Errorf("a skill gerada deveria passar no linter, mas falhou: %v", err)
	}
}

func TestRunSkillsNew_MissingID(t *testing.T) {
	tmpDir := t.TempDir()
	args := []string{"--description", "Use when testing"}
	err := RunNewWithBase(args, tmpDir)
	if err == nil {
		t.Fatalf("esperava erro por falta de id, mas obteve nil")
	}
	if !strings.Contains(err.Error(), "id da skill é obrigatório") {
		t.Errorf("mensagem inesperada de erro: %v", err)
	}
}

func TestRunSkillsNew_InvalidID(t *testing.T) {
	tmpDir := t.TempDir()
	invalidIDs := []string{"Invalid_Name", "name with spaces", "-leading-dash", "trailing-dash-", "UPPERCASE"}

	for _, id := range invalidIDs {
		args := []string{id, "--description", "Use when testing"}
		err := RunNewWithBase(args, tmpDir)
		if err == nil {
			t.Errorf("esperava erro para id inválido %q, mas obteve nil", id)
		}
	}
}

func TestRunSkillsNew_MissingDescription(t *testing.T) {
	tmpDir := t.TempDir()
	args := []string{"valid-id"}
	err := RunNewWithBase(args, tmpDir)
	if err == nil {
		t.Fatalf("esperava erro por falta de description, mas obteve nil")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("mensagem inesperada de erro: %v", err)
	}
}

func TestRunSkillsNew_AlreadyExistsAndForce(t *testing.T) {
	tmpDir := t.TempDir()

	args := []string{"test-skill", "--description", "Use when testing first time"}
	if err := RunNewWithBase(args, tmpDir); err != nil {
		t.Fatalf("primeira criação falhou: %v", err)
	}

	argsDuplicate := []string{"test-skill", "--description", "Use when testing second time"}
	err := RunNewWithBase(argsDuplicate, tmpDir)
	if err == nil {
		t.Fatalf("esperava erro de skill já existente, mas obteve nil")
	}
	if !strings.Contains(err.Error(), "já existe") {
		t.Errorf("mensagem de erro inesperada: %v", err)
	}

	argsForce := []string{"test-skill", "--description", "Use when testing with force", "--force"}
	if err := RunNewWithBase(argsForce, tmpDir); err != nil {
		t.Fatalf("criação com --force falhou: %v", err)
	}

	skillFile := filepath.Join(tmpDir, "skills", "test-skill", "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("erro ao ler SKILL.md: %v", err)
	}
	if !strings.Contains(string(content), "Use when testing with force") {
		t.Errorf("conteúdo não foi atualizado após --force:\n%s", string(content))
	}
}
