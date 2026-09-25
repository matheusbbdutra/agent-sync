package doctor

// doctor.go: subcommand `agent-sync doctor` (Entrega 2 / OmniRoute #5C).
//
// Diagnóstico centralizado e observabilidade de integridade do ambiente
// agent-sync. Valida 6 subsistemas reusando componentes existentes:
//   1. skills_index: manifest.json × disco (installed=total, missing=0, orphan=0)
//   2. skills_lint: conformidade do SKILL.md (frontmatter, name, gatilhos)
//   3. schemas: integridade e compilação dos 5 schemas JSON embarcados
//   4. opencode_plugins: presença dos plugins v2 e runtime Node/OpenCode
//   5. binaries: presença e permissões executáveis dos binários em ~/.local/bin/
//   6. agents_md: paridade e ausência de drift em relação a rules/global-rules.md
//
// Exit code: 0 se todos os checks PASS/WARN; 1 se qualquer check FAIL.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusdutra/agent-sync/internal/opencode"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/agent-sync/internal/skills"
	"github.com/matheusdutra/agent-sync/internal/target"
	"github.com/matheusdutra/token-tools/jsonschema"
)

type doctorCheckStatus string

const (
	doctorStatusPass doctorCheckStatus = "PASS"
	doctorStatusWarn doctorCheckStatus = "WARN"
	doctorStatusFail doctorCheckStatus = "FAIL"
)

type doctorCheckResult struct {
	Name   string            `json:"name"`
	Status doctorCheckStatus `json:"status"`
	Detail string            `json:"detail"`
	Issues []string          `json:"issues,omitempty"`
}

type doctorReport struct {
	Checks []doctorCheckResult `json:"checks"`
	Total  int                 `json:"total"`
	Passed int                 `json:"passed"`
	Warned int                 `json:"warned"`
	Failed int                 `json:"failed"`
}

type doctorEnv struct {
	baseDir           string
	homeDir           string
	localBinDir       string
	opencodePluginDir string
}

var knownDoctorSchemas = []string{
	"agent_tasks",
	"precompact-snapshot",
	"session-event",
	"session-state",
	"token-budget-status",
}

var expectedDoctorOpenCodePlugins = []string{
	"ctx-compact.ts",
	"precompact-snapshot.ts",
	"token-nudge.ts",
	"ctx-window-nudge.ts",
	"ctx-window-summarize-at-stop.ts",
	"context-guard-nudge.ts",
	"memory-nudge.ts",
	"agent-react-nudge.ts",
	"docs-cache.ts",
	"repo-map-warmup.ts",
	"memory-pipeline.ts",
}

var expectedDoctorBinaries = []string{
	"agent-sync",
	"agent-sync-session",
	"ast-outline",
	"ctx-window",
	"db-guardian",
	"delegate-run",
	"docs-cache-write",
	"docs-fetch",
	"docs-mcp",
	"false-success-guard",
	"git-diff-summary",
	"memory-mcp",
	"memory-sync",
	"mr-collect-cli",
	"mr-review-local",
	"repo-map",
	"shell-validate",
	"trace-strip",
}

func checkSkillsIndex(env doctorEnv) doctorCheckResult {
	res, err := skills.CollectIndex(env.baseDir)
	if err != nil {
		return doctorCheckResult{
			Name:   "skills_index",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("falha ao coletar skills: %v", err),
		}
	}

	var issues []string
	if res.Missing > 0 || res.Orphan > 0 {
		for _, s := range res.Skills {
			if s.Status == skills.StatusMissing {
				issues = append(issues, fmt.Sprintf("faltando: %s", s.ID))
			} else if s.Status == skills.StatusOrphan {
				issues = append(issues, fmt.Sprintf("órfã: %s", s.ID))
			}
		}
		return doctorCheckResult{
			Name:   "skills_index",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("%d/%d instaladas (%d faltando, %d órfãs)", res.Installed, res.Total, res.Missing, res.Orphan),
			Issues: issues,
		}
	}

	return doctorCheckResult{
		Name:   "skills_index",
		Status: doctorStatusPass,
		Detail: fmt.Sprintf("%d/%d skills instaladas (0 faltando, 0 órfãs)", res.Installed, res.Total),
	}
}

