package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureSkill cria um diretório de skill com SKILL.md válido.
func fixtureSkill(t *testing.T, root, id string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\nname: " + id + "\ndescription: a valid skill\n---\n\n## Use this skill when\n\n- x\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return dir
}

// fixtureEmptySkill cria um diretório de skill com SKILL.md VAZIO (caso perigoso).
func fixtureEmptySkill(t *testing.T, root, id string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte{}, 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return dir
}

// TestVendorSkill_KeepsDstWhenSrcMissing garante que dst NÃO é modificado
// quando src não existe (cenário do bug: source dir inexistente).
func TestVendorSkill_KeepsDstWhenSrcMissing(t *testing.T) {
	tmp := t.TempDir()
	dst := fixtureSkill(t, filepath.Join(tmp, "dst"), "my-skill")
	src := filepath.Join(tmp, "src-nonexistent", "my-skill")

	err := vendorSkill(src, dst, "my-skill")
	if err == nil {
		t.Fatal("esperava erro quando src não existe")
	}
	if !strings.Contains(err.Error(), "SKILL.md ausente") {
		t.Fatalf("erro inesperado: %v", err)
	}

	// dst deve permanecer intacto
	if _, statErr := os.Stat(filepath.Join(dst, "SKILL.md")); statErr != nil {
		t.Fatalf("dst foi removido mesmo com src faltando: %v", statErr)
	}
}

// TestVendorSkill_KeepsDstWhenSrcEmpty garante que dst NÃO é modificado
// quando src/SKILL.md está VAZIO (foot-gun: sobrescrevia skills do projeto).
func TestVendorSkill_KeepsDstWhenSrcEmpty(t *testing.T) {
	tmp := t.TempDir()
	dst := fixtureSkill(t, filepath.Join(tmp, "dst"), "my-skill")
	src := fixtureEmptySkill(t, filepath.Join(tmp, "src"), "my-skill")

	originalContent, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))

	err := vendorSkill(src, dst, "my-skill")
	if err == nil {
		t.Fatal("esperava erro quando src/SKILL.md está vazio")
	}
	// erro pode ser "frontmatter ausente" ou similar; o que importa é dst intacto

	currentContent, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if string(currentContent) != string(originalContent) {
		t.Fatalf("dst foi sobrescrito mesmo com src vazio")
	}
}

// TestVendorSkill_KeepsDstWhenSrcInvalidFrontmatter garante que dst NÃO é
// modificado quando src tem frontmatter inválido.
func TestVendorSkill_KeepsDstWhenSrcInvalidFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	dst := fixtureSkill(t, filepath.Join(tmp, "dst"), "my-skill")

	src := filepath.Join(tmp, "src", "my-skill")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "isto não é frontmatter válido\n"
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	originalContent, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))

	err := vendorSkill(src, dst, "my-skill")
	if err == nil {
		t.Fatal("esperava erro com frontmatter inválido")
	}

	currentContent, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if string(currentContent) != string(originalContent) {
		t.Fatalf("dst foi sobrescrito mesmo com frontmatter inválido")
	}
}

