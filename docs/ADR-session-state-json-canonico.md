# ADR: `session-state.json` canônico como fonte de verdade; `STATE.md` vira view

**Status**: Aceito
**Data**: 2026-09-19
**Decisor**: Matheus Dutra
**Tags**: state, handoff, schema, multi-cli, typesafe-principles

## Contexto

Hoje o estado de trabalho entre sessões é mantido em `STATE.md` no working tree do projeto (`/home/matheusdutra/Projects/agent-sync/STATE.md`). Inspeção do repo mostra:

- **`STATE.md` não é gerado por código.** `grep -r "generate.*STATE\|writeState\|renderState" --include="*.go"` retorna zero matches em todo o repo. Único artefato relacionado é `templates/STATE.md` (template estático, populado à mão).
- **Manutenção é 100% do modelo (LLM) durante a sessão.** O conteúdo é escrito no fim de cada turno pelo agente, sem schema, sem validação.
- **Compactação destrói semântica.** Quando o contexto estoura, o LLM reescreve o STATE a partir do que sobrou — pode omitir, resumir errado, ou duplicar.
- **5 CLIs leem o STATE de forma diferente.** Hook `SessionStart` em Claude/Codex/Cursor/Antigravity só injeta contexto ad-hoc (não tem injeção de STATE estruturado); OpenCode tem `experimental.session.compacting` mas sem leitura de STATE.md próprio. Cada CLI reinterpreta o markdown à sua maneira.
- **Não há git-friendly-by-design.** Drift entre sessões quando o usuário esquece de commitar; merge conflicts triviais em `## Próximos passos`.

O typesafe.ai manifesto prega que **decisões devem virar dados que outros componentes consomem sem reinterpretar**. STATE.md hoje é exatamente o oposto: prosa que cada CLI/agent reinterpreta.

A correção da sessão anterior (path errado citado nas minhas respostas, ADRs OK no repo) confirmou que o problema não é o conteúdo do STATE, é a **ausência de fonte estruturada**.

## Decisão

Estabelecer **`session-state.json` como fonte canônica de verdade**, com `STATE.md` como **view derivada e somente-leitura**. Três decisões de shape:

### Decisão 1 — JSON canônico, MD é render

Estrutura do arquivo:

```
<project>/.agent-sync/session-state.json   # fonte de verdade (schema fechado, validado)
<project>/STATE.md                          # view renderizada, regenerada a cada write
```

Direção da geração:

```
LLM/CLI escreve → session-state.json → agent-sync state render → STATE.md (view)
```

Quem nunca edita `STATE.md` à mão:

- `STATE.md` passa a ter header `# AUTO-GENERATED — edite .agent-sync/session-state.json e rode 'agent-sync state render'`
- Lint warning se `STATE.md` for editado à mão (git pre-commit hook comparando hash do MD vs hash do MD renderizado do JSON; mismatch = erro)
- Editor pode ignorar o aviso, mas o `git commit` falha com mensagem clara

### Decisão 2 — Schema fechado com campos mínimos

