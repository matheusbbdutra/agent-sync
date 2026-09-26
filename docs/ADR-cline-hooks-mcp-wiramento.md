# ADR — Wiramento completo de hooks + MCPs em Cline (A-80)

- **Status**: Proposto
- **Data**: 2026-09-25
- **Decisor**: agente + user (ses_atual)
- **Contexto**: Após A-79 wirar `bash-rm-guardian` em Cline (6ª CLI), verificou-se que **wiramento está parcial**:
  - 18 hooks wirados por default em outras CLIs **não estão wirados em Cline** (warnings no apply)
  - 4 MCP servers wirados em OpenCode (`context7`, `docs`, `memory`, `code-graph`) **não estão wirados em Cline**
- **Tags**: cline, hooks, mcp, wiramento, A-80

## Contexto

### Estado atual pós-A-79

`~/.cline/hooks/PreToolUse` (script bash-rm-guardian wirado) + `~/.cline/hooks/bash-rm-guardian.sh` (core).
`agent-sync apply -target=cline` wirar:
- ✅ rules (AGENTS.md)
- ✅ skills (symlinks)
- ❌ agents (tipo desconhecido)
- ❌ outros 18 hooks (warnings: "HooksSettingsPath is a directory")
- ❌ 4 MCP servers (`upsert_cline()` não existe em `scripts/setup-mcp.sh`)

### Causa dos warnings

`internal/hooks/apply_table.go` registra 18 hooks wirando por default (sem `agentKinds` restrito):
```go
{name: "context-guard", fn: syncHooks, detail: settingsPathDetail},
{name: "memory-nudge", fn: syncMemoryNudgeHook, detail: settingsPathDetail},
{name: "agent-react", fn: syncAgentReactNudgeHook, detail: settingsPathDetail},
{name: "secret-guard-pretooluse", fn: syncSecretGuardPreToolUseHook, ...},
{name: "memory-observe", fn: syncMemoryObserveHook, detail: settingsPathDetail},
{name: "bash-guardian", fn: syncBashGuardianClaude, agentKinds: ["claude"]},
{name: "bash-guardian", fn: syncBashGuardianAntigravity, ...},
// etc
```

`syncHooks` e similares leem `target.HooksSettingsPath` como **arquivo JSON** (`os.ReadFile`) → falha quando é diretório (Cline).

### Causa dos MCPs não wirados

`scripts/setup-mcp.sh` tem:
```bash
upsert_claude()    # ~/.claude/settings.json (mcpServers)
upsert_codex()     # ~/.codex/hooks.json
upsert_antigravity()  # ~/.gemini/config/mcp_config.json
upsert_opencode()  # ~/.config/opencode/opencode.json
upsert_cursor()    # ~/.cursor/mcp_config.json
# FALTA: upsert_cline()
```

Cline provavelmente usa `~/.cline/data/settings/cline_mcp_settings.json` ou path similar (a confirmar empiricamente).

## Decisão proposta (A-80)

### A-80.1: Wirar 18 hooks restantes em Cline

**Estratégia**: Detectar se `HooksSettingsPath` é diretório (Cline) ou arquivo (outras CLIs). Se diretório, wirar cada hook como **script individual** em `~/.cline/hooks/<HookName>.sh` que adapta o contrato:
- Lê payload via stdin (formato Cline: `{tool, input, context}`)
- Emite output no formato Cline: `{cancel, context, error}`

**Mudanças**:
1. `internal/hooks/hooks_apply.go`: `syncStandardHookAtEvent` aceita `isDirectory` baseado em `HooksFormat == "cline"`
2. Cada hook atual (`hooks/<name>.sh`) ganha uma variante `<name>.cline.sh` que parseia payload Cline
3. Wiramento copia script correto baseado em format
4. Test smoke real com payload Cline para cada hook

**Trade-off**: 18 scripts novos (~50L cada = 900L). Alternativa: 1 dispatcher único que detecta format e adapta. Vou propor 1 dispatcher para reduzir volume.

**Esforço**: ~2-3h

### A-80.2: Wirar MCP servers em Cline

**Estratégia**: Adicionar `upsert_cline()` em `scripts/setup-mcp.sh`. Validar empiricamente o path exato do MCP config em Cline v3.

**Mudanças**:
1. Investigar `~/.cline/data/settings/cline_mcp_settings.json` (ou similar)
2. Adicionar `upsert_cline()` que adiciona 4 entries: `context7` (remote URL), `docs` (local docs-mcp), `memory` (local memory-mcp), `code-graph` (local repo-map --mcp)
3. Test idempotência

**Esforço**: ~1-2h

### A-80.3: Smoke real + ADR

**Estratégia**: Validar end-to-end com Cline CLI real:
1. Rodar `cline -p "delete tools/cmd/memory-mcp"` → validar que bash-rm-guardian dispara (já wirado em A-79)
2. Rodar `cline -p "liste arquivos .go do repo"` → validar que MCP `code-graph` responde
3. Atualizar este ADR para **Aceito**

**Esforço**: ~30min

## Consequências

**Positivas:**
- Cline vira **6ª CLI equivalente** em funcionalidade às outras 5
- Modelo LLM em Cline ganha memória observability, codebase awareness via MCP, agent-react nudge, etc.
- Wiramento parcial do A-79 fica completo

**Negativas:**
- ~4-6h de trabalho
- Cria acoplamento entre scripts hooks/ e formato Cline (precisa manter paridade)
- Se Cline SDK mudar formato payload, scripts `.cline.sh` quebram

## Refs

- A-73 [done]: foundation audit
- A-74 [done]: MCP + scripts
- A-75 [done]: wiramento OpenCode (modelo de referência)
- A-76 [done]: wiramento claude/codex/cursor
- A-77 [done]: wiramento OpenCode permission.bash
- A-78 [done]: investigação consumo + mitigação
- A-79 [done]: wiramento parcial Cline (só bash-rm-guardian)
- `internal/hooks/apply_table.go`: hooks wirados por default
- `scripts/setup-mcp.sh`: upsert_*() para outras CLIs
- `docs/investigations/opencode-token-consumption.md`: contexto da migração para Cline

## Próximo passo

Aprovar A-80 e executar em 3 sub-tasks sequenciais. Cada sub-task tem smoke test próprio antes de prosseguir.
