package hooks

import (
	"path/filepath"

	"github.com/matheusdutra/agent-sync/internal/opencode"
)

// openCodeConfigFileRef retorna o comentario opcional de config do
// OpenCode; settingsPathDetail imprime path wirado; openCodePluginDetail*
// imprime nota/plugin detail.

func openCodeConfigFileRef(t TargetCLI) string {
	return filepath.Join(filepath.Dir(t.OpenCodePluginDir), openCodeConfigFile)
}

func settingsPathDetail(t TargetCLI) string {
	if t.HooksSettingsPath == "" {
		return ""
	}
	return "instalado em: " + t.HooksSettingsPath
}

func openCodePluginDetailNote(t TargetCLI) string {
	if opencode.MajorVersion() >= 2 {
		return "em " + t.OpenCodePluginDir + " (v2, ver README)"
	}
	return "em " + t.OpenCodePluginDir + " (best-effort, ver README)"
}

func openCodePluginDetail(t TargetCLI) string {
	if opencode.MajorVersion() >= 2 {
		return "em " + t.OpenCodePluginDir + " (v2)"
	}
	return "em " + t.OpenCodePluginDir + " (best-effort)"
}