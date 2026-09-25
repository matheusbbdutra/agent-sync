package apply

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/matheusdutra/agent-sync/internal/hooks"
	"github.com/matheusdutra/agent-sync/internal/pathutil"
	"github.com/matheusdutra/agent-sync/internal/target"
)

var (
	dryRunEnabled  bool
	dryRunEnvCache bool
)

// shouldDryRun combina a flag CLI `-dry-run`/`-n` com a env var AGENT_SYNC_DRY_RUN=1.
func shouldDryRun() bool {
	return dryRunEnabled || dryRunEnvCache
}

// RunCommand executa o fluxo principal do agent-sync (apply, status, vendor, observability).
func RunCommand(args []string) error {
	fs := flag.NewFlagSet("agent-sync", flag.ContinueOnError)
	applyFlag := fs.Bool("apply", false, "Aplica as regras e skills para todas as CLIs configuradas")
	targetFlag := fs.String("target", "", "Aplica para uma CLI específica (claude, codex, antigravity, opencode, cursor)")
	statusFlag := fs.Bool("status", false, "Exibe o status de sincronização com as CLIs")
	vendorFlag := fs.Bool("vendor", false, "Importa as skills curadas do catálogo definido em skills/manifest.json")
	observabilityFlag := fs.Bool("observability", false, "Exibe resumo dos erros persistidos pelos hooks (texto)")
	observabilityJSONFlag := fs.Bool("observability-json", false, "Exibe resumo dos erros persistidos pelos hooks em JSON estruturado")
	sourceFlag := fs.String("source", "", "Diretório de origem das skills para -vendor (default: skillsDir do manifest)")
	dryRunFlag := fs.Bool("dry-run", false, "Mostra o que seria feito sem escrever em disco (alias: -n)")
	dryRunShortFlag := fs.Bool("n", false, "Alias curto para -dry-run")

	if err := fs.Parse(args); err != nil {
		return err
	}

	dryRunEnabled = *dryRunFlag || *dryRunShortFlag
	dryRunEnvCache = os.Getenv("AGENT_SYNC_DRY_RUN") == "1"
	hooks.SetDryRun(shouldDryRun())

	exePath, _ := os.Executable()
	cwd, _ := os.Getwd()
	baseDir, err := pathutil.ResolveBaseDir(exePath, cwd, os.Getenv("AGENT_SYNC_HOME"))
	if err != nil {
		return err
	}

	rulesSource := filepath.Join(baseDir, "rules", "global-rules.md")
	skillsSource := filepath.Join(baseDir, "skills")

	if !*applyFlag && !*statusFlag && !*vendorFlag && !*observabilityFlag && !*observabilityJSONFlag && *targetFlag == "" {
		printUsage()
		return nil
	}

	if *vendorFlag {
		return runVendor(baseDir, *sourceFlag)
	}

	targets := target.GetTargets()
	if *observabilityJSONFlag {
		return printHookObservabilityJSON()
	}

	if *observabilityFlag {
		return printHookObservability()
	}

	if *statusFlag {
		printStatus(targets)
		return nil
	}

	fmt.Println("🔄 Iniciando sincronização...")
	if shouldDryRun() {
		fmt.Println("⚠️  Modo dry-run: nenhuma escrita em disco será feita.")
	}

	filtered := make([]target.TargetCLI, 0, len(targets))
	for _, t := range targets {
		if *targetFlag == "" || *targetFlag == t.Name {
			filtered = append(filtered, t)
		}
	}

	ctx := applyContext{
		baseDir:      baseDir,
		rulesSource:  rulesSource,
		skillsSource: skillsSource,
		log:          nil,
	}
	allLogs := make([][]string, len(filtered))

	var wg sync.WaitGroup
	for i := range filtered {
		i, t := i, filtered[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerLog := &workerLog{}
			c := ctx
			c.log = workerLog
			applyToTarget(c, t)
			allLogs[i] = workerLog.lines()
		}()
	}
	wg.Wait()

	for _, lines := range allLogs {
		for _, line := range lines {
			fmt.Println(line)
		}
	}

	if err := persistShellEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Falha ao persistir env no shell rc: %v\n", err)
	}

	fmt.Printf("\n✨ Concluído! %d CLI(s) sincronizada(s) com sucesso.\n", len(filtered))
	return nil
}

func printUsage() {
	fmt.Println("🚀 Agent-Sync: Gerenciador Unificado de Regras e Skills para Agentes AI")
	fmt.Println("\nUso:")
	fmt.Println("  agent-sync -apply              # Sincroniza em todas as CLIs instaladas")
	fmt.Println("  agent-sync -target <cli>       # Sincroniza apenas para claude, codex, antigravity, opencode ou cursor")
	fmt.Println("  agent-sync -status             # Verifica o status atual de cada CLI")
	fmt.Println("  agent-sync -vendor             # Importa as skills curadas do manifest")
	fmt.Println("  agent-sync -observability      # Resume erros persistidos pelos hooks")
	fmt.Println("  agent-sync -observability-json # Idem, saída JSON estruturada")
	fmt.Println("  agent-sync skills index        # Lista skills (manifest × disco) — instaladas/órfãs/faltantes")
	fmt.Println("  agent-sync doctor              # Diagnóstico de integridade (schemas, skills, plugins, binários, regras)")
}

func printStatus(targets []target.TargetCLI) {
	fmt.Println("📊 Status de Sincronização:")
	for _, t := range targets {
		rulesStatus := "❌ Não encontrado"
		if _, err := os.Stat(t.RulesPath); err == nil {
			rulesStatus = "✅ Presente"
		}
		skillsCount := 0
		if entries, err := os.ReadDir(t.SkillsDir); err == nil {
			for _, e := range entries {
				if e.IsDir() || (e.Type()&os.ModeSymlink != 0) {
					skillsCount++
				}
			}
		}
		agentsCount := 0
		if entries, err := os.ReadDir(t.AgentsDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					agentsCount++
				}
			}
		}
		fmt.Printf(" - %-11s | Regras: %-16s | Skills: %d instaladas | Agentes: %d\n", t.Name, rulesStatus, skillsCount, agentsCount)
	}
}
