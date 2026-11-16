# Outbox e idempotencia (entrega at-least-once)

> Referencia carregada por `../SKILL.md` §6 (consistencia e transacoes). O assunto aqui e **o caminho da mensagem**: como um evento sai do banco e chega ao consumidor sem duplicar efeito.
> Nao duplica: consistencia entre agregados e compensacao ficam no `SKILL.md` §6; retry, timeout e circuit breaker ficam em `error-handling-patterns/reference/resilience.md` §2-§5; contrato de recurso e `Idempotency-Key` na borda HTTP ficam em `api-design-principles/reference/rest-best-practices.md`.
> Fonte e spec externa, nao livro: cada afirmacao tecnica aponta a URL verificada em `## Fontes`. Inferencia do autor esta marcada como **[inferencia]**.

## 1. O problema: dual write

Publicar um evento e mudar o estado sao **dois sistemas distintos**. Qualquer ordem deixa uma janela:

| Ordem | Falha | Resultado |
| --- | --- | --- |
| commit, depois publish | publish falha ou processo cai | estado mudou, evento nao existe: consumidor nunca sabe (perda silenciosa) |
| publish, depois commit | commit falha | evento anuncia estado que nao existe no banco (fantasma) |

- O nome do problema e `dual write`: a AWS Prescriptive Guidance define o padrao como resposta a "the dual write operations issue that occurs in distributed systems when a single operation involves both a database write operation and a message or event notification".
- Inverter a ordem nao resolve; escolhe o tipo de inconsistencia.
- 2PC nao e a saida: no catalogo de microservices.io, "it is not viable to use a traditional distributed transaction (2PC) that spans the database and the message broker", porque "the database and/or the message broker might not support 2PC".
- Havendo CDC disponivel, a AWS cita "an outbox table or change data capture (CDC)" como as duas implementacoes do mesmo objetivo. CDC e a variante sem relay proprio (§3).

## 2. A tabela outbox

Participantes nomeados no padrao (microservices.io): **Sender** (o servico que envia), **Database** (guarda entidades e outbox), **Message outbox** ("if it's a relational database, this is a table that stores the messages to be sent"), **Message relay** e **Message broker**.

Regra central: o `INSERT` na outbox acontece na **mesma transacao local** do `UPDATE`/`INSERT` de negocio. Se a transacao commita, existem estado e mensagem; se da rollback, nao existe nenhum dos dois. A atomicidade vem do banco, nao do broker.

Colunas minimas (**[inferencia]** de engenharia, nao texto de spec):

| Coluna | Para que |
| --- | --- |
| `id` | identidade da mensagem; vira `message_id` no broker e base da chave de idempotencia |
| `aggregate_id` | chave de particao e fronteira de ordenacao (§9) |
| `type` | roteamento e schema do payload |
| `payload` | corpo serializado |
| `created_at` | lag do relay, auditoria, janela de retencao |
| `published_at` (nullable) | `NULL` = pendente; indice parcial nessa coluna e o que sustenta o polling |
| `attempts`, `last_error` | diagnostico; alimenta a decisao de DLQ (§8) |

- Indice parcial em `published_at IS NULL` evita varredura na tabela que so cresce.
- Retencao e limpeza das linhas publicadas sao requisito, nao faxina opcional: tabela infinita degrada o polling.

## 3. Relay: polling ou tailing do log

Duas formas de mover a linha da outbox para o broker (microservices.io documenta ambas como padroes separados):

- **Polling publisher**: o relay consulta a outbox em intervalo fixo, publica o que esta pendente e marca.
- **Transaction log tailing**: "Tail the database transaction log and publish each message/event inserted into the outbox to the message broker". O mecanismo depende do banco: MySQL binlog, Postgres WAL, AWS DynamoDB table streams.

| Criterio | Polling | Log tailing |
| --- | --- | --- |
| Latencia | limitada pelo intervalo | proxima do tempo real |
| Custo no banco | query periodica, mesmo sem mensagem | leitura do log |
| Complexidade operacional | baixa | alta (formato de log, posicao, permissoes) |
| Acoplamento | so SQL | binlog/WAL/streams do fornecedor |