Schema do `session-state.json` (versão 1.0):

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://agent-sync.local/schemas/session-state.json",
  "title": "SessionState",
  "version": "1.0",
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "project", "git", "session", "decisions", "next_actions", "blockers"],
  "properties": {
    "schema_version": { "const": "1.0" },
    "project": {
      "type": "object",
      "additionalProperties": false,
      "required": ["name", "root"],
      "properties": {
        "name": { "type": "string", "minLength": 1 },
        "root": { "type": "string", "minLength": 1 }
      }
    },
    "git": {
      "type": "object",
      "additionalProperties": false,
      "required": ["branch", "head", "working_tree_summary"],
      "properties": {
        "branch": { "type": "string" },
        "head": { "type": "string", "pattern": "^[0-9a-f]{7,40}$" },
        "working_tree_summary": { "type": "string" }
      }
    },
    "session": {
      "type": "object",
      "additionalProperties": false,
      "required": ["id", "started_at", "updated_at"],
      "properties": {
        "id": { "type": "string", "minLength": 1 },
        "started_at": { "type": "string", "format": "date-time" },
        "updated_at": { "type": "string", "format": "date-time" }
      }
    },
    "decisions": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["id", "title", "rationale", "made_at"],
        "properties": {
          "id": { "type": "string", "pattern": "^D-[0-9]+$" },
          "title": { "type": "string", "minLength": 1 },
          "rationale": { "type": "string", "minLength": 1 },
          "made_at": { "type": "string", "format": "date-time" },
          "links": { "type": "array", "items": { "type": "string" } }
        }
      }
    },
    "next_actions": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["id", "title", "status"],
        "properties": {
          "id": { "type": "string", "pattern": "^A-[0-9]+$" },
          "title": { "type": "string", "minLength": 1 },
          "status": { "enum": ["pending", "in_progress", "blocked", "done"] },
          "blocker_ref": { "type": "string", "pattern": "^B-[0-9]+$" },
          "depends_on": { "type": "array", "items": { "type": "string" } }
        }
      }
    },
    "blockers": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["id", "title", "blocking_action_ids"],
        "properties": {
          "id": { "type": "string", "pattern": "^B-[0-9]+$" },
          "title": { "type": "string", "minLength": 1 },
          "blocking_action_ids": { "type": "array", "items": { "type": "string", "pattern": "^A-[0-9]+$" } }
        }
      }
    },
    "open_questions": {
      "type": "array",
      "items": { "type": "string", "minLength": 1 }
    }
  }
}
```

Notas:

- **`additionalProperties: false`** no root e em cada sub-objeto. Qualquer campo extra é recursado na escrita. (Consistente com princípio da ADR-schema-output-versionado.)
- **IDs estáveis**: `D-N` (decisão), `A-N` (action), `B-N` (blocker) — pattern enforçado, sem UUID. Cross-references tipadas (`blocker_ref`, `depends_on`, `blocking_action_ids`).
- **`session.id` é estável** dentro da sessão de trabalho; `started_at` vs `updated_at` permitem reconstruir timeline.
- **Open questions**: lista de strings para pendências que não viraram blocker formal.

### Decisão 3 — CLI subcommands + hooks populam, humanos consultam

Novos subcommands no `agent-sync`:

| Subcommand | O que faz |
|---|---|
| `agent-sync state read` | Imprime `session-state.json` (texto ou `--json`) |
| `agent-sync state write` | Valida + escreve `session-state.json` (atomic write via temp+rename) |
| `agent-sync state render` | Lê JSON, escreve `STATE.md` (view). Idempotente. |
| `agent-sync state validate` | Valida JSON contra schema sem escrever nada |
| `agent-sync state next-action` | Retorna apenas o `next_actions[].status == "pending"` mais antigo (machine-readable, ideal para `SessionStart`) |

Ganchos de populate:

- **Hook `SessionStart` (5 CLIs)** → chama `agent-sync state next-action` e injeta o resultado como `additionalContext`. Substitui o atual context-shot ad-hoc por linha única determinística.
- **Skill `context-guard` → ação "atualizar STATE.md"** → vira "chamar `agent-sync state write` com payload construído a partir do que a sessão produziu".
- **Nudge `memory-nudge` (threshold 25)** → além de sugerir `store_memory`, sugere `agent-sync state write` se houve decisão/ação nova na sessão.

Atomicidade — opção **lockless** com fallback opcional:

- **Default (lockless)**: `state write` usa temp file + `rename(2)`. Para evitar colisão de tmpfile entre processos concorrentes, o tmp inclui PID + nanoTimestamp (`/session-state.json.tmp.<pid>.<nanoTimestamp>`). Cada writer tenta 3 retries com 10ms de backoff se o rename falhar por ENOTEMPTY/EEXIST transitório. Não há lockfile central.
- **Por que lockless primeiro**: na prática, escritores concorrentes no mesmo projeto são raros. Cada CLI roda em seu próprio processo de agente; SessionStart de Claude e memory-nudge de Codex dificilmente disparam no mesmo milissegundo. Manual `agent-sync state write` + hook simultâneo é a única race realisticamente possível — e mesmo assim é serializável via atomic rename (último vence).
- **Fallback opcional (lockfile)**: se observarmos em produção corrupção repetida (rename falhando, JSON truncado, etc.), adicionamos lockfile `flock(2)` com timeout 5s. Não vira caminho padrão — fica atrás de feature flag `AGENT_SYNC_SESSION_LOCK=1` e log explícito quando ativo.
- **Pre-commit hook** compara hash do `STATE.md` contra hash do MD regenerado a partir do JSON. Mismatch = falha (impede edição manual divergente).

### Decisão 4 — Migração: STATE.md atual vira snapshot inicial

O `STATE.md` desta sessão (e de qualquer projeto que já use o template) vira **seed** para o primeiro `session-state.json`:

- Script de migração `agent-sync state migrate-from-md <path>` lê STATE.md e extrai:
  - branch/head do working tree (via `git -C <root>`)
  - decisions/next_actions/blockers por heurística (regex em cabeçalhos markdown: `## Decisões`, `## Próximos passos`, `## Bloqueios`)
