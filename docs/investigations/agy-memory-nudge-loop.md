# Investigação — Loop de leitura do agy (Antigravity CLI)

- **Status**: Investigação empírica + **H1 refutada parcialmente / H2 (poluição de contexto) priorizada**. **NÃO aplicar fix até smoke controlado em agy**.
- **Data**: 2026-09-25
- **Investigador**: agente + usuário + outro agent (revisou empiricamente)
- **Refs**: D-87 (A-66 fix high-signal), hooks `memory-nudge.sh` vs `memory-nudge.antigravity.sh`, agy wiramento em `~/.gemini/config/hooks.json`

## 0. Correções factuais (revisão 2026-09-25, segundo agent)

Esta seção lista erros da versão original do doc, identificados via grep/wc em 2026-09-25:

| Afirmação original | Valor correto | Como verificar |
|---|---|---|
| `wc -l tools/cmd/memory-mcp/main.go` = 1146 | **1216 linhas** (+70) | `wc -l tools/cmd/memory-mcp/main.go` |
| `~/.cache/agent-sync/memory.db` = 357 entries | **392 entries** (380 scratch + 12 permanent) | `memory-mcp stats -json` |
| `injectSteps` em `hooks/memory-nudge.antigravity.sh:22` | **linha 21** (off-by-one) | `grep -n injectSteps hooks/memory-nudge.antigravity.sh` |
| Comentário sobre bug em `hooks/memory-nudge.sh:47` | **linha 36-37** (comentário; :47 é o `printf '{}'` final) | `grep -n "Schema correto" hooks/memory-nudge.sh` |
| 4 hooks wirados em agy (lista parcial) | **18 hooks wirados** (todos `agent-sync-*`) | `cat ~/.gemini/config/hooks.json` |

Refutação parcial de H1 (causa raiz proposta):
- `hooks/bash-guardian.antigravity.sh:20-21` documenta explicitamente: "**PreInvocation: só aceita injectSteps**. `decision` = unknown field."
- Isso contradiz a hipótese original (doc seção 3) de que injectSteps é bloqueante em PreInvocation. O schema válido do PreInvocation é justamente injectSteps + ephemeralMessage.
- A evidência original de runtime 2026-09-22 ("injectSteps faz Gemini interpretar como tool call denied") não foi reproduzida em 2026-09-25.

## 1. Contexto observado

Usuário reportou padrão anômalo durante sessão paralela do agy (Antigravity CLI) lendo `tools/cmd/memory-mcp/main.go`:

```
Read(~/Projects/agent-sync/tools/cmd/memory-mcp/main.go)   26 lines
Read(~/Projects/agent-sync/tools/cmd/memory-mcp/main.go)  101 lines
Read(~/Projects/agent-sync/tools/cmd/memory-mcp/main.go)   66 lines
Read(~/Projects/agent-sync/tools/cmd/memory-mcp/main.go)   56 lines
Read(~/Projects/agent-sync/tools/cmd/memory-mcp/main.go)   36 lines
Read(~/Projects/agent-sync/tools/cmd/memory-mcp/main.go)   26 lines  ← ciclo reinicia
Read(~/Projects/agent-sync/tools/cmd/memory-mcp/main.go)  101 lines
... (repete)
```

**Padrão cíclico de 5 chunks (26+101+66+56+36 = 285 linhas)** que se repete indefinidamente.

## 2. Evidência empírica coletada

### 2.1 Tamanho do arquivo vs leitura

| Item | Valor |
|---|---|
| `wc -l tools/cmd/memory-mcp/main.go` | **1216 linhas** *(corrigido: 1146 era estimativa)* |
| Soma dos chunks cíclicos (26+101+66+56+36) | 285 linhas |
| **% do arquivo lido** | **~23%** |

Agente nunca chega à linha 286+.

### 2.2 Hooks wirados em agy (confirmado, lista completa)

`/home/matheus_dutra/.gemini/config/hooks.json` carrega **18 hooks** `agent-sync-*`:

**PreInvocation (3)** — todos emitem `injectSteps+ephemeralMessage`:
- `agent-sync-memory-nudge` ← emite prompt sobre "aconteceu algo que deveria virar memória"
- `agent-sync-agent-react-nudge` ← emite prompt sobre "hipótese ativa sem validação"
- `agent-sync-context-guard-nudge` ← emite prompt sobre "carregue context-guard"

**PostToolUse (1)** — grava buffer-record a cada tool call:
- `agent-sync-memory-observe` (com `AGENT_SYNC_AGENT_KIND=antigravity`)

**Stop (3)**, **PreToolUse (1, bash-guardian)**, **SessionStart (1, memory-consolidate)**, outros hooks utilitários.

**Achado crítico**: 3 scripts emitem prompts simultâneos em **cada invocação** do modelo — candidato principal a poluição de contexto.