Ordem de operacao que evita duplicar em massa (**[inferencia]**):

1. publica no broker;
2. so depois marca `published_at`;
3. se o processo cair entre 1 e 2, a republicacao acontece — e aceitavel, porque o consumidor e idempotente (§5).

Marcar antes de publicar troca "duplicata" por "perda": pior negocio. Escolha deliberada, nunca default.

Com mais de uma instancia do relay, duas instancias podem pegar a mesma linha. Precisa de claim por linha (`SELECT ... FOR UPDATE SKIP LOCKED` no Postgres) ou lock equivalente — sem isso, o relay vira gerador de duplicata. **[inferencia]**

## 4. Anti-padrao: publicar no broker dentro da transacao

O erro tem forma reconhecivel: `BEGIN` → `INSERT` de negocio → `publish` no broker → `COMMIT`.

Por que quebra:

- **Commit pode falhar depois do publish**: o broker ja recebeu o evento de um estado que o rollback desfez. A transacao do banco nao desfaz o que saiu pela rede.
- **O publish nao participa do commit**: sucesso do broker nao e condicao de sucesso do banco, nem o contrario. E o mesmo dual write, so que escondido dentro de um bloco `BEGIN/COMMIT`.
- **I/O de rede dentro do lock**: a transacao segura lock de linha enquanto espera o broker; latencia do broker virou latencia de escrita e, sob carga, pool de conexoes esgotado.
- **Retry do publish dentro da transacao** duplica o evento sem que o banco saiba.
- **Nenhum ganho real**: se o broker participasse da transacao, seria 2PC — exatamente o que o padrao rejeita por falta de suporte.

Regra derivada (**[inferencia]** a partir do acima): **nenhum I/O de rede na transacao de escrita**. Commit curto, broker depois, fora da transacao, via outbox.

## 5. At-least-once e por que isso obriga idempotencia no consumidor

O relay nao consegue garantir entrega unica: ele pode publicar e falhar antes de marcar, ou o broker pode entregar duas vezes.

- Duplicata no broker e real em fila standard. A documentacao de exactly-once do SQS diz: "Unlike standard queues, FIFO queues don't introduce duplicate messages" — a frase so faz sentido porque a standard **pode** duplicar.
- Mesmo FIFO so elimina duplicata de **envio** dentro do intervalo de deduplicacao: "If you retry the `SendMessage` action within the 5-minute deduplication interval, Amazon SQS doesn't introduce any duplicates into the queue."
- Consequencia de design: o caminho e **at-least-once**; o efeito tem de ser **once**. Entrega unica ponta a ponta entre sistemas independentes nao e alcancavel em geral — o que se alcanca e processamento unico no consumidor. **[inferencia]** consolidada, consistente com o texto do SQS.

Portanto: deduplicacao no consumidor **nao e otimizacao, e requisito do padrao**. Quem adota outbox sem idempotencia no consumidor trocou perda silenciosa por duplicata silenciosa.

## 6. Chave de idempotencia

Nao e o `Idempotency-Key` do HTTP. Sao coisas diferentes, com escopos diferentes:

| Chave | Onde vive | Escopo |
| --- | --- | --- |
| `message_id` / `event_id` da outbox | envelope da mensagem | por mensagem publicada |
| `Idempotency-Key` | request HTTP do cliente | por operacao request/response (§Fontes: Internet-Draft) |

Propriedades que a chave precisa ter:

- **Derivada do fato, nao do transporte**: `event_id` gerado no `INSERT` da outbox sobrevive a retry do relay, a restart do consumidor e a replay de topico. Request id de HTTP nao sobrevive a nada disso.
- **Estavel entre tentativas**: se a chave muda no retry, a deduplicacao nunca acerta.
- **Escopada por consumidor**: cada consumidor tem seu proprio registro de dedupe; dois consumidores com a mesma chave nao se atrapalham. **[inferencia]**
- **Granularidade do efeito**: dedupe por `message_id` protege contra reentrega; dedupe por chave natural do dominio (`pedido_id` + versao) protege tambem contra dois eventos diferentes que representam o mesmo fato. **[inferencia]**

