package hooks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/matheusdutra/agent-sync/internal/opencode"
)

// hooks_opencode_plugin.go: OpenCode plugin versioned helper.

// Migrado de hooks.go em 2026-09-21 (Fase 2.3 do refator por feature).
// Sem mudanca de comportamento: mesma logica, mesmo package.

// syncOpenCodePluginVersioned: OpenCode plugin helper (v2-only).
// Instala o plugin TS no formato @opencode/plugin (Plugin.define + setup):
// tenta hooks/<base>.opencode.v2.ts (padrão novo) e depois hooks/<base>.v2.ts
// (padrão legado). v1 foi aposentado: MajorVersion < 2 agora retorna erro
// explícito (fail-fast) em vez de wirar plugin legado.
func syncOpenCodePluginVersioned(baseDir string, target TargetCLI, base, dst string, required bool) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	major := opencode.MajorVersion()
	if major < 2 {
		return fmt.Errorf("opencode v1 aposentado: atualize para >= v2.0.0 (ou AGENT_SYNC_OPENCODE_VERSION=2); hooks/*.opencode.ts removidos")
	}
	hooksDir := filepath.Join(baseDir, "hooks")
	// v2: tenta primeiro o padrão novo (.opencode.v2.ts), depois o legado
	// (.v2.ts). Compatibilidade com plugins legados que seguem o sufixo
	// curto sem perder os 4 plugins novos do A-14/A-16/A-17.
	for _, suffix := range []string{".opencode.v2.ts", ".v2.ts"} {
		candidate := filepath.Join(hooksDir, base+suffix)
		if _, err := os.Stat(candidate); err == nil {
			return copyFile(candidate, filepath.Join(target.OpenCodePluginDir, dst))
		}
	}
	if !required {
		return nil
	}
	return fmt.Errorf("plugin do hook não encontrado: %s{.opencode.v2.ts,.v2.ts}", base)
}

// syncOpenCodeCtxCompactPlugin: OpenCode plugin ctx-compact
// syncOpenCodeCtxCompactPlugin instala o plugin TS para tracking e para o
// callback de compactação nativa. A limitação #13574 afeta tool.execute.after
// apenas na linha v1; em v2 a injeção vai via ctx.session.hook("context").
func syncOpenCodeCtxCompactPlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "ctx-compact", "ctx-compact.ts", true)
}

// syncOpenCodePrecompactSnapshotPlugin: OpenCode plugin precompact-snapshot
// syncOpenCodePrecompactSnapshotPlugin instala o plugin v2 para PreCompact
// cross-CLI (ADR-precompact-snapshot Decisao 3). Plugin apenas-v2:
// OpenCode v1 nao expoe "session.hook('compaction')". Em v1 e silencioso
// (sem instalacao), em v2 wirado normalmente.
func syncOpenCodePrecompactSnapshotPlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "precompact-snapshot", "precompact-snapshot.ts", false)
}

// syncOpenCodeTokenNudgePlugin: OpenCode plugin token-nudge
// syncOpenCodeTokenNudgePlugin instala o plugin v2 para token nudge
// (ADR-token-nudge-contract Decisao 4). Plugin apenas-v2: usa
// ctx.client.session.tokens() que so existe em v2. Fallback silent se
// SDK method nao existir nesta versao. Em v1 e silencioso.
func syncOpenCodeTokenNudgePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "token-nudge", "token-nudge.ts", false)
}

// syncOpenCodeCtxWindowNudgePlugin: OpenCode plugin ctx-window-nudge
// syncOpenCodeCtxWindowNudgePlugin instala o plugin v2 para ctx-window
// nudge (A-17). Plugin apenas-v2: usa ctx.session.hook("context") para
// injecao one-shot no system prompt + ctx.storage para contador de tool
// calls. Em v1 e silencioso (rede de seguranca fica no bash hook wirado
// nos outros 4 CLIs).
func syncOpenCodeCtxWindowNudgePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "ctx-window-nudge", "ctx-window-nudge.ts", false)
}

// syncOpenCodeCtxWindowSummarizeAtStopPlugin: OpenCode plugin ctx-window-summarize-at-stop
// syncOpenCodeCtxWindowSummarizeAtStopPlugin instala o plugin v2 para
// ctx-window summarize no Stop (A-17). Plugin apenas-v2: mesmo padrao do
// nudge (ctx.session.hook para injecao + storage para estado). Em v1
// silencioso. Nao faz spawn do binario ctx-window (rede de seguranca
// fica no bash hook wirado nos outros 4 CLIs).
func syncOpenCodeCtxWindowSummarizeAtStopPlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "ctx-window-summarize-at-stop", "ctx-window-summarize-at-stop.ts", false)
}

// syncOpenCodePlugin instala o plugin de lembrete do context-guard
// para o OpenCode (v2-only, injeção via ctx.session.hook("context")).
func syncOpenCodePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "context-guard-nudge", "context-guard-nudge.ts", true)
}

// syncOpenCodeMemoryNudgePlugin instala o plugin de lembrete de
// memory-mcp (store_memory) para o OpenCode (v2-only).
func syncOpenCodeMemoryNudgePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "memory-nudge", "memory-nudge.ts", true)
}

// syncOpenCodeAgentReactNudgePlugin instala o plugin de lembrete
// de validacao de hipoteses (agent-react) para o OpenCode (v2-only).
func syncOpenCodeAgentReactNudgePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "agent-react-nudge", "agent-react-nudge.ts", true)
}

// syncOpenCodeDocsCachePlugin instala o plugin que cacheia
// passivamente docs consultadas via webfetch/context7 no OpenCode (v2-only).
func syncOpenCodeDocsCachePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "docs-cache", "docs-cache.ts", true)
}

// syncOpenCodeRepoMapWarmupPlugin instala o warm-up lazy do repo-map (v2-only).
func syncOpenCodeRepoMapWarmupPlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "repo-map-warmup", "repo-map-warmup.ts", false)
}

// syncOpenCodeMemoryPipelinePlugin: OpenCode plugin memory-pipeline
// Instala o plugin v2 de observação e consolidação contínua de memória (A-39).
func syncOpenCodeMemoryPipelinePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "memory-pipeline", "memory-pipeline.ts", false)
}

// syncOpenCodeMemoryPruneSessionStartPlugin: OpenCode plugin memory-prune (A-69)
// Wirar auto-prune SessionStart via proxy tool.execute.after com flag armed
// (padrao repo-map-warmup v2). Plugin v2-only
// (hooks/memory-prune-session-start.opencode.v2.ts).
func syncOpenCodeMemoryPruneSessionStartPlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "memory-prune-session-start", "memory-prune-session-start.ts", false)
}
