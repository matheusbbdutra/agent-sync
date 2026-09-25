# ADR: Schema versionado para outputs emitidos por binários do agent-sync

**Status**: Aceito
**Data**: 2026-09-19
**Decisor**: Matheus Dutra
**Tags**: schema, jsonl, observability, agents, contracts, typesafe-principles

## Contexto

O agent-sync hoje emite outputs em pelo menos 4 formatos distintos via stdout / arquivos locais, sem contrato versionado:

| Origem | Destino | Formato atual | Schema? |
|---|---|---|---|
| `false-success-guard` (`tools/cmd/false-success-guard/main.go:75`) | stdout do hook Stop | `{"flagged":bool,"reason":string}` | implícito |
| `codex-protect-mcp-adapter.sh` (`hooks/codex-protect-mcp-adapter.sh:21`) | stdout do hook PreToolUse Codex | `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow\|deny","permissionDecisionReason":...}}` | implícito |
| `observe-error.sh` (`hooks/observe-error.sh:8`) | `~/.cache/agent-sync/hooks/errors.jsonl` | linha JSON ad-hoc por hook | nenhum |
| `mr-review-local` (citado em `agents/mr-reviewer.md:24`) | stdout do tool | JSON com SHAs, merge base, summary, patch, truncation/redaction flags | implícito |

Problemas concretos causados pela ausência de schema versionado:

1. **Consumers têm que re-parsear a cada mudança de campo.** Cada vez que um binário adiciona um campo novo, hooks e agentes que consomem quebram silenciosamente.
2. **Ingestão no `memory-mcp` é frágil.** O `agent-sync -observability` lê `errors.jsonl` assumindo shape não documentado.
3. **Não há como recusar um output malformado na escrita.** O agente diz "Pr[X=...]" e o consumidor aceita qualquer coisa que tenha `json.Valid()`.
4. **Não há migração.** Quando o shape evolui, registros antigos ficam ininterpretáveis sem aviso.

O typesafe.ai manifesto (visão "smart if-statements") exige que toda saída de componente componível seja **dado estruturado validável**, não prosa. A correção da sessão anterior (receipts órfãos + path errado + origem não-rastreada) reforçou que o agent-sync precisa de contratos explícitos antes de qualquer expansão do que é emitido.

## Decisão

Adotar **schema versionado por output** com 3 decisões de shape:

### Decisão 1 — Cada output tem um arquivo `.schema.json` ao lado

Para cada binário / script que emite output estruturado, criar `*.schema.json` em `schemas/` na raiz do repo (novo diretório), com:

- `$schema` apontando para `https://json-schema.org/draft/2020-12/schema`
- `$id` único por output (slug kebab-case do binário + output name)
- `version` semântico (`MAJOR.MINOR`) — campo obrigatório no payload
- `properties` fechadas onde fizer sentido (ver Decisão 3)
- `required` explícito

