package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/matheusdutra/agent-sync/internal/target"
)

func setupTestDoctorEnv(t *testing.T) (doctorEnv, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	baseDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	localBinDir := filepath.Join(homeDir, ".local", "bin")
	opencodePluginDir := filepath.Join(homeDir, ".config", "opencode", "plugins")

	// 1. Criar regras canônicas
	rulesDir := filepath.Join(baseDir, "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canonRules := "# Regras Globais Canônicas\n"
	if err := os.WriteFile(filepath.Join(rulesDir, "global-rules.md"), []byte(canonRules), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Criar skills manifest e pastas
	skillsDir := filepath.Join(baseDir, "skills")
	if err := os.MkdirAll(filepath.Join(skillsDir, "test-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifestJSON := `{"skills":[{"id":"test-skill","domain":"test","bundleRef":"core"}]}`
	if err := os.WriteFile(filepath.Join(skillsDir, "manifest.json"), []byte(manifestJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: test-skill\ndescription: Use when testing doctor command\n---\n# Test Skill\n"
	if err := os.WriteFile(filepath.Join(skillsDir, "test-skill", "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Criar binários falsos executáveis em ~/.local/bin
	if err := os.MkdirAll(localBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, b := range expectedDoctorBinaries {
		binPath := filepath.Join(localBinDir, b)
		if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// 4. Criar plugins OpenCode falsos
	if err := os.MkdirAll(opencodePluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range expectedDoctorOpenCodePlugins {
		pluginPath := filepath.Join(opencodePluginDir, p)
		if err := os.WriteFile(pluginPath, []byte("// plugin\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// 5. Criar AGENTS.md sincronizado para todos os targets
	targets := target.GetTargetsForHome(homeDir)
	for _, tgt := range targets {
		if err := os.MkdirAll(filepath.Dir(tgt.RulesPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tgt.RulesPath, []byte(canonRules), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	env := doctorEnv{
		baseDir:           baseDir,
		homeDir:           homeDir,
		localBinDir:       localBinDir,
		opencodePluginDir: opencodePluginDir,
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return env, cleanup
}

func TestDoctorSkillsIndexCheck(t *testing.T) {
	env, cleanup := setupTestDoctorEnv(t)
	defer cleanup()

	// Inicialmente OK
	res := checkSkillsIndex(env)
	if res.Status != doctorStatusPass {
		t.Fatalf("esperava PASS, obteve %s: %s", res.Status, res.Detail)
	}

	// Adiciona skill órfã (pasta sem entrada no manifest)
	orphanDir := filepath.Join(env.baseDir, "skills", "orphan-skill")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resOrphan := checkSkillsIndex(env)
	if resOrphan.Status != doctorStatusFail {
		t.Fatalf("esperava FAIL para skill órfã, obteve %s", resOrphan.Status)
	}
	if len(resOrphan.Issues) == 0 {
		t.Fatalf("esperava issues listando órfã")
	}
}

func TestDoctorSkillsLintCheck(t *testing.T) {
	env, cleanup := setupTestDoctorEnv(t)
	defer cleanup()

	res := checkSkillsLint(env)
	if res.Status != doctorStatusPass {
		t.Fatalf("esperava PASS, obteve %s: %s", res.Status, res.Detail)
	}

	// Cria skill com frontmatter quebrado
	badSkillDir := filepath.Join(env.baseDir, "skills", "bad-skill")
	if err := os.MkdirAll(badSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badSkillDir, "SKILL.md"), []byte("sem frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}

	resBad := checkSkillsLint(env)
	if resBad.Status != doctorStatusFail {
		t.Fatalf("esperava FAIL para frontmatter ausente, obteve %s", resBad.Status)
	}
}

func TestDoctorSchemasCheck(t *testing.T) {
	env, cleanup := setupTestDoctorEnv(t)
	defer cleanup()

	res := checkSchemas(env)
	if res.Status != doctorStatusPass {
		t.Fatalf("esperava PASS para schemas embutidos, obteve %s: %s", res.Status, res.Detail)
	}
}

func TestDoctorBinariesCheck(t *testing.T) {
	env, cleanup := setupTestDoctorEnv(t)
	defer cleanup()

	res := checkBinaries(env)
	if res.Status != doctorStatusPass {
		t.Fatalf("esperava PASS para binários instalados, obteve %s: %s", res.Status, res.Detail)
	}

	// Remove um binário
	os.Remove(filepath.Join(env.localBinDir, "repo-map"))
	resMissing := checkBinaries(env)
	if resMissing.Status != doctorStatusFail {
		t.Fatalf("esperava FAIL após remover binário, obteve %s", resMissing.Status)
	}
}

func TestDoctorAgentsMdDriftCheck(t *testing.T) {
	env, cleanup := setupTestDoctorEnv(t)
	defer cleanup()

	res := checkAgentsMd(env)
	if res.Status != doctorStatusPass {
		t.Fatalf("esperava PASS para AGENTS.md sincronizados, obteve %s: %s", res.Status, res.Detail)
	}

	// Induz drift em um target (ex: claude)
	claudePath := filepath.Join(env.homeDir, ".claude", "AGENTS.md")
	if err := os.WriteFile(claudePath, []byte("# Drift não sincronizado\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resDrift := checkAgentsMd(env)
	if resDrift.Status != doctorStatusFail {
		t.Fatalf("esperava FAIL com drift de regra, obteve %s", resDrift.Status)
	}
	if len(resDrift.Issues) == 0 {
		t.Fatalf("esperava issues relatando drift do claude")
	}
}

func TestDoctorOpenCodePluginsCheck(t *testing.T) {
	env, cleanup := setupTestDoctorEnv(t)
	defer cleanup()

	// Remove um plugin
	os.Remove(filepath.Join(env.opencodePluginDir, "memory-pipeline.ts"))
	res := checkOpenCodePlugins(env)
	if res.Status != doctorStatusFail {
		t.Fatalf("esperava FAIL com plugin ausente, obteve %s", res.Status)
	}
}

func TestDoctorRunAllPass(t *testing.T) {
	env, cleanup := setupTestDoctorEnv(t)
	defer cleanup()

	report := runDoctor(env)
	if report.Total != 6 {
		t.Fatalf("esperava 6 checks, obteve %d", report.Total)
	}
	// Note: OpenCode runtime pode ter WARN se node/opencode não estiver configurado no ambiente de teste,
	// mas não deve ter FAIL.
	if report.Failed > 0 {
		t.Fatalf("esperava 0 falhas em setup limpo, obteve %d falha(s): %+v", report.Failed, report.Checks)
	}
}

func TestDoctorCanonicalSha256Matches(t *testing.T) {
	raw := []byte("# Regras\n")
	h := sha256.Sum256(raw)
	hexStr := hex.EncodeToString(h[:])
	if len(hexStr) != 64 {
		t.Fatalf("esperava hex sha256 de 64 chars, obteve %d", len(hexStr))
	}
}

func TestDoctorReportJSONMarshaling(t *testing.T) {
	report := doctorReport{
		Checks: []doctorCheckResult{
			{Name: "schemas", Status: doctorStatusPass, Detail: "5/5 schemas"},
		},
		Total:  1,
		Passed: 1,
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("falha ao serializar doctorReport: %v", err)
	}
	var decoded doctorReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("falha ao deserializar doctorReport: %v", err)
	}
	if decoded.Checks[0].Name != "schemas" || decoded.Passed != 1 {
		t.Fatalf("dados divergentes após round-trip JSON: %+v", decoded)
	}
}