func checkSkillsLint(env doctorEnv) doctorCheckResult {
	res, err := skills.CollectLint(env.baseDir)
	if err != nil {
		return doctorCheckResult{
			Name:   "skills_lint",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("falha ao validar skills: %v", err),
		}
	}

	var issues []string
	for _, iss := range res.Issues {
		issues = append(issues, fmt.Sprintf("[%s] %s: %s (%s)", iss.Severity, iss.Skill, iss.Message, iss.Check))
	}

	if res.Errors > 0 {
		return doctorCheckResult{
			Name:   "skills_lint",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("%d erro(s) em %d skills", res.Errors, res.Total),
			Issues: issues,
		}
	}

	if res.Warnings > 0 {
		return doctorCheckResult{
			Name:   "skills_lint",
			Status: doctorStatusWarn,
			Detail: fmt.Sprintf("%d skills verificadas (0 erros, %d avisos)", res.Total, res.Warnings),
			Issues: issues,
		}
	}

	return doctorCheckResult{
		Name:   "skills_lint",
		Status: doctorStatusPass,
		Detail: fmt.Sprintf("%d skills verificadas (0 erros)", res.Total),
	}
}

func checkSchemas(env doctorEnv) doctorCheckResult {
	var issues []string
	failedCount := 0

	for _, name := range knownDoctorSchemas {
		if _, err := jsonschema.Load(name); err != nil {
			failedCount++
			issues = append(issues, fmt.Sprintf("schema %q: %v", name, err))
		}
	}

	if failedCount > 0 {
		return doctorCheckResult{
			Name:   "schemas",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("%d/%d schemas JSON compilados com sucesso", len(knownDoctorSchemas)-failedCount, len(knownDoctorSchemas)),
			Issues: issues,
		}
	}

	return doctorCheckResult{
		Name:   "schemas",
		Status: doctorStatusPass,
		Detail: fmt.Sprintf("%d/%d schemas JSON válidos e compilados", len(knownDoctorSchemas), len(knownDoctorSchemas)),
	}
}

func checkOpenCodePlugins(env doctorEnv) doctorCheckResult {
	pluginDir := env.opencodePluginDir
	if pluginDir == "" {
		pluginDir = filepath.Join(env.homeDir, ".config", "opencode", "plugins")
	}

	var issues []string
	missingCount := 0

	for _, p := range expectedDoctorOpenCodePlugins {
		targetPath := filepath.Join(pluginDir, p)
		if _, err := os.Stat(targetPath); err != nil {
			missingCount++
			issues = append(issues, fmt.Sprintf("plugin ausente: %s", p))
		}
	}

	// Runtime issues (Node >= 20.11, OpenCode >= 2.0.0, @opencode/plugin)
	rtIssues := opencode.CheckRuntime(pluginDir)
	for _, rti := range rtIssues {
		issues = append(issues, fmt.Sprintf("runtime [%s]: %s (hint: %s)", rti.Kind, rti.Detail, rti.Hint))
	}

	totalPlugins := len(expectedDoctorOpenCodePlugins)
	if missingCount > 0 {
		return doctorCheckResult{
			Name:   "opencode_plugins",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("%d/%d plugins v2 presentes em %s", totalPlugins-missingCount, totalPlugins, pluginDir),
			Issues: issues,
		}
	}

	if len(rtIssues) > 0 {
		return doctorCheckResult{
			Name:   "opencode_plugins",
			Status: doctorStatusWarn,
			Detail: fmt.Sprintf("%d/%d plugins presentes, mas runtime tem %d pendência(s)", totalPlugins, totalPlugins, len(rtIssues)),
			Issues: issues,
		}
	}

	return doctorCheckResult{
		Name:   "opencode_plugins",
		Status: doctorStatusPass,
		Detail: fmt.Sprintf("%d/%d plugins v2 instalados e runtime OK", totalPlugins, totalPlugins),
	}
}

func checkBinaries(env doctorEnv) doctorCheckResult {
	binDir := env.localBinDir
	if binDir == "" {
		binDir = filepath.Join(env.homeDir, ".local", "bin")
	}

	var issues []string
	missingCount := 0

	for _, name := range expectedDoctorBinaries {
		targetPath := filepath.Join(binDir, name)
		info, err := os.Stat(targetPath)
		if err != nil {
			missingCount++
			issues = append(issues, fmt.Sprintf("binário ausente: %s", name))
			continue
		}
		if info.IsDir() || (info.Mode()&0111 == 0) {
			missingCount++
			issues = append(issues, fmt.Sprintf("arquivo não executável: %s", name))
		}
	}

	total := len(expectedDoctorBinaries)
	if missingCount > 0 {
		return doctorCheckResult{
			Name:   "binaries",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("%d/%d binários presentes em %s", total-missingCount, total, binDir),
			Issues: issues,
		}
	}

	return doctorCheckResult{
		Name:   "binaries",
		Status: doctorStatusPass,
		Detail: fmt.Sprintf("%d/%d binários instalados e executáveis", total, total),
	}
}

