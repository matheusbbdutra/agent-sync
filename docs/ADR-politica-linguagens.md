# ADR — Política de linguagens: Go é o motor, shell é cola, TS só na borda, Python é zero

- **Status**: Aceito (travada em `rules/global-rules.md §4` por `c5e2e09` e aplicada em
  `6b9a565` + `41f62b0` na mesma sessão; registro formal escrito em 2026-10-15)
- **Data**: 2026-10-15
- **Decisor**: agente + usuário (sessão "menos linguagens, menos CLI")
- **Fonte**: `rules/global-rules.md:23-28` (§4), `git show c5e2e09`, `git show 6b9a565`,
  `git show 41f62b0`, `docs/ADR-reuso-bash-sudo-agents.md` §1
- **Tags**: linguagens, arquitetura, go, shell, typescript, python, cross-cli, policy

## Contexto (verificável)

1. **O problema não era o Go, era a duplicação.** O usuário: *"Go é interessante, mas acho que
   fica mais facil utilizar JS TS ou PY no lugar?"* → investigado, o diagnóstico apontou o custo
   em outra direção: **três linguagens implementando a mesma regra** (shell + TS + JS), não a
   linguagem do motor.
2. **Drift real medido antes da política.** O `secret-guard` tinha a deny-list implementada
   **3 vezes**, cada uma com sua própria cópia da regra — `hooks/secret-guard.pretooluse.sh`
   (149L antes), `hooks/secret-guard.posttooluse.sh` (93L antes) e
   `cline-plugin/src/hooks/secret-guard.ts` (52L antes). O `docs-cache` extraía `{url,text}` em
   **Python** (`hooks/docs-cache.py` 49L + `hooks/docs-cache.antigravity.py` 69L).
3. **O repo externo analisado** (`docs/ADR-reuso-bash-sudo-agents.md` §1/§4) implementava o
   motor em **Python** (`governance-gate.py`) — reaproveitável apenas como **conceito**, nunca
   como código, sob esta política.
4. **Superfície atual**: adapters shell finos (`hooks/secret-guard.pretooluse.sh` 17L,
   `.posttooluse.sh` 16L, `docs-cache.sh` 25L, `bash-guardian.antigravity.sh` 50L), 12 arquivos
   `hooks/*.v2.ts` (borda Cline/OpenCode, 6 deles `*.opencode.v2.ts`) e `hooks/*.py` = **0**
   (verificado por `ls hooks/*.py`).
5. **Sessão de referência** (`645f665..d46ca06`, 11 commits): 4 são `refactor(...)` explicitamente
   motivados por esta política (Python-zero no docs-cache, fonte-única do secret-guard, folds A/B)
   + o commit que travou a regra (`c5e2e09`).

## Decisão

### §1. Go é a fonte da verdade de toda lógica

Auditoria, cache, budget, memória, deny-list, parsing de transcript: tudo vive em Go
(`internal/`, `tools/cmd/`). O que já está em Go **fica** em Go. Binários: `agent-sync`,
`secret-scan`, `docs-cache-write`, `ctx-window`, `repo-map`, `memory-mcp`, `shell-validate`.

### §2. Shell é cola fina (≤50 LOC), nunca lógica

O adapter shell só: (a) lê o payload do stdin, (b) chama o binário Go, (c) traduz a saída para o
contrato do host (`{}`/`hookSpecificOutput` para Claude/Codex, `decision` para Antigravity).
Exemplo canônico — `hooks/secret-guard.pretooluse.sh` (17L):

```bash
BIN="$SCRIPT_DIR/../bin/secret-scan"
[ -x "$BIN" ] || BIN="$(command -v secret-scan || true)"
if [ -z "$BIN" ]; then printf '{}\n'; exit 0; fi   # fail-open
"$BIN" check --format auto || printf '{}\n'
```

Fail-open obrigatório: binário ausente = `{}` + exit 0, nunca derrubar o turno do host.

### §3. TypeScript só na borda onde o SDK obriga (≤150 LOC)

Cline (`cline-plugin/src/hooks/`) e OpenCode v2 (`hooks/*.v2.ts`) são os únicos hosts com SDK TS
nativo — ali o adapter é TS, mas **sem lógica**: delega via bridge/`execFile` para o binário Go.
Nenhum `*.ts` reimplementa deny-list, deny-list de path, contador de nudge ou parsing de
transcript.

### §4. Python = zero (inclusive reaproveitamento externo)

Proibido criar `.py` novo. Código Python externo (ex.: `governance-gate.py`, extração do
docs-cache) é **portado para Go**, nunca copiado. Motivo: `.py` no caminho crítico do hook soma
um runtime a mais sem entregar o que o Go já entrega (binário único, <5ms, sem dependência de
interpretador no host).