Sobre o header HTTP: `Idempotency-Key` ainda **nao e RFC**. O documento e `draft-ietf-httpapi-idempotency-key-header-08`, Internet-Draft ativo do WG httpapi, atualizado em 2026-10-22; o abstract descreve "The HTTP Idempotency-Key request header field can be used to make non-idempotent HTTP methods such as POST or PATCH fault-tolerant". Tratar como draft, nao como norma publicada.

Pratica de fornecedor: o Stripe "saves the status code and body" da primeira request por chave e devolve o mesmo resultado na repeticao. A janela exata de retencao da chave **nao foi confirmada** no trecho que li (ver `## Fontes`).

## 7. Deduplicacao no consumidor

- Registro de dedupe: tabela `(consumer, idempotency_key)` com constraint `UNIQUE` e `processed_at`.
- **O efeito e o registro de dedupe vao na mesma transacao.** Marcar "processado" fora da transacao recria o dual write no consumidor, um nivel abaixo.
- Corrida entre duas entregas simultaneas: `INSERT` que viola o `UNIQUE` significa "outro ja processou". Tratar a violacao como sucesso idempotente, nao como erro.
- **Janela**: dedupe infinito custa storage; janela curta deixa passar reprocesso tardio (replay de topico, redrive de DLQ depois de dias). A janela precisa cobrir o maior tempo de reprocesso que a operacao admite — nao um numero escolhido por habito. **[inferencia]**
- Referencia concreta de janela curta: o SQS FIFO deduplica por 5 minutos. Se o redrive da DLQ acontece 1 hora depois, o broker nao deduplica nada: a protecao e a do consumidor.
- Operacao idempotente e mais barata que dedupe: `UPSERT` com versao (`WHERE version < :nova`) dispensa tabela de dedupe quando o dominio tem versao monotonica. **[inferencia]**

## 8. DLQ: o que vai e o que nunca vai

Mecanismo (SQS, redrive policy): "The redrive policy redirects messages to a dead-letter queue after the source queue fails to process a message a specified number of times", com `maxReceiveCount` default 10; a DLQ precisa estar "in the same AWS account and Region as the source queue".

**Vai para a DLQ** — falha deterministica, que nao melhora com repeticao:

- poison pill: mensagem que derruba o handler toda vez;
- schema incompativel: payload que o consumidor nao consegue desserializar;
- referencia inexistente: `aggregate_id` que nao existe e nunca vai existir;
- violacao de invariante de negocio que exige intervencao humana;
- payload acima do limite aceito.

**Nunca vai para a DLQ** — e aqui esta a decisao que importa:

- **erro transitorio** (timeout de banco, 503 de dependencia, lock): retry com backoff resolve; mandar para DLQ transforma incidente de 30 s em fila manual de horas;
- **erro que exige deploy**: a mensagem na DLQ continua quebrada depois do deploy; o caminho e corrigir, drenar e reprocessar, nao acumular;
- **falha de configuracao do consumidor** (credencial, permissao, topico errado): e falha do sistema, nao da mensagem; DLQ mascara o problema;
- **mensagem cuja causa ja foi corrigida**: reprocessar direto no topico de origem, sem passar pela DLQ.

DLQ sem alarme, sem dono e sem processo de redrive e cemiterio de dados: as mensagens existem, ninguem sabe, e a retencao expira. Os tres itens sao pre-requisito, nao melhoria.

## 9. Ordem de mensagem vs particao

- No Kafka, a particao e "an ordered, immutable sequence of records"; o offset identifica a posicao do record dentro dela.
- Ordem so existe **dentro** da particao. Consumindo varias particoes, a ordem entre elas nao e garantida.
- Consequencia direta: **a chave de particao define a fronteira de ordenacao**. Particionar por `aggregate_id` preserva a ordem dos eventos daquele agregado — e so dele.
- Ordem global em topico raramente e o requisito real; ordem **por agregado** quase sempre e. Se o consumidor pede ordem global, vale perguntar qual invariante quebra sem ela. **[inferencia]**
- Paralelismo dentro do consumidor desfaz a garantia: N threads na mesma particao processam fora de ordem. Ordem por agregado exige serializacao por chave, nao por topico. **[inferencia]**
- Quando a ordem nao pode ser garantida (multiplas particoes, relay concorrente, replay), o substituto e **versao no payload**: o consumidor descarta evento com versao menor ou igual a aplicada. Troca ordenacao por convergencia. **[inferencia]**

