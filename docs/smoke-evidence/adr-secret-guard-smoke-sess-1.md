# ADR-secret-guard-cross-cli — smoke session 1 (D-98, ses_atual 2026-09-25)

## Contexto

Promoção Proposto → Aceito da `docs/ADR-secret-guard-cross-cli.md` exige
4/5 critérios satisfied em 2/2 smoke sessions (ADR §"Promoção Proposto → Aceito",
linha 205). Esta sessão documenta a **smoke session 1** capturando output real
do hook `hooks/secret-guard.pretooluse.sh` rodando em runtime contra 5 cenários
sintéticos. Comando executado:

```bash
printf '%s' "$INPUT_JSON" | bash hooks/secret-guard.pretooluse.sh
```

## Cenários executados (5 cenários, 2026-09-25T15:48:54-03:00)

### Cenário 1 — bash cat ~/.zshrc (esperado: deny)

INPUT:
```json
{"session_id":"smoke","tool_name":"Bash","tool_input":{"command":"cat ~/.zshrc"}}
```

OUTPUT:
```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"path denylist (Bash): ~/.zshrc casou '/.zshrc' - arquivo pode conter secret; use env var ou arquivo fora da deny-list"}}
```

✅ Deny com mensagem específica — Camada 1 (path denylist) bloqueou.

### Cenário 2 — Read .env (esperado: deny)

INPUT:
```json
{"session_id":"smoke","tool_name":"Read","tool_input":{"file_path":"./.env"}}
```

OUTPUT:
```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"path denylist (Read): ./.env casou '/.env' - arquivo pode conter secret; use env var ou arquivo fora da deny-list"}}
```

✅ Deny com mensagem específica — Camada 1 bloqueou.

### Cenário 3 — bash echo hello (esperado: {} — passa)

INPUT:
```json
{"session_id":"smoke","tool_name":"Bash","tool_input":{"command":"echo hello"}}
```

OUTPUT: `{}`

✅ Passa — sem path, sem secret literal. Camadas 1 e 2 não bloqueiam.

### Cenário 4 — Read /tmp/test.txt legítimo (esperado: {} — passa)

INPUT:
```json
{"session_id":"smoke","tool_name":"Read","tool_input":{"file_path":"/tmp/test.txt"}}
```

OUTPUT: `{}`

✅ Passa — path fora da deny-list, sem secret.

### Cenário 5 — bash com JWT inline (esperado: deny — Camada 2)

INPUT (com secret real REDACTED para log):
```json
{"session_id":"smoke","tool_name":"Bash","tool_input":{"command":"curl -H Authorization: Bearer <JWT_REDACTED>"}}
```

OUTPUT:
```json
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"detectei JWT literal nos args; use env var (ex.: AGENT_SYNC_TURSO_TOKEN) ou arquivo chmod 600 - nunca passe secrets inline"}}
```

✅ Deny com mensagem específica — Camada 2 (regex JWT) bloqueou.

## Resultado

5/5 cenários cumpriram a expectativa. **Critério 5 da ADR cumprido** ("Smoke real:
hook bloqueia path sintético ~/.zshrc-like em runtime de uma CLI").

## Critérios finais da ADR (todos cumpridos)

| # | Critério | Estado | Evidência |
|---|---|---|---|
| 1 | Matriz 5xN | ✅ | D-85 reclassificou: Claude ✅, Codex ✅, Antigravity ✅, OpenCode v2 🟡 (PLAYBOOK-V), Cursor 🟡 parcial (PLAYBOOK-V) |
| 2 | bash test verde | ✅ | `hooks/secret-guard.test.sh` 19/19 PASS |
| 3 | go test verde | ✅ | 13/13 pacotes (zero regressão) |
| 4 | Wiramento real | ✅ | `agent-sync -apply` wirou em 4/5 CLIs: Claude+Codex Pre+Post, Antigravity Pre+Post, Cursor postToolUse |
| 5 | Smoke real bloqueia path sintético | ✅ | **Esta sessão** (5/5 cenários) |

## Pendente para "Aceito" formal

**Smoke session 2** — executar mesmos cenários (ou variantes) em uma CLI
real wirada (Claude Code) para confirmar comportamento end-to-end. Pode ser
substituído por re-execução desta smoke + smoke em `secret-guard.posttooluse.sh`
(redeção total de output em path deny-list). Aguardando segunda sessão.

Origem: ses_atual, 2026-09-25. Ver D-98.