Exemplo para `false-success-guard`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://agent-sync.local/schemas/false-success-guard.json",
  "title": "FalseSuccessGuardOutput",
  "version": "1.0",
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "flagged", "reason"],
  "properties": {
    "schema_version": { "const": "1.0" },
    "flagged": { "type": "boolean" },
    "reason": { "type": "string", "minLength": 1 }
  }
}
```

### Decisão 2 — Validator Go centralizado em `tools/internal/jsonschema/`

Novo pacote que:

- Carrega `schemas/*.schema.json` na inicialização via `embed.FS`
- Exporta `Validate(name string, payload []byte) error` retornando erro tipado (`*ValidationError` com campo e motivo)
- Falha fechada: payload sem `schema_version` ou com versão desconhecida é rejeitado (não silenciosamente aceito)
- Validação por **content check + schema check**: JSON válido + schema válido + version match

Uso em cada produtor:

```go
if err := jsonschema.Validate("false-success-guard", payload); err != nil {
    // log estruturado e recusa; nunca escrever payload inválido
}
```

### Decisão 3 — `additionalProperties: false` por padrão, whitelist explícita

Outputs são **fechados por contrato**. Quando um campo novo for necessário, MAJOR bump do schema + migração documentada. Razão: outputs do agent-sync são consumidos por código externo (hooks de 5 CLIs, agents em outros repos). Permitir campos extras = quebra silenciosa em produção.

Exceções explícitas (documentadas no schema como `additionalProperties: true`):

- `errors.jsonl` (libera campos arbitrários para extensibilidade de hooks)
- Qualquer output explicitamente marcado como `forward-compatible`

### Decisão 4 — Migração com co-existence window

Quando um schema evoluir (MINOR ou MAJOR bump):

- Binário continua emitindo **versão antiga** por padrão
- Flag `--schema-version=v2` (ou env `AGENT_SYNC_SCHEMA_VERSION`) opta pela nova
- Consumers migram por leitura: `if version == "v1" { ... } else if version == "v2" { ... }`
- Janela de co-existence: mínimo 30 dias entre bump MAJOR e remoção da v1
- Documentado em `docs/SCHEMA-CHANGELOG.md` (novo arquivo)

## Consequências

### Positivas

- **Contrato explícito antes de expansão.** Próxima mudança de output é bump versionado, não mutação silenciosa.
- **Consumers robustos.** Hooks e agents que consomem JSONL validam uma vez na entrada, não a cada chamada.
- **Falha fechada na escrita.** Produtor não consegue emitir payload quebrado sem aviso.
- **Tiposafe-style alinhado.** Outputs viram "smart if-statement" data — outros components consomem como dado, não reinterpretam.
- **Inventário unificado.** `schemas/` vira índice de tudo que o agent-sync emite hoje (e do que cada CLI/agent pode consumir).

### Negativas / trade-offs

- **Trabalho inicial alto.** Mapear e escrever schema para cada output existente (~4 hoje). Aceitável: trabalho é de mapeamento, não de decisão arquitetural repetida.
- **Ritmo mais lento para adicionar campos.** Hoje é só adicionar campo no struct Go; amanhã é bump versionado + schema. Aceitável: troca velocidade de prototipagem por segurança em produção.
- **Dependência externa mínima:** precisa de lib de JSON Schema em Go (proposta: `github.com/santhosh-tekuri/jsonschema/v5`, MIT, sem CGO). Validação lazy na inicialização para não dobrar tempo de startup.
- **MAJOR bump quebra consumers.** Se hook externo dependia de campo que saiu, vai falhar. Mitigação: janela de 30 dias + changelog público.

## Decisões revisadas

(nenhuma — ADR em estado Proposto na primeira iteração.)

## Evidência / Implementação

Esta ADR é Proposta — sem código ainda. Quando aceita, o escopo de implementação é:

| Arquivo | Mudança |
|---|---|
| `schemas/false-success-guard.json` | Schema do output atual (1.0) |
| `schemas/codex-protect-mcp-adapter.json` | Schema do output do adapter (1.0) |
| `schemas/observe-error.json` | Schema da linha de `errors.jsonl` (1.0, `additionalProperties: true`) |
| `schemas/mr-review-local.json` | Schema do output do tool (1.0) |
| `tools/internal/jsonschema/` | Pacote novo: `embed.FS` dos schemas + `Validate()` |
| `tools/cmd/false-success-guard/main.go` | Validar payload antes de imprimir |
| `hooks/observe-error.sh` | `jq` para validar contra schema antes de escrever (best-effort: warn + segue) |
| `hooks/codex-protect-mcp-adapter.sh` | Idem — `jq` + check `schema_version` |
| `docs/SCHEMA-CHANGELOG.md` | Log de versões por output |

### Critério de aceite (para mudar de Proposto → Aceito)

- Schemas escritos para os 4 outputs atuais, versionados 1.0.
- `tools/internal/jsonschema/` com testes unitários: payload válido passa, inválido falha com `*ValidationError`.
- Cada produtor chama `jsonschema.Validate()` antes de emitir; teste confirma que payload inválido é recusado.
- `docs/SCHEMA-CHANGELOG.md` criado com entrada inicial por output.
- `go test ./...` verde no módulo `tools`.

## Limites conhecidos

1. **JSON Schema draft 2020-12 cobre os casos atuais.** Se um output precisar de lógica condicional mais complexa (ex.: discriminação por enum), revisitar lib.
2. **Validação no shell é best-effort.** `jq` pode validar shape básico mas não schema completo. Hooks `.sh` warn-and-continue; quem valida de verdade é o consumer.
3. **Não cobre outputs de agentes (markdown).** Agents emitem prosa por design; schema é para outputs **estruturados** de binários/scripts. Agents ganham output estruturado via skill/tool, não via schema direto.
4. **`receipts/receipts.jsonl` e `review-receipts/receipts.jsonl` (registrados como órfãos) NÃO fazem parte deste schema** — origem não identificada (sessão anterior), provavelmente signer externo fora do repo. Gap separado.

## Próximos passos

1. **Aceitar a ADR** (revisão do usuário).
2. **Escrever schemas para os 4 outputs** atuais em uma única sessão (mapeamento + draft).
3. **Implementar `tools/internal/jsonschema/`** com lib `santhosh-tekuri/jsonschema/v5`.
4. **Wire-up nos 4 produtores** com testes.
5. **Mover ADR para Aceito** após smoke real (validar 1 round-trip completo em dev).
6. **Iterar para outputs de agents** (próxima fase, ADR separada): definir se agents devem emitir JSON estruturado no fim do turno (ex.: `mr-reviewer` retorna findings como JSON, não prosa).

## Referências

- **Typesafe AI manifesto**: <https://typesafe.ai/manifesto> (lido em 2026-09-19) — princípio "smart if-statements": outputs viram dado estruturado consumível, não prosa reinterpretada.
- **ADR relacionada**: `docs/ADR-opencode-granular-permissions.md` (precursor: outputs de agentes ganharam `permission.task` estruturado; próximo passo é estender para todos os outputs).
- **ADR relacionada**: `docs/ADR-harness-trace-guard.md` — `ExecutionEvidence` em `tools/cmd/false-success-guard/detector.go:37-42` já é struct validável; este ADR generaliza o princípio.
- **JSON Schema lib**: `github.com/santhosh-tekuri/jsonschema/v6` (Apache-2.0, sem CGO, drafts 4/6/7/2019-09/**2020-12** completos, 1.3k stars, bowtie-conformant — validado em pkg.go.dev em 2026-09-19). **Correção**: a ADR original citou `v5` por engano — v5.3.1 é de 2023-07-22 e foi supercedida por v6. v6 traz extras (`custom $schema url`, vocabulary-based, contentSchema) sem custo para nosso caso. Decisão: começar com **v6** (major atual).
- **Sessão anterior (2026-09-19)**: identificou inconsistência — receipts órfãos sem path rastreável. Reforça a tese: outputs sem schema versionado são exatamente o tipo de artefato que vira órfão.
- **Pendência Gap 2 (budget tracking)**: continua rebaixado. Tabela própria de telemetria, sem reaproveitar `embedding_json` (já reservado para embeddings semânticas em `tools/internal/agentmemory/store.go:89`).