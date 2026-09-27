# ADR — Linguagem de hooks decidida pelo host (não pelo projeto)

- **Status**: Aceito
- **Data**: 2026-09-27
- **Decisor**: agente + usuário (ses_atual)
- **Fonte**: `docs/ADR-cline-plugin-ts-first.md` (companion, Proposta), `docs/PADRAO-HOOKS-CLIS.md`, `docs/ADR-trilha-c-cobertura-cross-cli.md`
- **Tags**: hooks, typescript, shell, cross-cli, policy, A-88

## Contexto (verificável)

1. Hoje o agente-sync wirar hooks em **6 hosts** com **3 linguagens distintas** (TS puro, JS, shell). A escolha de linguagem hoje é **herdada**, não decidida — cada hook foi escrito quando o host alvo apareceu:
   - **Cline**: `cline-plugin/index.js` (JS, nasceu antes da doc `/sdk/plugins` ficar disponível)
   - **OpenCode v2**: `hooks/*.v2.ts` (TS, paralelo ao SDK v2)
   - **OpenCode v1 (legado)**: `hooks/*.opencode.ts` (TS, v1 SDK)
   - **Claude Code / Cursor / Antigravity / Codex**: `hooks/*.sh` (shell, único mecanismo aceito)
2. `docs/PADRAO-HOOKS-CLIS.md:21-28` — matriz 5×N já reconhece que cada host tem **superfície de hooks diferente**. O padrão atual mistura linguagens sem regra explícita de quando cada uma é usada.
3. `docs/ADR-cline-plugin-ts-first.md` (Proposta, companion deste) — propõe trocar Cline JS por TS. Este ADR generaliza a regra para todos os hosts.
4. Premissa anti-overengineering (`AGENTS.md §3`): "prefira editar existente a criar novo". Hooks novos que duplicam linguagem por gosto pessoal violam isso.

## Decisão

### Política: host decide, projeto obedece

A linguagem de um hook é função **do host onde ele roda**, não da preferência do time. Cada hook tem **1 fonte de verdade** na linguagem nativa do host:

| Host | Linguagem canônica | Mecanismo | Por quê |
|---|---|---|---|
| **Cline** | TypeScript | `cline plugin install` → `dist/index.js` | SDK nativo do Cline é TS via AgentPlugin |
| **OpenCode v2** | TypeScript | Plugins wirados em `~/.config/opencode/plugins/` | SDK nativo do OpenCode v2 é TS |
| **Claude Code** | Shell | `~/.claude/settings.json` → `command` | Único mecanismo aceito |
| **Cursor** | Shell | `~/.cursor/hooks.json` → shell | Único mecanismo aceito |
| **Antigravity** | Shell | `~/.gemini/config/hooks.json` → shell | Único mecanismo aceito |
| **Codex** | Shell | `~/.codex/hooks.json` → shell | Único mecanismo aceito |

### Regras operacionais

1. **Hook novo nasce em TS** quando o host alvo é Cline ou OpenCode v2.
2. **Hook novo nasce em shell** quando o host alvo é Claude Code, Cursor, Antigravity ou Codex.
3. **Adapter shell ≤ 30 LOC**: só faz `jq` no payload, chama `agent-sync hook <cli> --event=... --base-dir=...`, mapeia saída para o contrato do host. Zero lógica de negócio.
4. **Adapter TS ≤ 200 LOC**: mapeia contexto do plugin para o contrato interno do bridge Go e devolve `{appendContext}` ou `{skip, reason}`. Sem lógica de negócio.
5. **Lógica de negócio sempre no motor Go**: binário `agent-sync` (e subcomandos `ctx-window`, `audit`, `repo-map`) é dono da execução real do hook. Adapter é thin mapper.
6. **Nunca ter duas fontes de verdade para o mesmo hook em hosts diferentes** (ex.: mesmo hook em `.v2.ts` E em `.sh`). Cada hook é dono de um único host.

### Coexistência com hooks legados

- Hooks shell wirados hoje em Claude Code / Cursor / Antigravity / Codex **permanecem** (modo legacy aceito).
- Hooks `.v2.ts` wirados hoje em OpenCode **permanecem** (já estão no padrão correto).
- `cline-plugin/index.js` **migra para TS** no escopo do ADR companion (`ADR-cline-plugin-ts-first.md`).

