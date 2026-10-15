# ADR — Adapter único por evento com detecção de contrato do host (fold A/B)

- **Status**: Aceito (executado em `bd45573` + `7f90e87`; registro formal em 2026-10-15)
- **Data**: 2026-10-15
- **Decisor**: agente + usuário (sessão "menos linguagens, menos CLI")
- **Fonte**: `git show bd45573` (fold grupo A), `git show 7f90e87` (fold B1+B2),
  `hooks/memory-nudge.pretooluse.sh:1-38`, `hooks/bash-guardian.antigravity.sh:19-29`
- **Tags**: hooks, adapters, cross-cli, shell, refactor, anti-duplicação

## Contexto (verificável)

1. **O padrão anterior era N arquivos por (hook × host).** Cada nudge existia em 2–3 versões:
   `hooks/<nudge>.sh` (Claude/Codex), `hooks/<nudge>.antigravity.sh` (contrato próprio) e
   `hooks/<nudge>.v2.ts` (OpenCode). Trocar a mensagem ou o threshold = editar 3 arquivos.
2. **Custo medido no fold B** (`7f90e87`): 6 arquivos deletados — `agent-react-nudge.{,antigravity.}sh`
   (42L + 29L), `context-guard-nudge.{,antigravity.}sh` (40L + 29L), `memory-nudge.{,antigravity.}sh`
   (42L + 25L) — 14 arquivos, **+130 / −251**.
3. **Custo medido no fold A** (`bd45573`): 4 wrappers deletados — `bash-rm-guardian.{antigravity,pretooluse}.sh`
   (43L + 44L), `docs-cache.antigravity.sh` (23L), `repo-map-warmup.antigravity.sh` (15L) —
   10 arquivos, **+79 / −155**.
4. **O contrato do host é dedutível do payload**, sem `--cli=` hardcoded:
   - Claude/Codex `PreToolUse` → tem `session_id`, **não** tem `invocationNum`.
   - Antigravity `PreInvocation` → tem `invocationNum` (contador nativo) e, quando é PreToolUse,
     tem `toolCall`.
5. **Efeitos colaterais por host já conhecidos** (verificados em runtime 2026-09-22): em
   Antigravity `PreInvocation`, `additionalContext`/`injectSteps` fazem o Gemini CLI interpretar
   como **deny** — o nudge precisa sair por `stderr` + `{}` no stdout
   (`docs/investigations/agy-memory-nudge-loop.md`). Ou seja: o mesmo hook **deve** se comportar
   diferente por host — mas isso é uma decisão de runtime, não um arquivo por host.

## Decisão

### §1. 1 adapter por **evento**, não por (hook × host)

Os 3 nudges de `PreToolUse` viram 3 arquivos canônicos:
`hooks/memory-nudge.pretooluse.sh`, `hooks/agent-react-nudge.pretooluse.sh`,
`hooks/context-guard-nudge.pretooluse.sh` — os mesmos arquivos wirados em Claude, Codex e
Antigravity (o `sync*Hook` decide o evento por host).

### §2. Detecção de contrato no payload (padrão canônico)

```bash
invocation_num="$(printf '%s' "$input" | grep -o '"invocationNum"[[:space:]]*:[[:space:]]*[0-9]*' | head -n1 | grep -o '[0-9]*$')"
if [ -n "$invocation_num" ]; then
  count="$invocation_num"; agent="antigravity"   # contador nativo, sem estado proprio
else
  agent="claude-code"                            # contador em $TMPDIR/<session_id>.count
  counter_file="$STATE_DIR/${session_id:-default}.count"
  count=$(( $(cat "$counter_file" 2>/dev/null || echo 0) + 1 ))
  printf '%s' "$count" > "$counter_file"
fi
```

(abreviado de `hooks/memory-nudge.pretooluse.sh:17-38`)

Regras: (a) `set -euo pipefail` + `{ ... || true; }` em cada `grep` (sem o `|| true`, o `grep`
que não casa aborta o script e o host bloqueia a tool call); (b) **fail-open** — payload sem as
chaves esperadas cai no caminho Claude (`{}`/`additionalContext`), nunca em exit de erro.

### §3. O mesmo vale para os guards de core (`bash-guardian`, `docs-cache`)

- `hooks/bash-guardian.antigravity.sh:19-29`: a função `allow_pass()` decide pelo payload —
  se o input contém `"toolCall"` (PreToolUse) emite `{"decision":"allow"}`; senão emite `{}`
  (PreInvocation). O comentário do próprio arquivo documenta o contrato:
  *"PreToolUse: exige `decision` (allow/deny/ask). `{}` vazio = deny. PreInvocation: so aceita
  `injectSteps`. `decision` = unknown field."* — o mesmo script é wirado nos 2 eventos e resolve
  por payload.
