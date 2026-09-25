# ADR — Principles-inject PreToolUse (verdade absoluta + anti-overengineering)

- **Status**: Aceito (promovido por D-64 em 2026-09-23; wirado em Claude + Codex via `syncPrinciplesInjectHook` + 2 testes verdes; escopo 2/5 CLIs declarado)
- **Data**: 2026-09-21
- **Decisor**: agente + usuário
- **Refs**: rules/global-rules.md seções 2 e 3; ADR Trilha C (matriz 5xN).

## Contexto

`agent-sync` carrega `rules/global-rules.md` em 5 CLIs. Em sessões longas
ou com drift de contexto, o agente pode esquecer premissas críticas
(verdade absoluta + anti-overengineering), mesmo que elas estejam no
`AGENTS.md`/`CLAUDE.md`/`~/.config/opencode/AGENTS.md` etc.Queremos
mecanismo adicional que injete essas premissas **just-in-time no turno
do usuário**, sem custo de tokens em tool calls repetidos.

## Decisão

### 1. Hook one-shot via PreToolUse

Wirar `hooks/principles-inject.pretooluse.sh` em `PreToolUse` para
Claude Code + Codex. Plugin TS paralelo pode wirar OpenCode v2 (item
separado). Antigravity e Cursor ficam como matriz 5xN pendente.

### 2. Mecanismo: dedup por sessionID

Variável `STATE_DIR="${TMPDIR:-/tmp}/agent-sync-principles-injected"`.
Por sessão:
- 1ª PreToolUse: emite `{"hookSpecificOutput":{"additionalContext":"..."}}` e cria `<sid>.flag`.
- Chamadas subsequentes: emite `{}` (zero custo).

### 3. Texto do hook

Derivado verbatim de `rules/global-rules.md` seções 2 e 3, em uma
frase por premissa. Texto curto (~200 chars) para minimizar custo de
tokens quando o agente injeta.

### 4. Matriz 5xN

| CLI | Status | Como wirar |
| --- | --- | --- |
| Claude Code | ✅ Wirado | `syncPrinciplesInjectHook` → PreToolUse |
| Codex | ✅ Wirado | `syncPrinciplesInjectHook` → PreToolUse |
| OpenCode v2 | 🟡 Pendente | Plugin TS paralelo (`hooks/principles-inject.opencode.v2.ts`) |
| Antigravity | 🟡 Pendente | PreInvocation ou como proxy |
| Cursor | 🟡 Pendente | Sem block nativo; ver se `PostToolUse` inject serve |

## Consequências

**Positivas:**
- Premissas críticas injetadas no turno do usuário, não em tool call repetido.
- Custo de tokens: ~200 chars injetados UMA vez por sessão (resto é `{}`).
- Audit trail: cada wirar aparece no `~/.claude/settings.json` / `~/.codex/hooks.json`.

**Negativas:**
- Risco de fadiga se o usuário tiver 50 sessões/dia (cada uma vê o injection).
- Wirar em 5 CLIs precisa de 5 code paths diferentes. Escopo desta entrega: 2 (Claude + Codex).

## Critério de promoção Proposto → Aceito

- [ ] Smoke T-T: 3 cenários (sessão nova injeta; sessão repetida dedup; OpenCode Plugin TS wirado).
- [ ] Verificação de que `~/.claude/settings.json` e `~/.codex/hooks.json` contêm o entry.
- [ ] Diff de `additionalContext` matching verbatim com `rules/global-rules.md`.

## Pendências separadas

- OpenCode v2: plugin TS paralelo (A-22 ou A-23 conforme matriz).
- Antigravity + Cursor: wirar PreInvocation ou PostToolUse com `additionalContext`.
- Smoke runtime cross-CLI (mesma fixture, as 2 CLIs wiradas).
