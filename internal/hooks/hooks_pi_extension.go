package hooks

// hooks_pi_extension.go: wirar do Pi (coding agent harness).
//
// Pi nao tem hooks declarativos em JSON. O ponto de extensao nativo sao
// extensoes TS carregadas de <agent-dir>/extensions/. A extensao do
// agent-sync e um adapter fino (regra §4): toda a decisao vive no binario Go
// via `agent-sync hook pi --event=<evento>`. Aqui so copiamos o arquivo para o
// diretorio de descoberta do Pi e gravamos o sidecar de configuracao
// (binario + base dir) que a extensao usa em runtime.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// piExtensionFileName e o nome do arquivo dentro de <agent-dir>/extensions/.
	// Pi descobre qualquer *.ts no diretorio (um nivel de profundidade).
	piExtensionFileName = "agent-sync.ts"
	// piConfigFileName e o sidecar lido pela extensao. Evita depender de PATH
	// (que pode resolver um agent-sync antigo) e de AGENT_SYNC_HOME exportado.
	piConfigFileName = "pi-agent-sync.json"
)

type piExtensionConfig struct {
	AgentSyncBin string `json:"agent_sync_bin"`
	BaseDir      string `json:"base_dir"`
}

// syncPiExtension instala o adapter TS do Pi em HookExtensionDir.
func syncPiExtension(baseDir string, target TargetCLI) error {
	if target.HookExtensionDir == "" {
		return nil
	}
	src := filepath.Join(baseDir, "hooks", "pi-extension.ts")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("extensão do Pi não encontrada: %s", src)
	}

	exePath, err := os.Executable()
	if err != nil {
		exePath = filepath.Join(baseDir, "agent-sync")
	}
	cfg := piExtensionConfig{AgentSyncBin: exePath, BaseDir: baseDir}
	cfgRaw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	cfgRaw = append(cfgRaw, '\n')

	if shouldDryRun() {
		fmt.Printf("[dry-run] copy %s -> %s\n", src, filepath.Join(target.HookExtensionDir, piExtensionFileName))
		fmt.Printf("[dry-run] write json %s\n", filepath.Join(target.HookExtensionDir, piConfigFileName))
		return nil
	}
	if err := os.MkdirAll(target.HookExtensionDir, 0o755); err != nil {
		return err
	}
	if err := copyFile(src, filepath.Join(target.HookExtensionDir, piExtensionFileName)); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(target.HookExtensionDir, piConfigFileName), cfgRaw, 0o644)
}