func checkAgentsMd(env doctorEnv) doctorCheckResult {
	canonPath := filepath.Join(env.baseDir, "rules", "global-rules.md")
	canonRaw, err := os.ReadFile(canonPath)
	if err != nil {
		return doctorCheckResult{
			Name:   "agents_md",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("não conseguiu ler regra canônica %s: %v", canonPath, err),
		}
	}

	canonHashBytes := sha256.Sum256(canonRaw)
	canonHash := hex.EncodeToString(canonHashBytes[:])

	targets := target.GetTargetsForHome(env.homeDir)
	var issues []string
	driftCount := 0

	for _, t := range targets {
		raw, err := os.ReadFile(t.RulesPath)
		if err != nil {
			driftCount++
			issues = append(issues, fmt.Sprintf("%s: arquivo ausente em %s", t.Name, t.RulesPath))
			continue
		}
		hBytes := sha256.Sum256(raw)
		h := hex.EncodeToString(hBytes[:])
		if h != canonHash {
			driftCount++
			issues = append(issues, fmt.Sprintf("%s: drift detectado em %s (hash %s != %s)", t.Name, t.RulesPath, h[:8], canonHash[:8]))
		}
	}

	totalTargets := len(targets)
	if driftCount > 0 {
		return doctorCheckResult{
			Name:   "agents_md",
			Status: doctorStatusFail,
			Detail: fmt.Sprintf("%d/%d targets com drift ou ausência", driftCount, totalTargets),
			Issues: issues,
		}
	}

	return doctorCheckResult{
		Name:   "agents_md",
		Status: doctorStatusPass,
		Detail: fmt.Sprintf("%d/%d targets sincronizados com rules/global-rules.md (sem drift)", totalTargets, totalTargets),
	}
}

func runDoctor(env doctorEnv) doctorReport {
	checkFns := []func(doctorEnv) doctorCheckResult{
		checkSkillsIndex,
		checkSkillsLint,
		checkSchemas,
		checkOpenCodePlugins,
		checkBinaries,
		checkAgentsMd,
	}

	report := doctorReport{
		Checks: make([]doctorCheckResult, 0, len(checkFns)),
		Total:  len(checkFns),
	}

	for _, fn := range checkFns {
		res := fn(env)
		report.Checks = append(report.Checks, res)
		switch res.Status {
		case doctorStatusPass:
			report.Passed++
		case doctorStatusWarn:
			report.Warned++
		case doctorStatusFail:
			report.Failed++
		}
	}

	return report
}

func printDoctorReport(report doctorReport) {
	fmt.Printf("🔍 Agent-Sync Doctor: %d checks executados (%d PASS, %d WARN, %d FAIL)\n\n",
		report.Total, report.Passed, report.Warned, report.Failed)

	fmt.Printf("%-20s  %-6s  %s\n", "CHECK", "STATUS", "DETAIL")
	fmt.Println(strings.Repeat("-", 80))

	for _, c := range report.Checks {
		icon := "✅"
		if c.Status == doctorStatusWarn {
			icon = "⚠️ "
		} else if c.Status == doctorStatusFail {
			icon = "❌"
		}
		fmt.Printf("%-20s  %s %-4s  %s\n", c.Name, icon, c.Status, c.Detail)
		for _, iss := range c.Issues {
			fmt.Printf("  └─ %s\n", iss)
		}
	}
	fmt.Println()
}

func RunCommand(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Saída em JSON estruturado")
	if err := fs.Parse(args); err != nil {
		return err
	}

	exePath, _ := os.Executable()
	cwd, _ := os.Getwd()
	baseDir, err := pathutil.ResolveBaseDir(exePath, cwd, os.Getenv("AGENT_SYNC_HOME"))
	if err != nil {
		return err
	}
	home := target.GetHome()

	env := doctorEnv{
		baseDir:           baseDir,
		homeDir:           home,
		localBinDir:       filepath.Join(home, ".local", "bin"),
		opencodePluginDir: filepath.Join(home, ".config", "opencode", "plugins"),
	}

	report := runDoctor(env)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printDoctorReport(report)
	if report.Failed > 0 {
		return fmt.Errorf("%d check(s) com falha", report.Failed)
	}
	return nil
}