- `hooks/docs-cache.sh:8-10`: o adapter chama o Go com `--from-hook --cli=auto`; a detecção de
  Antigravity (payload com `toolCall`) vive no **binário**, não no shell.

### §4. `.v2.ts` continua separado (não é fold)

OpenCode não entra no fold: o SDK nativo é TS/`@opencode/plugin`, então o adapter é TS por host
(`hooks/<nudge>.v2.ts`) — política de `ADR-linguagem-hooks-por-host` + `ADR-politica-linguagens` §3.

### §5. Resíduos que **não** foram dobrados (decisão explícita)

| Arquivo | Por que fica |
|---|---|
| `hooks/bash-guardian.antigravity.sh` | é o próprio core do guard (deny/ask por padrão de risco); já detecta contrato (§3) |
| `hooks/agent-stop.antigravity.sh` | lógica de Stop própria do host (contrato diferente do `Stop` de Claude/Codex) |
| `hooks/agent-preinvocation.antigravity.sh` | NOP wirado — decisão B3: mantido para preservar o wire existente |
| `hooks/*.v2.ts` (12 arquivos, 6 no padrão `*.opencode.v2.ts`) | SDK TS do Cline/OpenCode (§4) |

## Consequências

### Positivas

- **−197 LOC líquidas** nos 2 folds (`bd45573`: +79/−155; `7f90e87`: +130/−251), com o
  comportamento por host preservado.
- **1 arquivo por nudge**: trocar mensagem/threshold = 1 edição, não 3.
- **Host novo barato**: se o payload novo tiver as mesmas chaves, nenhum arquivo novo; se tiver
  contrato diferente, entra como branch explícito **no adapter**, documentado em comentário.
- **Contrato visível no código**: cada branch do adapter cita o host e o motivo
  (`memory-nudge.pretooluse.sh:3-8`), em vez de ficar implícito no nome do arquivo.

### Negativas / limitações

- **Sniffing de payload é heurística**: a detecção é "tem a chave X?". Se um host futuro emitir
  `invocationNum` com semântica diferente, ele cai no caminho Antigravity. Mitigação: chaves são
  específicas do host e há teste por payload (`internal/hooks/hooks_test.go:893` usa
  `{"invocationNum": 1}`).
- **Adapter único = ponto de regressão única**: um bug no `.pretooluse.sh` atinge 2–3 hosts ao
  mesmo tempo (antes, um arquivo por host isolava). Compensação: `bash-guardian-patterns.txt`,
  `nudge_target_test.go` e os smokes de wiramento cross-CLI.
- Shell do adapter cresceu em linhas absolutas: `memory-nudge.pretooluse.sh` tem 72L (antes:
  42L em `hooks/memory-nudge.sh` + 25L em `hooks/memory-nudge.antigravity.sh` = 67L em 2
  arquivos). O teto de 50L da `rules/global-rules.md §4` vale para "cola fina" simples; nudges
  com contador próprio + branch de contrato são a exceção declarada — o ganho é a **redução de
  arquivos por regra**, não de linhas.

## Reversibilidade

- `git revert 7f90e87` recria os 6 arquivos por host; `git revert bd45573` recria os 4 wrappers.
  Ambos revertem só arquivos de hook + entradas de sync — nenhum schema ou estado muda.
- Não há migração de estado: contadores vivem em `$TMPDIR/agent-sync-memory-nudge/` e são
  descartáveis por sessão.

## Não escopo

- Dobrar OpenCode (`.v2.ts`) no adapter shell — SDK do host é TS (§4).
- Unificar `Stop` (`agent-stop.antigravity.sh` fica separado: contrato próprio).
- Apagar `agent-preinvocation.antigravity.sh` (NOP) — decisão B3 preservada.
- Reescrever os adapters em Go (o binário não pode ser o handler do host; `ADR-linguagem-hooks-por-host` regra 5).

## Refs

- `git show bd45573`, `git show 7f90e87` — os 2 folds, com a lista de arquivos deletados
- `hooks/memory-nudge.pretooluse.sh:1-38`, `hooks/agent-react-nudge.pretooluse.sh:21`,
  `hooks/context-guard-nudge.pretooluse.sh:18` — detecção `invocationNum`
- `hooks/bash-guardian.antigravity.sh:19-29` — detecção por `toolCall` no core do guard
- `hooks/docs-cache.sh:8-10` — `--cli=auto` (detecção no binário Go)
- `docs/investigations/agy-memory-nudge-loop.md` — por que Antigravity exige `stderr` + `{}`
- `docs/ADR-linguagem-hooks-por-host.md`, `docs/ADR-politica-linguagens.md` — tetos e exceções
- `internal/hooks/hooks_test.go:893`, `internal/hooks/nudge_target_test.go` — testes por payload