### 2.3 Diferença entre scripts memory-nudge

**`hooks/memory-nudge.sh` (Claude Code, comentário linhas 36-37, printf '{}' na linha 47)**:
```bash
# Schema correto do Antigravity PreInvocation: emite {} (vazio).
# Verificado em runtime 2026-09-22: injectSteps + ephemeralMessage
# faz o Gemini CLI interpretar como `tool call denied by pre-tool hook`.
printf '{}'
```

**`hooks/memory-nudge.antigravity.sh` (Antigravity, injectSteps na linha 21)**:
```bash
printf '{"injectSteps":[{"ephemeralMessage":"[agent-sync] Aconteceu algo..."}]}'
```

**Atenção**: o comentário em `memory-nudge.sh` foi escrito em 2026-09-22 mas o em `<PRIVATE_AGENT_CUSTOM>/docs/hooks.md` (citado em `bash-guardian.antigravity.sh:20-21`) diz que **injectSteps é schema válido para PreInvocation**. O comentário original pode estar desatualizado — ver §3 (refutação parcial).

### 2.4 Memória do DB

- `~/.cache/agent-sync/memory.db` tem **392 entries** (380 scratch + 12 permanent) — `memory-mcp stats -json`
- Sem evidência de entry stale específica "Read memory-mcp main.go" (`search_memory` retornou vazio)
- DB cheio mas não é causa direta do loop

## 3. Hipóteses (revisadas após investigação 2026-09-25)

### H1 (original do doc): injectSteps bloqueia tool call ❌ REFUTADA PARCIALMENTE

**Afirmação**: `memory-nudge.antigravity.sh` injeta `injectSteps+ephemeralMessage` que Gemini CLI interpreta como "tool call denied by pre-tool hook".

**Evidência refutadora**:
- `hooks/bash-guardian.antigravity.sh:20-21` documenta explicitamente: "**PreInvocation: só aceita injectSteps**. `decision` = unknown field."
- Logs reais do agy 2026-09-25 (`~/.gemini/antigravity-cli/log/cli-20260925_162102.log`, 943 linhas + `cli.log` 944 linhas): busca por `denied|injectSteps|ephemeralMessage|memory-nudge` retornou **0 hits relevantes**.
- Smoke isolado (passo 4.3 do doc): `echo '{"invocationNum":25}' | bash hooks/memory-nudge.antigravity.sh` → stdout contém `{"injectSteps":[{"ephemeralMessage":"..."}]}` (script emite, mas emit ≠ bloqueia).
- `agent-preinvocation.antigravity.sh` (já corrigido, emite `{}`) também é wirado em PreInvocation — coexistência de 2 formatos no mesmo evento é evidência de que ambos são aceitos.

**Conclusão H1**: injectSteps **é schema válido para PreInvocation** (não bloqueante). A interpretação original do comentário em `memory-nudge.sh` (2026-09-22) parece desatualizada ou específica de uma versão antiga do agy. Loop não é causado por `denied by pre-tool hook`.

### H2 (alternativa priorizada): poluição de contexto efêmero ⚠️ NÃO VALIDADA

**Hipótese**: os **3 scripts wirados em PreInvocation** emitem `injectSteps+ephemeralMessage` simultaneamente em cada invocação múltipla de 25. Modelo recebe 3 ephemeralMessage em sequência competindo pela atenção → confunde paginação do arquivo → loop de leitura.

**3 scripts culpados** (todos verificados via grep, linha 21 cada):
| Script | Texto do ephemeralMessage |
|---|---|
| `hooks/memory-nudge.antigravity.sh:21` | "[agent-sync] Aconteceu algo nesta sessão que deveria virar memória..." |
| `hooks/agent-react-nudge.antigravity.sh:21` | "[agent-sync] Hipótese ativa sem validação? Faça 1 grep antes de assumir..." |
| `hooks/context-guard-nudge.antigravity.sh:21` | "[agent-sync] Carregue context-guard para consolidar info antes de continuar..." |

**Cadência confirmada** em `~/.cache/agent-sync/memory.db`:
- 20+ eventos `memory-nudge #N` com `N ∈ {25, 50, 75, ..., 450}`, `agent=antigravity`
- Timestamps crescentes em 2026-09-25 19:35 → 20:46
- Threshold = 25 (env `AGENT_SYNC_MEMORY_NUDGE_THRESHOLD`)

**Loop resultante (H2)**:
```
[turn N, N múltiplo de 25]
  ↓
[3 PreInvocation simultâneos] cada um injeta ephemeralMessage
  ↓
[Modelo recebe 3 prompts efêmeros] competem pela atenção
  ↓
[Paginação do arquivo reseta] modelo lê chunks cíclicos (26+101+66+56+36 = 285 linhas)
  ↓
[Ciclo continua até modelo ignorar um dos prompts]
```

