// index.ts — entry point do Cline Plugin (A-90).
//
// A partir do A-90 este arquivo é apenas um re-export do plugin TS autocontido
// em `src/plugin.ts`. O adapter proxy antigo (`callBridge` via execFileSync +
// index.ts:160-LOC) foi eliminado — sua lógica migrou para `src/hooks/*` e
// `src/bridge.ts`.
//
// Status da migração (12 passos do plano):
//   ✅ Passo 4: 3 hooks PoC wirados (principles-inject puro, memory-nudge
//      via bridge, ctx-window-summarize via bridge)
//   ⏳ Passo 5: este re-export
//   ⏳ Passo 6: 12 hooks restantes (PostToolUse, TaskStart, TaskComplete, smoke)
//   ⏳ Passo 9: delete `internal/hooks/cline_bridge.go` + `apply_cline.go`
//   ⏳ Passo 10: delete `hooks/*.sh` (15 arquivos)
//
// Durante a migração, hooks ainda não portados ficam temporariamente off —
// documentado no ADR-090 §"Critérios de aceite".

import plugin = require("./src/plugin.js");
export = plugin;