- Heurística é best-effort: campos que não conseguir extrair ficam vazios + warning.
- Após migração, `STATE.md` é regenerado pelo `state render` e diff é mostrado ao usuário.

Não há lock-in: o JSON é editável à mão (com cuidado), e o MD é regenerável.

## Consequências

### Positivas

- **Fonte única de verdade.** Acaba divergência entre CLI A e CLI B interpretando STATE.md diferentemente.
- **Compactação não destrói estado.** `session-state.json` é arquivo; sobrevive a qualquer compactação de contexto do LLM.
- **Próxima ação executável como dado.** `agent-sync state next-action` retorna string determinística; `SessionStart` injeta sem LLM no caminho.
- **Tiposafe-style: estado vira primitiva.** Outras ferramentas (scripts de CI, dashboards, `mr-reviewer`) consomem o JSON sem parsear markdown.
- **Cross-references tipadas.** `blocker_ref: B-3` permite navegação estruturada vs "ir achando" no markdown.
- **Migração viável.** STATE.md vira view — nada se perde.

### Negativas / trade-offs

- **Trabalha manual inicial para projetos existentes.** Cada repo que já tem STATE.md precisa rodar `state migrate-from-md` uma vez. Aceitável: comando idempotente.
- **Heurística de migração é best-effort.** Pode perder nuances do markdown original (ênfase, links contextuais). Mitigação: warning explícito por campo não-extraído; usuário revisa.
- **Dois arquivos para commit.** `STATE.md` + `.agent-sync/session-state.json`. Mitigação: mesmo `git add` cobre os dois; pre-commit hook valida que MD é render válido do JSON.
- **Adiciona 3 subcommands ao CLI.** Escopo de teste cresce. Mitigação: PoC cobre read/write/render apenas; subcommands são wiring fino sobre funções já testadas.
- **Lockfile pode dar deadlock** se hook travar. **Decisão revisada nesta revisão**: lockfile **não é caminho padrão**. Vai atrás de feature flag `AGENT_SYNC_SESSION_LOCK=1` e só é ativado se observarmos corrupção repetida em produção. Default é lockless com tmpfile único por writer.

## Decisões revisadas

- **Lockfile default OFF** (revisão pós-feedback): detalhamento acima. Lockless com tmpfile único por writer é o default; lockfile só se ativa se observarmos corrupção em produção.

## Evidência / Implementação

Esta ADR é Proposta — sem código ainda além do PoC mínimo (entregue junto). Quando aceita, o escopo de implementação é:

| Arquivo | Mudança |
|---|---|
| `schemas/session-state.json` | Schema do JSON canônico (1.0) |
| `tools/internal/jsonschema/` | (compartilhado com ADR-schema-output-versionado) |
| `cmd/agent-sync/session_state.go` | Struct `SessionState` + `Read()` / `Write()` / `Render()` / `Validate()` |
| `cmd/agent-sync/session_state_test.go` | Round-trip + render estável + schema válido |
| `cmd/agent-sync/main.go` | Wire de `state read/write/render/validate/next-action/migrate-from-md` |
| `hooks/session-state.antigravity.sh` | `SessionStart` Antigravity → injeta `state next-action` |
| `hooks/session-state.cursor.sh` | Idem Cursor |
| `hooks/session-state.opencode.ts` | Idem OpenCode (`experimental.session.compacting` + `session.start`) |
| `hooks/session-state.sh` | Idem Claude Code + Codex |
| `skills/context-guard/SKILL.md` | "Atualizar STATE.md" → "chamar `state write`" |
| `templates/STATE.md` | Header com aviso `# AUTO-GENERATED` |

### Critério de aceite (dois estágios)

**Estágio A — "Implementação validada"** (sai de Proposto assim que cumprido):

- PoC passa (round-trip JSON → render MD → diff estável). ✅ já cumprido.
- `go test ./cmd/agent-sync/...` verde com novos testes. ✅ já cumprido.
- `agent-sync state read/write/render/validate/next-action/migrate-from-md` wirados em `main.go`.
- Schema `schemas/session-state.json` publicado e integrado em `tools/internal/jsonschema/`.
- Smoke real nas **5 CLIs, nesta ordem: Claude → OpenCode → Antigravity → Codex → Cursor**. Cada uma com seu `SessionStart` (ou equivalente) invocando `agent-sync state next-action` e observando o resultado no log do hook. Razão da ordem: Claude é o caso base (JSON `additionalContext`); OpenCode valida o caso TypeScript/Cordis-style; Antigravity valida `PreInvocation` (nome/contrato diferente); Codex valida PreToolUse + PostToolUse; Cursor fecha com `additional_context` (snake_case). A ordem permite detectar regressão de shape cedo — falha em uma CLI reverte antes de investir nas próximas.

