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

// syncOpenCodePluginVersioned: OpenCode plugin versioned helper
// syncOpenCodePluginVersioned copia o plugin OpenCode no formato da versão
// instalada: v1 usa hooks/<base>.opencode.ts; v2 tenta hooks/<base>.v2.ts
// (padrão legado) com fallback para hooks/<base>.opencode.v2.ts (padrão
// novo a partir de precompact-snapshot/token-nudge/ctx-window-*).
// required=false (plugins novos, só-v2): em v1 retorna nil silencioso.
// required=true (plugins legados): em v1 exige o .opencode.ts.
func syncOpenCodePluginVersioned(baseDir string, target TargetCLI, base, dst string, required bool) error {
	if target.OpenCodePluginDir == "" {
		return nil
	}
	major := opencode.MajorVersion()
	hooksDir := filepath.Join(baseDir, "hooks")
	if major < 2 {
		src := filepath.Join(hooksDir, base+opencodePluginSuffix(major))
		if _, err := os.Stat(src); err != nil {
			if !required {
				// Plugin novo, só existe em v2 — nada a instalar em v1.
				return nil
			}
			return fmt.Errorf("plugin do hook não encontrado: %s", src)
		}
		return copyFile(src, filepath.Join(target.OpenCodePluginDir, dst))
	}
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

// syncOpenCodePlugin: OpenCode plugin context-guard-nudge
// syncOpenCodePlugin instala o plugin best-effort de lembrete do context-guard
// para o OpenCode (não é config declarativa: precisa de um plugin TS real).
// Em v1 mantém o .opencode.ts (best-effort, issue #13574); em v2 usa o .v2.ts
// com injeção via ctx.session.hook("context").
func syncOpenCodePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "context-guard-nudge", "context-guard-nudge.ts", true)
}

// syncOpenCodeMemoryNudgePlugin: OpenCode plugin memory-nudge
// syncOpenCodeMemoryNudgePlugin instala o plugin best-effort de lembrete de
// memory-mcp (store_memory) para o OpenCode (versão conforme linha instalada).
func syncOpenCodeMemoryNudgePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "memory-nudge", "memory-nudge.ts", true)
}

// syncOpenCodeAgentReactNudgePlugin: OpenCode plugin agent-react-nudge
// syncOpenCodeAgentReactNudgePlugin instala o plugin best-effort de lembrete
// de validacao de hipoteses (agent-react) para o OpenCode (versão conforme
// linha instalada).
func syncOpenCodeAgentReactNudgePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "agent-react-nudge", "agent-react-nudge.ts", true)
}

// syncOpenCodeDocsCachePlugin: OpenCode plugin docs-cache
// syncOpenCodeDocsCachePlugin instala o plugin best-effort que cacheia
// passivamente docs consultadas via webfetch/context7 no OpenCode
// (versão conforme linha instalada).
func syncOpenCodeDocsCachePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "docs-cache", "docs-cache.ts", true)
}

// syncOpenCodeRepoMapWarmupPlugin: OpenCode plugin repo-map-warmup
// syncOpenCodeRepoMapWarmupPlugin instala o warm-up lazy do repo-map.
// Plugin novo: só existe em v2 (hooks/repo-map-warmup.v2.ts). Em v1 é
// silencioso (sem fallback .opencode.ts) conforme regra "novo plugin só v2".
func syncOpenCodeRepoMapWarmupPlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "repo-map-warmup", "repo-map-warmup.ts", false)
}

// syncOpenCodeMemoryPipelinePlugin: OpenCode plugin memory-pipeline
// Instala o plugin v2 de observação e consolidação contínua de memória (A-39).
func syncOpenCodeMemoryPipelinePlugin(baseDir string, target TargetCLI) error {
	return syncOpenCodePluginVersioned(baseDir, target, "memory-pipeline", "memory-pipeline.ts", false)
}