**Por que H2 é mais consistente com a empiria**:
- Explica por que loop **não** aparece em Claude Code (apenas 1 script injectSteps: agent-react-nudge.pretooluse.sh)
- Explica cadência de 25 (3 scripts disparam simultaneamente — threshold comum)
- Compatível com `bash-guardian.antigravity.sh` emitir `{}` (não compete) e modelo seguir funcionando

**Status H2**: formulada, **NÃO validada empiricamente** (não rodei sessão controlada do agy).

### Coerência da hipótese:

| Observação | Explicação |
|---|---|
| 5 chunks cíclicos | Antes do próximo múltiplo de 25, modelo tenta ler o máximo possível |
| Reset para offset 0 | Cada bloqueio reseta estado de paginação do modelo |
| Não chega a 286+ | Bloqueios impedem leitura sustentada |
| Tamanho 1146 é irrelevante | Loop se estabelece antes de o agente terminar de ler |

## 4. Como validar a hipótese (passos concretos)

### 4.1 Verificar log do Gemini CLI ✅ FEITO (2026-09-25)

```bash
ls -la ~/.gemini/antigravity/logs/  # ou logs/
grep -i "denied by pre-tool hook\|injectSteps\|memory-nudge" ~/.gemini/antigravity/logs/*.log | head -20
```

**Resultado real**: busca por `denied|injectSteps|ephemeralMessage|memory-nudge|context-guard|agent-react` em `cli-20260925_162102.log` (943 linhas) + `cli.log` (944 linhas) retornou **0 hits relevantes** (única menção: `command_assessor` é hook built-in do agy, não nossos).

**Interpretação**: H1 (injectSteps bloqueia) **não tem evidência em runtime** de 2026-09-25. Logs não rastreiam stdout dos nossos hooks, então ausência não confirma nem refuta definitivamente — mas é consistente com `bash-guardian.antigravity.sh:20-21` dizendo que injectSteps é schema válido.

### 4.2 Verificar contagem de invocations ✅ FEITO (2026-09-25)

```bash
~/.local/bin/memory-mcp recent -last 50 -kind guard_nudge -json | grep antigravity
```

**Resultado real**: 20+ eventos `memory-nudge #N` com `N ∈ {25, 50, 75, ..., 450}`, `agent=antigravity`, timestamps em 2026-09-25 19:35 → 20:46. Threshold = 25 (env `AGENT_SYNC_MEMORY_NUDGE_THRESHOLD`).

### 4.3 Teste isolado (smoke mínimo) ✅ FEITO (2026-09-25)

```bash
echo '{"invocationNum":25}' | bash hooks/memory-nudge.antigravity.sh
```

**Resultado real**:
- stdout = `{"injectSteps":[{"ephemeralMessage":"[agent-sync] Aconteceu algo..."}]}` (exit 0)
- Confirma que script **emite** injectSteps, mas isso **não implica bloqueio**.

### 4.4 Smoke controlado em agy real ⚠️ NÃO FEITO

**Pré-requisito**: agy em modo verboso, capturar stdout dos 3 hooks (memory-nudge, agent-react-nudge, context-guard-nudge) num único turno, correlacionar com resets de paginação do modelo.

**Risco de aplicar fix sem este passo**: pode ser a causa errada — loop persiste e investigação fica inconclusa.

**Quem pode executar**: usuário em sessão agy controlada (não rodei nesta sessão porque agy aqui é subprocesso secundário, wirado mas com namespace MCP não exposto conforme D-75).

## 5. Fix proposto (NÃO APLICADO — aguarda §4.4)

### Mudança proposta (apenas 1 dos 3 scripts, ref H1)

**Arquivo**: `hooks/memory-nudge.antigravity.sh` (linha 21)

**Antes**:
```bash
printf '{"injectSteps":[{"ephemeralMessage":"[agent-sync] Aconteceu algo nesta sessao que deveria virar memoria (correcao do usuario, decisao de projeto, preferencia confirmada)? Se sim, grave um resumo com o porque via store_memory (memory-mcp)."}]}'
```

**Depois**:
```bash
printf '{}'
```

### Análise de risco (fix isolado só deste script)

| Aspecto | Avaliação |
|---|---|
| Esforço | 1 linha |
| Risco de regressão | **Médio** — comentário interno (2026-09-22) diz que injectSteps bloqueia, mas bash-guardian (mais recente) diz que é schema válido. Comentários conflitantes = risco. |
| Impacto positivo se H1 correta | Loop para neste script específico |
| Impacto positivo se H2 correta | **NENHUM** — outros 2 scripts (agent-react-nudge, context-guard-nudge) continuam emitindo |
| Reversibilidade | Trivial |
| Compat com H2 | **Inadequado** — fix é só de 1/3 |

### Fix alternativo (H2 — poluição de contexto) ⚠️ RECOMENDADO se §4.4 confirmar H2