### §5. 1 fonte de verdade por função

Proibido ter a mesma regra em `.sh` **e** `.ts` com lógica divergente. A fonte é o binário Go; os
adapters por host são descartáveis e substituíveis. Paridade entre adapter e binário é provada
por teste (ex.: `TestGuardParityWithLegacyShell` em `tools/cmd/secret-scan/guard_test.go:9` e o
runner de contrato `hooks/secret-guard.test.sh`).

## Enforcement na mesma sessão (evidência)

| Item | Antes | Depois | Commit |
|---|---|---|---|
| `secret-guard` PreToolUse | 154L de lógica shell (deny-list duplicada) | adapter 17L → `secret-scan check` | `41f62b0` |
| `secret-guard` PostToolUse | 97L de lógica shell | adapter 16L → `secret-scan redact` | `41f62b0` |
| Fonte da deny-list | shell + TS replicados | `tools/cmd/secret-scan/guard.go` (216L) + `main.go` (147L) | `41f62b0`, `3dd6836` |
| Adapter TS (`cline-plugin/src/hooks/secret-guard.ts`) | implementação própria | adapter com paridade testada | `41f62b0` |
| `docs-cache` extração | Python: `docs-cache.py` (49L) + `.antigravity.py` (69L) | `tools/cmd/docs-cache-write/hook_extract.go` (207L) + adapter `docs-cache.sh` (25L) com `--from-hook` | `6b9a565` |

`ls hooks/*.py` = **0 arquivos**; `ls hooks/*.v2.ts` = 12 arquivos (6 no padrão `*.opencode.v2.ts`).

## Consequências

### Positivas

- **Um lugar para consertar cada regra**: corrigir a deny-list é editar `guard.go` + rodar o
  teste de paridade, não sincronizar 3 arquivos em 3 linguagens à mão.
- **Menos runtime no caminho crítico do hook**: shell cola + binário Go (<5ms) substitui lógica
  shell extensa e Python (que exigiria interpretador no host).
- **Adaptadores descartáveis**: um host novo = 1 adapter fino; o motor não muda.
- **Reaproveitamento externo com critério**: conceito do `bash-sudo-agents`, decisão aqui, código
  em Go (`ADR-reuso-bash-sudo-agents` §1).

### Negativas / limitações

- Toolchain Go obrigatório para tocar em lógica (não dá para corrigir um hook só com `sed`).
- Shell continua existindo como linguagem de borda nos 4 hosts sem SDK TS — a política reduz
  lógica em shell, não elimina shell.
- Adapter fino pode **mascarar** falha de binário (fail-open): se o binário não for instalado
  (`make install`), o hook vira no-op silencioso. Mitigação: `agent-sync doctor` reporta binário
  ausente e os smokes de wiramento checam presença.

## Reversibilidade

- Reverter a política = reverter `c5e2e09` (§4 do `global-rules.md`) e, por item, os commits de
  aderência (`6b9a565`, `41f62b0`). Nada é estruturalmente irreversível: os adapters são
  arquivos isolados.
- Reintroduzir Python exigiria ADR nova — a proibição é deliberada e datada.

## Não escopo

- Reescrever o motor Go em TS/JS (rejeitado: `ctx-window`, `audit`, `budget`, `memory-mcp` e a
  telemetria sustentam o produto; `ADR-linguagem-hooks-por-host` define a borda, não o motor).
- Portar os `.v2.ts` do OpenCode para Go (o SDK do host é TS — §3 manda manter TS ali).
- Definir **qual** linguagem cada host usa (isso é `ADR-linguagem-hooks-por-host`); esta ADR
  define **onde vive a lógica**.
- Proibir shell em hosts que só aceitam shell (contradiria a ADR de host).

## Refs

- `rules/global-rules.md:23-28` — §4 (texto normativo desta ADR)
- `docs/ADR-linguagem-hooks-por-host.md` — linguagem de hook por host (borda)
- `docs/ADR-reuso-bash-sudo-agents.md` §1 — portar `overwrite`/cache para Go, não para `.py`
- `tools/cmd/secret-scan/` — fonte única da deny-list (paridade provada por teste)
- `tools/cmd/docs-cache-write/hook_extract.go` — extração `--from-hook` em Go
- `hooks/secret-guard.pretooluse.sh`, `hooks/docs-cache.sh` — referências de adapter fino
- `git log c5e2e09 6b9a565 41f62b0 3dd6836` — sessão que travou e aplicou a política