// TestVendorSkill_SuccessPath garante que o caminho feliz continua funcionando:
// dst é substituído atomicamente com o conteúdo de src.
func TestVendorSkill_SuccessPath(t *testing.T) {
	tmp := t.TempDir()
	src := fixtureSkill(t, filepath.Join(tmp, "src"), "my-skill")
	dst := filepath.Join(tmp, "dst", "my-skill")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := vendorSkill(src, dst, "my-skill"); err != nil {
		t.Fatalf("vendorSkill falhou no caminho feliz: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if err != nil {
		t.Fatalf("SKILL.md não foi criado: %v", err)
	}
	if !strings.Contains(string(content), "name: my-skill") {
		t.Fatalf("conteúdo inesperado: %s", content)
	}
}

// TestRunVendor_AbortsWhenSourceDirMissing garante que vendor aborta antes
// de iterar skills se o diretório de origem não existe.
func TestRunVendor_AbortsWhenSourceDirMissing(t *testing.T) {
	tmp := t.TempDir()
	baseDir := tmp

	// cria pasta skills/ (loadManifest exige)
	if err := os.MkdirAll(filepath.Join(baseDir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"source": {"skillsDir": "~/definitely-does-not-exist-xyz"},
		"skills": [{"id": "my-skill", "domain": "test"}]
	}`
	if err := os.WriteFile(filepath.Join(baseDir, "skills", "manifest.json"),
		[]byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := fixtureSkill(t, filepath.Join(baseDir, "skills"), "my-skill")
	originalContent, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))

	err := runVendor(baseDir, "")
	if err == nil {
		t.Fatal("esperava erro quando sourceDir não existe")
	}

	currentContent, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if string(currentContent) != string(originalContent) {
		t.Fatalf("dst foi modificado mesmo com sourceDir inexistente")
	}
}

// TestRunVendor_AbortsWhenAllSkillsHaveEmptySource garante que vendor aborta
// antes de deletar destinos se todos os sources têm SKILL.md vazio (o bug
// real: 54 skills zeradas em produção).
func TestRunVendor_AbortsWhenAllSkillsHaveEmptySource(t *testing.T) {
	tmp := t.TempDir()
	baseDir := tmp

	// cria pasta skills/ (loadManifest exige)
	if err := os.MkdirAll(filepath.Join(baseDir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
		"source": {"skillsDir": "` + filepath.Join(tmp, "src") + `"},
		"skills": [
			{"id": "skill-a", "domain": "test"},
			{"id": "skill-b", "domain": "test"}
		]
	}`
	if err := os.WriteFile(filepath.Join(baseDir, "skills", "manifest.json"),
		[]byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// source: ambos com SKILL.md vazio (simula o bug)
	fixtureEmptySkill(t, filepath.Join(tmp, "src"), "skill-a")
	fixtureEmptySkill(t, filepath.Join(tmp, "src"), "skill-b")

	// destinations: ambos com SKILL.md válido (proteção que deve permanecer)
	dstA := fixtureSkill(t, filepath.Join(baseDir, "skills"), "skill-a")
	dstB := fixtureSkill(t, filepath.Join(baseDir, "skills"), "skill-b")
	origA, _ := os.ReadFile(filepath.Join(dstA, "SKILL.md"))
	origB, _ := os.ReadFile(filepath.Join(dstB, "SKILL.md"))

	err := runVendor(baseDir, "")
	if err == nil {
		t.Fatal("esperava erro quando todos os sources estão vazios")
	}

	curA, _ := os.ReadFile(filepath.Join(dstA, "SKILL.md"))
	curB, _ := os.ReadFile(filepath.Join(dstB, "SKILL.md"))
	if string(curA) != string(origA) {
		t.Fatalf("skill-a foi sobrescrita mesmo com source vazio")
	}
	if string(curB) != string(origB) {
		t.Fatalf("skill-b foi sobrescrita mesmo com source vazio")
	}
}

// TestVendorSkill_KeepsDstWhenSrcHasBrokenSubdir garante que dst fica
// intacto se src tem estrutura parcial (ex.: SKILL.md válido mas
// subdiretórios quebrados que fazem copyDir falhar).
func TestVendorSkill_KeepsDstWhenSrcHasBrokenSubdir(t *testing.T) {
	tmp := t.TempDir()
	dst := fixtureSkill(t, filepath.Join(tmp, "dst"), "my-skill")
	src := fixtureSkill(t, filepath.Join(tmp, "src"), "my-skill")

	// cria um symlink inválido dentro de src (vai quebrar o filepath.Walk)
	if err := os.Symlink("/proc/this/does/not/exist",
		filepath.Join(src, "broken-symlink")); err != nil {
		t.Skipf("não foi possível criar symlink: %v", err)
	}

	originalContent, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))

	err := vendorSkill(src, dst, "my-skill")
	// erro é esperado; o que importa é dst intacto
	if err == nil {
		t.Log("copyDir completou apesar do symlink quebrado (pode ser tolerante em alguns FS)")
	}

	currentContent, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if string(currentContent) != string(originalContent) {
		t.Fatalf("dst foi modificado: %q -> %q", originalContent, currentContent)
	}
}

// TestVendorSkill_LeavesNoTempDir garante que após sucesso, o diretório
// temporário (.new-<id>) foi renomeado e não fica orfão.
func TestVendorSkill_LeavesNoTempDir(t *testing.T) {
	tmp := t.TempDir()
	src := fixtureSkill(t, filepath.Join(tmp, "src"), "my-skill")
	dst := filepath.Join(tmp, "dst", "my-skill")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := vendorSkill(src, dst, "my-skill"); err != nil {
		t.Fatalf("vendorSkill falhou: %v", err)
	}

	parent := filepath.Dir(dst)
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".new-") {
			t.Fatalf("diretório temp órfão encontrado: %s", e.Name())
		}
	}
}