**Estágio B — "Aceito"** (final, o que esta ADR precisa para fechar):

- Cumprir Estágio A (5/5 smoke verde).
- **Cross-CLI handoff observado**: um CLI A escreve `session-state.json`, CLI B (instalado no mesmo projeto) lê via `next-action` no turno seguinte. Confirma que o JSON é portável entre runtimes.
- Migração do STATE.md atual desta sessão executada + diff do MD regenerado revisado pelo usuário.
- `STATE.md` desta sessão com header `# AUTO-GENERATED` no commit que fechar a ADR.

**Por que esta ordem e não outra**: Claude é o canônico (`additionalContext` em camelCase JSON); OpenCode é o mais divergente em schema (plugin TS + `experimental.session.compacting`); Antigravity reusa `PreInvocation` mas com shell semantics próprias; Codex entra depois porque o adapter `codex-protect-mcp-adapter.sh` já existe e pode servir de modelo; Cursor fecha por ter `additional_context` em snake_case (variante rara). Falha em qualquer ponto reverte a fila inteira até a prancheta.

## Limites conhecidos

1. **Heurística de migração do MD é imperfeita.** Decisões com rationale longo podem ser truncadas; usuário revisa.
2. **Não substitui `memory-mcp`.** `session-state.json` é estado **do projeto atual**; `memory-mcp` é memória **entre projetos**. Sobrepõem-se em "decisão" mas com escopos diferentes. Migração entre eles é ADR futura.
3. **Lockfile (quando ativo) em FS local.** Em projetos com múltiplos working trees no mesmo FS, podem colidir. Mitigação: lockfile em `<project_root>/.agent-sync/session-state.lock` é único por projeto, não global. **Default OFF** (lockless com tmpfile único por PID+nanoTimestamp).
4. **Nudge `state write` pode viciar o LLM a escrever a cada turno.** Mitigação: heurística — só sugere se houve mudança detectável (diff contra último JSON).
5. **Não cobre decisões não-tomadas** (i.e., `open_questions`). Lista de strings é placeholder; ADR futura pode promover a objeto com `id`/`raised_at`/`owner`.

## Próximos passos

1. **Aceitar a ADR.**
2. **PoC atual** (entregue nesta sessão): `session_state.go` com struct + 3 funções (`Read`/`Write`/`Render`) + teste de round-trip. Sem wire em `main.go`, sem hook, sem migração.
3. **Wire-up dos subcommands** (`main.go`) + flag `--project` apontando para root.
4. **Schema em `schemas/session-state.json`** + integração com `tools/internal/jsonschema/` (da ADR-schema-output-versionado).
5. **Migração do STATE.md atual** desta sessão + diff manual.
6. **Hooks `SessionStart`** nos 5 CLIs (sequencial, smoke por CLI).
7. **Atualizar `skills/context-guard`** para apontar para `state write` em vez de "editar STATE.md".
8. **Mover ADR para Aceito** após smoke real.

## Referências

- **Typesafe AI manifesto**: <https://typesafe.ai/manifesto> — princípio "make intelligence composable": o estado tem que ser primitiva, não prosa que cada componente reinterpreta.
- **Templates**: `templates/STATE.md` (template estático atual; vai virar view renderizada).
- **ADR relacionada**: `docs/ADR-schema-output-versionado.md` — mesma filosofia aplicada a outputs de binários; este ADR estende para estado de sessão.
- **Skill relacionada**: `skills/context-guard/SKILL.md` — vai passar a invocar `state write` em vez de editar MD à mão.
- **Sessão 2026-09-19**: STATE.md foi podado de 704→126 linhas neste exato trabalho. Hoje o podar é decisão do LLM a cada turno; com `state render`, a apresentação é determinística.
- **Limitação conhecida**: OpenCode `experimental.session.compacting` (citado em `docs/ADR-fim-de-turno-hooks.md`) tem injeção real de contexto. As outras 4 CLIs (Claude/Codex/Cursor/Antigravity) só têm `additionalContext` limitado — o ganho real desta ADR é maior para essas 4.