## 10. Checklist

- [ ] `INSERT` na outbox na mesma transacao do estado; nenhum I/O de rede dentro da transacao.
- [ ] Relay publica antes de marcar `published_at`; duplicata aceita, perda nao.
- [ ] Relay com claim por linha quando ha mais de uma instancia.
- [ ] `event_id` gerado no banco, estavel entre tentativas.
- [ ] Consumidor com registro de dedupe na mesma transacao do efeito.
- [ ] Janela de dedupe maior que o maior tempo de reprocesso admitido.
- [ ] Politica de DLQ escrita: o que vai, o que nao vai, `maxReceiveCount`, dono, alarme, redrive.
- [ ] Chave de particao = chave do agregado, quando ordem por agregado e requisito.
- [ ] Versao no payload quando ordem nao e garantida.
- [ ] Retencao e limpeza da outbox e da tabela de dedupe definidas.

## 11. Anti-padroes

- **Publish dentro do `BEGIN/COMMIT`**: dual write disfarcado de transacao.
- **Marcar `published_at` antes do publish**: perda silenciosa garantida em falha.
- **Outbox sem consumidor idempotente**: duplicata silenciosa; o problema so mudou de lugar.
- **Dedupe fora da transacao do efeito**: dual write um nivel abaixo.
- **DLQ como retry**: erro transitorio enterrado em fila manual.
- **DLQ sem alarme**: mensagens expiram sem ninguem saber.
- **Ordem global como requisito**: particao unica para resolver com serializacao o que deveria ser ordem por agregado.
- **Tabela outbox sem limpeza**: polling degrada ate o relay parar de acompanhar.

## Fontes

- **Pattern: Transactional outbox** (microservices.io) — participantes, problema "How to atomically update the database and send messages to a message broker?", rejeicao de 2PC. https://microservices.io/patterns/data/transactional-outbox.html
- **Pattern: Transaction log tailing** (microservices.io) — solution "Tail the database transaction log and publish each message/event inserted into the outbox"; MySQL binlog, Postgres WAL, DynamoDB streams. https://microservices.io/patterns/data/transaction-log-tailing.html (texto lido em espelho do mesmo padrao: https://gallery.lanxiaohe.com/patterns/data/transaction-log-tailing.html)
- **Transactional outbox pattern — AWS Prescriptive Guidance** — "resolves the dual write operations issue"; outbox table ou CDC. https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html
- **Exactly-once processing in Amazon SQS** — "Unlike standard queues, FIFO queues don't introduce duplicate messages"; intervalo de deduplicacao de 5 minutos. https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/FIFO-queues-exactly-once-processing.html
- **Using dead-letter queues in Amazon SQS** — redrive policy, mesma conta/Region. https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html
- **Capturing problematic messages in Amazon SQS** — redrive policy "after the source queue fails to process a message a specified number of times". https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/capturing-problematic-messages.html
- **Understand the Amazon SQS dead-letter queue** (AWS re:Post) — `maxReceiveCount` default 10. https://repost.aws/knowledge-center/sqs-dead-letter-queue
- **Apache Kafka — Introduction** — particao como "an ordered, immutable sequence of records"; offset. https://kafka.apache.org/24/getting-started/introduction/
- **Red Hat Streams for Apache Kafka — consumer configuration tuning §4.3** — "Kafka only provides ordering guarantees for messages in a single partition". https://docs.redhat.com/en/documentation/red_hat_streams_for_apache_kafka/2.5/html/kafka_configuration_tuning/con-consumer-config-properties-str
- **draft-ietf-httpapi-idempotency-key-header-08** — Internet-Draft ativo (nao RFC), abstract do `Idempotency-Key`. https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/
- **Stripe API — Idempotent requests** — "saves the status code and body" da primeira request por chave. https://docs.stripe.com/api/idempotent_requests
- **Nao confirmado**: janela exata de retencao da chave de idempotencia no Stripe; semantica literal de `MessageGroupId` no SQS FIFO.