### Exceções documentadas

- Hooks que **só fazem nudge de contexto** (ex.: `memory-nudge`, `context-guard-nudge`) podem ter adapter shell quase trivial. Se o host alvo não suportar o SDK, shell é a única opção — não há "outra forma" para evitar shell.
- Bridge Go em `internal/hooks/cline_bridge.go` aceita tanto nomes de arquivo (`PreToolUse`) quanto nomes internos do runtime (`tool_call`) — polyfill não é necessário.
- Tipos compartilhados entre Cline TS e OpenCode TS ficam em ADR filha A-89 (não decidido agora).

## Consequências

### Positivas

- **Decisão consistente**: mistura de linguagens deixa de ser "herança histórica" e vira "política explícita". Onboarding de novo contribuidor fica trivial — escolhe a linguagem olhando o host.
- **Anti-overengineering reforçado**: proíbe criar hook em linguagem X quando o host só aceita Y (e vice-versa).
- **Manutenibilidade**: 1 fonte de verdade por hook. Adapter shell nunca tem lógica; adapter TS nunca tem lógica. Mudança de contrato é local.
- **Cobertura cross-CLI explícita**: cada hook declara seu host-alvo no nome do arquivo (`.opencode.ts`, `.v2.ts`, `.sh`, `.ts`) — não ambíguo.

### Negativas / limitações

- **Revisão de hooks legados**: alguns arquivos `.sh` viraram "modo legacy aceito" sem plano de remoção. Mantê-los é aceito pela regra 5×N, mas a dívida fica visível.
- **Barreira de entrada TS**: contribuidor que conhece só shell precisa aprender TS para mexer no plugin Cline. Mitigação: o adapter é fino, e o motor Go continua shell-friendly via `agent-sync hook <cli>`.
- **Toolchain TS**: `tsc` + `@types/node` viram pré-requisito para tocar em hooks Cline/OpenCode. Já é pré-requisito para OpenCode v2 (ADR Aceito), então não é overhead novo.

## Critérios de promoção (Proposto → Aceito)

| # | Critério | Como medir |
|---|---|---|
| 1 | Política tabelada acima aparece literalmente em `AGENTS.md` (ou link para este ADR) | `grep "Host define"` retorna a seção |
| 2 | ADR companion `ADR-cline-plugin-ts-first.md` promovido para `docs/` | `ls docs/ADR-cline-*.md` mostra o Aceito |
| 3 | Hooks novos nas últimas 2 sprints seguem a regra (sem exceção silenciosa) | Code review + grep por extensão |
| 4 | `docs/PADRAO-HOOKS-CLIS.md` referencia este ADR na seção "Filosofia de Design" | Link presente |
| 5 | Sem regressão cross-CLI: smoke dos 5 hosts verdes após migração Cline | Suite de smokes existente |

## Reversibilidade

- Reverter este ADR = voltar a "linguagem herdada". Nenhum código muda imediatamente; o ADR vira "Proposta" novamente, e cada hook novo volta a ser decidido por gosto.
- Hooks já migrados (Cline JS→TS) **não revertem** — eles seguem o ADR companion.

## Não escopo

- Reescrever Go em TS (rejeitado — motor sustenta telemetria, ctx-window, audit).
- Forçar SDK TS em hosts que não aceitam (Claude Code, Cursor, Antigravity, Codex).
- Unificar sintaxe entre shell e TS via abstração (rejeitado — divergência é por host).
- Compartilhar tipos entre Cline e OpenCode (ADR filha A-89, opcional).

## Refs

- `docs/ADR-cline-plugin-ts-first.md` (Proposta, companion) — migração concreta Cline JS→TS
- `docs/ADR-trilha-c-cobertura-cross-cli.md` (Aceito) — regra 5×N que embasa "host decide"
- `docs/PADRAO-HOOKS-CLIS.md` — matriz 5×N de equivalência
- `docs/ADR-opencode-v2-ts-runtime.md` (Aceito) — paralelo TS-first OpenCode
- `hooks/*.v2.ts` (13 arquivos) — plugins OpenCode já em TS
- `cline-plugin/index.js:1-160` — origem da migração Cline