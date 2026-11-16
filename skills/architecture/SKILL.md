---
name: architecture
description: "Use when modelar domínio complexo, decidir fronteiras ou revisar arquitetura. Gatilhos: DDD, bounded context (contexto delimitado), agregado/aggregate root, linguagem ubíqua, Clean Architecture, Hexagonal (portas e adaptadores), dependency rule entre camadas, ADR/decision record, monolito modular, decomposição em microsserviços, revisão arquitetural."
---

# Arquitetura

Fronteira antes de padrão. Só existe decisão arquitetural quando há uma dor concreta: invariante difícil de manter, mudança que atravessa módulos, contrato instável entre times. Sem dor, aplicar padrão é custo puro.

## 0. Pré-voo (antes de propor qualquer coisa)

1. **Skill especializada** — o assunto já tem dono? Ver §7 e carregue a vizinha em vez de reimplementar aqui. Não responda de memória genérica quando o repo já tem material curado.
2. **Decisão vigente** — procure em `docs/ADR-*.md` e na memória do projeto. Contradizer decisão registrada exige discussão explícita, nunca omissão silenciosa.
3. **Código real** — leia o trecho e cite `path:line`. Não descreva comportamento presumido.
4. **Profundidade declarada** — responda §1 antes de modelar: domínio CRUD não recebe DDD.

Toda resposta separa três origens: o que vem desta skill, o que vem de decisão registrada e o que é análise nova com `path:line`. Decisão aceita volta para a memória (`store_memory`, `type: project`, `scratch: false`, proveniência correta) — sem isso a consulta do passo 2 não a encontra depois.

## 1. DDD paga a conta?

| Sinal no domínio | Use DDD | Não use |
|---|---|---|
| Regras com exceções que mudam por contrato | sim | CRUD, telas, relatórios |
| Mesmo termo, sentidos diferentes por área | sim | vocabulário único e estável |
| Fronteiras de módulo/time em disputa | sim | app pequena, um time, sem fronteira |
| Evolução do modelo prevista por anos | sim | protótipo descartável |

Detalhe estratégico e tático: [reference/domain-modeling.md](reference/domain-modeling.md). Carregue quando a tarefa envolver invariante, agregado ou bounded context.

## 2. Estilo arquitetural e dependency rule

- **Dependência aponta para dentro**: regra de negócio não conhece framework, banco nem HTTP.
- **Hexagonal**: o núcleo expõe portas (interfaces) e a infraestrutura implementa adaptadores; trocar tecnologia vira trocar adaptador.
- **Camadas por nome não são arquitetura**: `service/` e `utils/` com import cruzado livre mantêm o acoplamento intacto.

Estrutura de pacotes em Go (`internal/`) e TypeScript, com o mesmo exemplo resolvido nas duas linguagens: [reference/architectural-styles.md](reference/architectural-styles.md).

## 3. Padrões de código (GoF e de domínio)

Padrão só entra com **eixo de variação real**. Sinais legítimos: `switch` que cresce a cada caso novo, tipo de fornecedor vazando para a regra de negócio, SDK de terceiro acoplado ao núcleo.

Catálogo, quando usar/evitar e checklist anti-over-engineering: [reference/design-patterns.md](reference/design-patterns.md).

## 4. Revisão arquitetural

Ordene os achados por risco, não por gosto. Cada achado leva severidade, evidência `path:line` e alternativa com trade-off.

- **Blocker** — viola dependency rule, compartilha entidade/tabela entre contextos, invariante sem dono único.
- **Risco** — N+1, transação atravessando agregados, contrato sem versão, acoplamento a fornecedor sem ACL.
- **Débito aceitável** — atalho declarado, com gatilho de revisão.

Rubrica completa (qualidade, escalabilidade, consistência, gates): [reference/review-rubric.md](reference/review-rubric.md).

Quando a discussão for escala (performance vs escalabilidade, CAP, failover, shard, cache write-behind, back pressure): [reference/scalability-tradeoffs.md](reference/scalability-tradeoffs.md).

## 5. Registrar a decisão (ADR)

Decisão estruturante sem registro volta como discussão. A ADR cobre contexto, opções descartadas e consequências — inclusive as negativas.

Neste repo: documento em `docs/ADR-<topico>.md`, ciclo `Proposto → Implementado → Aceito → Superado`, vínculo com `STATE.md`/`.agent-sync/session-state.json` por `D-N`/`A-N`.

Template e anti-patterns: [reference/adr.md](reference/adr.md).

## 6. Consistência e transações

- Invariante **dentro** do agregado: transação imediata, uma só.
- Invariante **entre** agregados: consistência eventual por evento, com política de compensação.
- Regra que precisa ser atômica entre dois "agregados" é sinal de que são um agregado só.

## 7. Rotas (não duplicar aqui)

| Assunto | Skill |
|---|---|
| Decomposição, service boundaries, resiliência distribuída | `microservices-patterns` |
| CQRS, projeções, read models | `cqrs-implementation` |
| Event sourcing, event store, replay | `cqrs-implementation` (reference/event-sourcing-architect.md) |
| Regras de código: indentação, sem `else`, coleções de primeira classe | `object-calisthenics` |
| Persistência, migration, transação de banco | `backend`, `sql-optimization-patterns` |
| Contrato de API e versionamento | `api-design-principles`, `openapi-spec-generation` |
| Idioma e idioms de Go/TS | `backend` (reference/go.md, reference/ts.md) |
| Delegação a outro CLI com este contexto embutido | `agent-delegate` |

## 8. Gates de verificação

- `agent-sync skills lint` → 0 errors; sem warning `collision` citando `architecture`.
- `agent-sync doctor` → `skills_index` com `missing=0` e `orphan=0` após mexer em `skills/`.
- Código: `go build ./... && go test ./...`; TS: `tsc --noEmit` mais os testes do pacote tocado.

## 9. Anti-padrões

- Camada criada "para o futuro"; interface com uma única implementação estável.
- DDD em CRUD; agregado deus com dezenas de entidades e trava de concorrência.
- Entidade de um contexto importada em outro; tabela compartilhada como integração.
- Repository devolvendo entidade ORM; SQL/HTTP dentro do domínio.
- ADR que omite trade-off ou é editada depois de aceita.
- "Microsserviço" com banco compartilhado: é monolito distribuído.

## 10. Checklist antes de entregar

- [ ] Cada afirmação tem `path:line` ou fonte externa citada.
- [ ] Dependências apontam para o domínio; nenhuma infraestrutura na regra de negócio.
- [ ] Fronteiras e donos explícitos; nenhum modelo compartilhado sem tradução (ACL).
- [ ] Agregados pequenos, raiz única, transação respeitando o limite.
- [ ] Decisão registrada em ADR quando muda contrato, fronteira ou tecnologia.
- [ ] Trade-off declarado; alternativa descartada citada.
