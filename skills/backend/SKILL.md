---
name: backend
description: "Use when writing, reviewing or debugging server-side code in Go/Golang, Rust, TypeScript, Node or PHP/Symfony — goroutine, channel, concorrência, CLI, API, transação, migration, teste de unidade. Aplica princípios de backend (fronteiras, contratos, concorrência, persistência, observabilidade) e roteia para reference/go.md, reference/ts.md, reference/php.md, reference/rust.md."
---

# Backend

Camada de **princípios** para código de servidor, válida em qualquer stack do catálogo. Não é catálogo de API nem manual de framework: sintaxe, biblioteca e receita por linguagem ficam em `reference/`.

## Quando dispara

- Escrever/revisar código de servidor em **Go/Golang, Rust, TypeScript, Node, PHP/Symfony**.
- Decidir **concorrência** (goroutine, channel, worker pool, mutex, async), **CLI**, **API HTTP**, **transação**, **migration**, **teste de unidade**.
- Dúvida de fronteira: onde fica regra de negócio, o que é payload, o que é infraestrutura.

## Quando NÃO dispara

- Desenho de sistema, camadas macro, ADR → `architecture`.
- Teste E2E de navegador/UI → `e2e-testing-patterns`.
- Refactor puro de forma (método longo, `else` em cascata) → `object-calisthenics`.
- Tuning de SQL/índice isolado → `sql-optimization-patterns`.
- Contrato público de API (OpenAPI) → `openapi-spec-generation`; modelagem de domínio → `architecture` (reference/domain-modeling.md).

## 1. Fronteiras (dependency rule aplicada a código)

A dependência aponta para dentro: `handler → caso de uso → domínio`, nunca o inverso.

- O domínio não importa `net/http`, `express`, ORM, `Symfony\Component\...` nem `tauri`.
- Infraestrutura **implementa** interfaces declaradas por dentro (port); não as dita.
- Teste objetivo da regra: o pacote de domínio compila sem a camada de I/O (`go build ./domain/...`).

## 2. Contratos

- **Erro é valor, não exceção de fluxo:** em Go `return fmt.Errorf("carregar pedido %s: %w", id, err)`; em TS `Result<T, E>` explícito ou erro tipado na borda. `panic`/`throw` só para bug de programação.
- **Validação na borda:** um único ponto valida o payload (DTO/schema) e converte para tipo do domínio. Depois da borda o tipo garante a invariante — nada de `if` defensivo espalhado.
- **Nunca atravessa a borda:** entidade de ORM, `Request`/`Response` do framework, `context.Context` de transporte dentro do domínio, SQL cru na aplicação, `time.Now()` sem clock injetado, segredo em log.
- Erro exposto ao cliente é estável (código + mensagem curta); detalhe interno fica no log correlacionado.

## 3. Concorrência: critério e custo

Escolha pela **forma do trabalho**, não por moda:

| trabalho | escolha | custo |
|---|---|---|
| I/O concorrente, fan-out curto, ≥4 tarefas independentes | goroutine + `errgroup` / `Promise.all` limitado | exige cancelamento e limite de paralelismo explícitos |
| mutação de estado compartilhado | `sync.Mutex` ou canal serializando acesso único | contenção; operação curta → mutex vence canal |
| CPU-bound pesado | worker pool fixo (`GOMAXPROCS`) / `worker_threads` | satura o processo: nunca por request |
| sequencial simples, < ~10 ms no total | **nada** | concorrência aqui é só bug novo |

Todo caminho concorrente tem **cancelamento** (contexto/`AbortSignal`), **limite** e **espera** (WaitGroup/`close`). Goroutine vazada e promise sem `await` são o mesmo defeito com nomes diferentes.

## 4. Persistência