Mudar **os 3 scripts** para emitir `{}`:

| Script | Linha atual | Mudança |
|---|---|---|
| `hooks/memory-nudge.antigravity.sh` | 21 | `printf '{}'` |
| `hooks/agent-react-nudge.antigravity.sh` | 21 | `printf '{}'` |
| `hooks/context-guard-nudge.antigravity.sh` | 21 | `printf '{}'` |

Mover a diretriz de cada ephemeralMessage para `GEMINI.md` (system prompt estático) — segue o padrão já aplicado em `agent-preinvocation.antigravity.sh:13-20` (que emite `{}` em vez de injectSteps).

**Risco do fix alternativo**: baixo — comentário em `memory-nudge.sh:36-37` já documenta que `{}` é schema válido para PreInvocation (pelo menos no Claude Code; pode aplicar para agy também).

## 6. Critérios de aceitação do fix (revisados)

| # | Critério | Como verificar | Status |
|---|---|---|---|
| 1 | Smoke §4.3 confirmado: script emite `injectSteps` (H1) ou `{}` (H2) | `echo '{"invocationNum":25}' \| bash` | ✅ feito |
| 2 | Cadência 25 confirmada no DB | `memory-mcp stats` + grep `guard_nudge` | ✅ feito |
| 3 | Logs do agy 2026-09-25 sem "denied by pre-tool hook" | `grep` em cli-*.log | ✅ feito (H1 refutada) |
| 4 | `bash-guardian.antigravity.sh:20-21` documenta injectSteps válido | grep no script | ✅ feito |
| 5 | Smoke controlado em agy real correlaciona resets de paginação com cadência | sessão verbosa | ⚠️ NÃO feito |
| 6 | Loop do agy para após aplicar fix (qualquer um) | observação direta | ⚠️ pendente |

**Critérios 1-4 ✅, critério 5 ⚠️ bloqueante, critério 6 só após 5**.

## 7. Ações pendentes (revisadas — ordembloqueante)

1. **[Bloqueante]** Smoke controlado em agy real (§4.4) com captura de stdout dos 3 hooks — necessário para confirmar H2 antes de qualquer fix.
2. **[Bloqueante]** Decidir entre fix H1 (1 script) ou fix H2 (3 scripts) com base no resultado do smoke.
3. **[Aplicar]** Fix granular após smoke:
   - Se H1: 1 commit `fix(agy-loop)` mudando só `memory-nudge.antigravity.sh:21`
   - Se H2: 3 commits separados (um por script) + 1 commit em `GEMINI.md` para mover diretriz
4. **[Validar]** Re-wirar agy (já wirado), observar 1 sessão subsequente, confirmar loop parou
5. **[Documentar]** Criar D-N no STATE.md documentando causa + fix + refs cruzadas (este doc vira insumo)

## 8. Refs cruzadas (atualizado)

- **D-87** (STATE.md): A-66 fix high-signal — patch de cadência `count%THRESHOLD==0` em `ctx-window-nudge.sh`. Mesmo padrão aplicado em agy (3 scripts wirados).
- **`hooks/memory-nudge.sh:36-37`**: comentário sobre `{}` ser schema válido para Antigravity PreInvocation (escrito 2026-09-22).
- **`hooks/bash-guardian.antigravity.sh:20-21`**: comentário sobre `injectSteps` ser schema válido para PreInvocation no agy atual (mais recente, contradiz parcialmente memory-nudge.sh).
- **`hooks/memory-nudge.antigravity.sh:21`**: emite injectSteps (loop candidato).
- **`hooks/agent-react-nudge.antigravity.sh:21`**: emite injectSteps (loop candidato — hipótese H2).
- **`hooks/context-guard-nudge.antigravity.sh:21`**: emite injectSteps (loop candidato — hipótese H2).
- **`hooks/agent-preinvocation.antigravity.sh:13-20`**: emite `{}` (referência de padrão correto já aplicado).
- **Investigation D-75** (STATE.md): daemon memory-mcp wirado mas não exposto em sub-agent — padrão similar (CLI wirado ≠ funcional).
- **Investigação futura (D-N)**: se H2 confirmar, investigar se `agent-react-nudge.cursor.sh` e `context-guard-nudge.cursor.sh` têm mesmo padrão.

## 9. NOTA: status no board

Esta investigação **não é A-N** (não é entrega). É **investigação + proposta de fix** aguardando validação empírica (§4.4).

Quando fix for aplicado e validado:
- Criar D-N no STATE.md documentando bug + fix + refs
- Marcar fix como entrada do board se houver padrão recorrente

Quando fix for **rejeitado** (H1/H2 refutadas em smoke controlado):
- Documentar contra-evidência nesta mesma investigação (nova §10)
- Prosseguir para hipótese secundária (paginação do modelo, memory-observe loop)