- **Transação** tem fronteira no caso de uso — não no repositório, não no controller. Um handler, no máximo uma transação.
- **Migration** é versionada no VCS, idempotente e **nunca** editada depois de aplicada. Destrutivo vai em passos compatíveis: expand (coluna nullable) → backfill → contract (drop).
- **Idempotência:** escrita acionada por evento/retry carrega chave idempotente (id da mensagem, `unique constraint`). Retry sem idempotência é corrupção com atraso.
- N+1 e lazy loading em loop são defeito de fronteira: o dado sai carregado do repositório, não é puxado durante a serialização.

## 5. Testes: o que cada nível prova

| nível | prova | desperdício nele |
|---|---|---|
| unidade | regra e caso de borda do domínio puro | subir container/DB; mockar o que você não possui |
| integração | contrato com DB/fila/HTTP real | reassertar a regra já provada na unidade |
| ponta a ponta | que a composição liga (um caminho feliz) | matriz de regra por E2E: caro e frágil |

- Teste que não pode falhar por motivo de negócio é cerimônia: delete.
- Determinismo: relógio, aleatoriedade e rede injetados. `sleep` no teste é flake agendado.
- Cobertura é consequência, não meta: 100% de getter não paga um caso de borda.

## 6. Observabilidade

- **Log estruturado** (chave/valor, `slog` / `pino`), nunca prosa montada com `fmt.Sprintf`: campo pesquisável vence frase bonita.
- **Correlação:** id de requisição/trace no contexto desde a borda e repetido em todo log do caminho — inclusive no erro devolvido ao cliente.
- Loga **uma vez por falha**, onde há contexto para decidir (não em cada `if err`); sem segredo/PII.
- Meça a fronteira: latência por rota/dependência e contagem de erro por código. Sem número, "está lento" é opinião.

## 7. Roteamento para reference/

| abra | quando |
|---|---|
| [reference/go.md](reference/go.md) | Go/Golang: goroutine, channel, worker pool, `context`, profiling — playbook em [reference/go/implementation-playbook.md](reference/go/implementation-playbook.md) |
| [reference/ts.md](reference/ts.md) | TypeScript/JavaScript/Node: tipagem estrita, async, Express/Fastify, middleware — playbook em [reference/ts/implementation-playbook.md](reference/ts/implementation-playbook.md) |
| [reference/php.md](reference/php.md) | PHP/Symfony/Doctrine/PHPUnit — **não mantido** (0 projeto PHP ativo); só consulta pontual |
| [reference/rust.md](reference/rust.md) | Rust: ownership/borrowing e quando vale a pena nesta máquina (rascunho sobre os crates locais) |

Não abra reference por reflexo: abra quando a pergunta é **de linguagem** (sintaxe, API de biblioteca, idioma). Pergunta de design fica neste arquivo.

## 8. Anti-padrões (sintoma concreto)

- **Handler gordo:** arquivo de rota com >50 linhas misturando validação, SQL e regra → a regra vazou para o transporte.
- **Serviço-deus:** um `Service` com 15 dependências injetadas; mudar um requisito toca quatro métodos não relacionados.
- **`panic`/`catch` genérico** engolindo erro: log sem causa, 500 com `Request failed`, retry que nunca converge.
- **Transação por repositório:** cada `save` abre transação; duas escritas relacionadas ficam não atômicas sob concorrência.
- **Goroutine/promise órfã:** processo não encerra, teste trava, memória cresce em degrau.
- **Mock no lugar de integração:** suíte verde e produção quebrada no contrato do banco — sintoma: nenhum teste falha ao renomear a coluna.
- **Log em prosa:** `grep` não encontra a requisição problemática; correlação perdida entre serviços.

## 9. Checklist final

- [ ] A dependência aponta para dentro? o domínio compila sem a camada de I/O?
- [ ] Erro carrega contexto (`%w`/`cause`) e é tratado uma única vez?
- [ ] Todo caminho concorrente tem cancelamento, limite e espera?
- [ ] A escrita é idempotente e a transação fica no caso de uso?
- [ ] A migration é aditiva e reversível no plano?
- [ ] Cada nível de teste prova o que só ele prova?
- [ ] Log estruturado com correlação, sem segredo/PII?
- [ ] Comportamento externo mudou? então há teste novo — senão, é refactor.
