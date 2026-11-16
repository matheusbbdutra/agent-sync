# Outbox and idempotency (at-least-once delivery)

> Reference loaded by `../SKILL.md` §6 (consistency and transactions). The subject here is **the path of the message**: how an event leaves the database and reaches the consumer without duplicating effect.
> It does not duplicate: consistency between aggregates and compensation stay in `SKILL.md` §6; retry, timeout and circuit breaker stay in `error-handling-patterns/reference/resilience.md` §2-§5; resource contract and `Idempotency-Key` at the HTTP edge stay in `api-design-principles/reference/rest-best-practices.md`.
> Source is an external spec, not a book: each technical claim points to the URL verified in `## Sources`. Author's inference is marked as **[inference]**.

## 1. The problem: dual write

Publishing an event and changing the state are **two distinct systems**. Any order leaves a window:

| Order | Failure | Result |
| --- | --- | --- |
| commit, then publish | publish fails or the process dies | state changed, event does not exist: the consumer never knows (silent loss) |
| publish, then commit | commit fails | the event announces a state that does not exist in the database (ghost) |

- The name of the problem is `dual write`: AWS Prescriptive Guidance defines the pattern as the answer to "the dual write operations issue that occurs in distributed systems when a single operation involves both a database write operation and a message or event notification".
- Inverting the order does not solve it; it chooses the type of inconsistency.
- 2PC is not the way out: in the microservices.io catalog, "it is not viable to use a traditional distributed transaction (2PC) that spans the database and the message broker", because "the database and/or the message broker might not support 2PC".
- Where CDC is available, AWS cites "an outbox table or change data capture (CDC)" as the two implementations of the same goal. CDC is the variant with no relay of its own (§3).

## 2. The outbox table

Participants named in the pattern (microservices.io): **Sender** (the service that sends), **Database** (stores entities and the outbox), **Message outbox** ("if it's a relational database, this is a table that stores the messages to be sent"), **Message relay** and **Message broker**.

Central rule: the `INSERT` in the outbox happens in the **same local transaction** as the business `UPDATE`/`INSERT`. If the transaction commits, state and message exist; if it rolls back, neither of the two exists. Atomicity comes from the database, not from the broker.

Minimum columns (**[inference]** from engineering, not spec text):

| Column | What for |
| --- | --- |
| `id` | identity of the message; becomes `message_id` in the broker and the basis of the idempotency key |
| `aggregate_id` | partition key and ordering boundary (§9) |
| `type` | routing and payload schema |
| `payload` | serialized body |
| `created_at` | relay lag, audit, retention window |
| `published_at` (nullable) | `NULL` = pending; the partial index on this column is what sustains the polling |
| `attempts`, `last_error` | diagnostics; feeds the DLQ decision (§8) |

- A partial index on `published_at IS NULL` avoids a scan on the table that only grows.
- Retention and cleanup of the published rows are a requirement, not optional housekeeping: an infinite table degrades the polling.

## 3. Relay: polling or log tailing

Two ways of moving the outbox row to the broker (microservices.io documents both as separate patterns):

- **Polling publisher**: the relay queries the outbox at a fixed interval, publishes what is pending and marks it.
- **Transaction log tailing**: "Tail the database transaction log and publish each message/event inserted into the outbox to the message broker". The mechanism depends on the database: MySQL binlog, Postgres WAL, AWS DynamoDB table streams.

| Criterion | Polling | Log tailing |
| --- | --- | --- |
| Latency | limited by the interval | close to real time |
| Cost on the database | periodic query, even with no message | log read |
| Operational complexity | low | high (log format, position, permissions) |
| Coupling | SQL only | vendor binlog/WAL/streams |

Operating order that avoids mass duplication (**[inference]**):

1. publish to the broker;
2. only then mark `published_at`;
3. if the process dies between 1 and 2, the republication happens — and it is acceptable, because the consumer is idempotent (§5).

Marking before publishing trades "duplicate" for "loss": a worse deal. A deliberate choice, never a default.

With more than one instance of the relay, two instances can pick the same row. It needs a per-row claim (`SELECT ... FOR UPDATE SKIP LOCKED` in Postgres) or an equivalent lock — without that, the relay becomes a duplicate generator. **[inference]**

## 4. Anti-pattern: publishing to the broker inside the transaction

The error has a recognizable shape: `BEGIN` → business `INSERT` → `publish` to the broker → `COMMIT`.

Why it breaks:

- **The commit can fail after the publish**: the broker has already received the event of a state that the rollback undid. The database transaction does not undo what left through the network.
- **The publish does not take part in the commit**: broker success is not a condition of database success, nor the other way around. It is the same dual write, only hidden inside a `BEGIN/COMMIT` block.
- **Network I/O inside the lock**: the transaction holds a row lock while it waits for the broker; broker latency became write latency and, under load, an exhausted connection pool.
- **A retry of the publish inside the transaction** duplicates the event without the database knowing.
- **No real gain**: if the broker took part in the transaction, it would be 2PC — exactly what the pattern rejects for lack of support.

Derived rule (**[inference]** from the above): **no network I/O in the write transaction**. Short commit, broker afterwards, outside the transaction, via outbox.

## 5. At-least-once and why it forces idempotency on the consumer

The relay cannot guarantee a single delivery: it can publish and fail before marking, or the broker can deliver twice.

- A duplicate in the broker is real in a standard queue. The SQS exactly-once documentation says: "Unlike standard queues, FIFO queues don't introduce duplicate messages" — the sentence only makes sense because the standard one **can** duplicate.
- Even FIFO only eliminates **send** duplicates within the deduplication interval: "If you retry the `SendMessage` action within the 5-minute deduplication interval, Amazon SQS doesn't introduce any duplicates into the queue."
- Design consequence: the path is **at-least-once**; the effect has to be **once**. A single end-to-end delivery between independent systems is not achievable in general — what is achieved is single processing at the consumer. **[inference]** consolidated, consistent with the SQS text.

Therefore: deduplication at the consumer **is not an optimization, it is a requirement of the pattern**. Whoever adopts outbox without idempotency at the consumer traded silent loss for silent duplication.

## 6. Idempotency key

It is not the HTTP `Idempotency-Key`. They are different things, with different scopes:

| Key | Where it lives | Scope |
| --- | --- | --- |
| `message_id` / `event_id` of the outbox | message envelope | per published message |
| `Idempotency-Key` | client HTTP request | per request/response operation (§Sources: Internet-Draft) |

Properties the key needs to have:

- **Derived from the fact, not from the transport**: `event_id` generated in the outbox `INSERT` survives a relay retry, a consumer restart and a topic replay. An HTTP request id survives none of that.
- **Stable across attempts**: if the key changes on the retry, deduplication never gets it right.
- **Scoped per consumer**: each consumer has its own dedupe record; two consumers with the same key do not get in each other's way. **[inference]**
- **Granularity of the effect**: dedupe by `message_id` protects against redelivery; dedupe by the domain's natural key (`pedido_id` + version) also protects against two different events that represent the same fact. **[inference]**

About the HTTP header: `Idempotency-Key` is still **not an RFC**. The document is `draft-ietf-httpapi-idempotency-key-header-08`, an active Internet-Draft of the httpapi WG, updated on 2026-10-22; the abstract describes "The HTTP Idempotency-Key request header field can be used to make non-idempotent HTTP methods such as POST or PATCH fault-tolerant". Treat it as a draft, not as a published standard.

Vendor practice: Stripe "saves the status code and body" of the first request per key and returns the same result on the repetition. The exact retention window of the key **was not confirmed** in the excerpt I read (see `## Sources`).

## 7. Deduplication at the consumer

- Dedupe record: table `(consumer, idempotency_key)` with a `UNIQUE` constraint and `processed_at`.
- **The effect and the dedupe record go in the same transaction.** Marking "processed" outside the transaction recreates the dual write at the consumer, one level down.
- Race between two simultaneous deliveries: an `INSERT` that violates the `UNIQUE` means "another one already processed it". Treat the violation as idempotent success, not as an error.
- **Window**: infinite dedupe costs storage; a short window lets a late reprocess through (topic replay, DLQ redrive after days). The window must cover the longest reprocess time that the operation admits — not a number chosen out of habit. **[inference]**
- Concrete reference for a short window: SQS FIFO deduplicates for 5 minutes. If the DLQ redrive happens 1 hour later, the broker deduplicates nothing: the protection is the consumer's.
- An idempotent operation is cheaper than dedupe: an `UPSERT` with version (`WHERE version < :nova`) dispenses with the dedupe table when the domain has a monotonic version. **[inference]**

## 8. DLQ: what goes and what never goes

Mechanism (SQS, redrive policy): "The redrive policy redirects messages to a dead-letter queue after the source queue fails to process a message a specified number of times", with `maxReceiveCount` default 10; the DLQ must be "in the same AWS account and Region as the source queue".

**Goes to the DLQ** — deterministic failure, which does not improve with repetition:

- poison pill: a message that takes the handler down every time;
- incompatible schema: payload that the consumer cannot deserialize;
- non-existent reference: an `aggregate_id` that does not exist and never will;
- violation of a business invariant that requires human intervention;
- payload above the accepted limit.

**Never goes to the DLQ** — and here is the decision that matters:

- **transient error** (database timeout, dependency 503, lock): a retry with backoff solves it; sending it to the DLQ turns a 30 s incident into a manual queue of hours;
- **error that requires a deploy**: the message in the DLQ is still broken after the deploy; the path is to fix, drain and reprocess, not to accumulate;
- **consumer configuration failure** (credential, permission, wrong topic): it is a failure of the system, not of the message; the DLQ masks the problem;
- **message whose cause has already been fixed**: reprocess directly in the origin topic, without going through the DLQ.

A DLQ without an alarm, without an owner and without a redrive process is a data graveyard: the messages exist, nobody knows, and the retention expires. The three items are a prerequisite, not an improvement.

## 9. Message order vs partition

- In Kafka, the partition is "an ordered, immutable sequence of records"; the offset identifies the position of the record inside it.
- Order only exists **inside** the partition. Consuming several partitions, the order between them is not guaranteed.
- Direct consequence: **the partition key defines the ordering boundary**. Partitioning by `aggregate_id` preserves the order of that aggregate's events — and only of that one.
- Global order in a topic is rarely the real requirement; **per-aggregate** order almost always is. If the consumer asks for global order, it is worth asking which invariant breaks without it. **[inference]**
- Parallelism inside the consumer undoes the guarantee: N threads in the same partition process out of order. Per-aggregate order requires serialization by key, not by topic. **[inference]**
- When the order cannot be guaranteed (multiple partitions, concurrent relay, replay), the substitute is **version in the payload**: the consumer discards an event with a version lower than or equal to the applied one. It trades ordering for convergence. **[inference]**

## 10. Checklist

- [ ] `INSERT` in the outbox in the same transaction as the state; no network I/O inside the transaction.
- [ ] The relay publishes before marking `published_at`; duplicate accepted, loss not.
- [ ] Relay with a per-row claim when there is more than one instance.
- [ ] `event_id` generated in the database, stable across attempts.
- [ ] Consumer with a dedupe record in the same transaction as the effect.
- [ ] Dedupe window larger than the longest admitted reprocess time.
- [ ] Written DLQ policy: what goes, what does not go, `maxReceiveCount`, owner, alarm, redrive.
- [ ] Partition key = aggregate key, when per-aggregate order is a requirement.
- [ ] Version in the payload when order is not guaranteed.
- [ ] Retention and cleanup of the outbox and of the dedupe table defined.

## 11. Anti-patterns

- **Publish inside the `BEGIN/COMMIT`**: dual write disguised as a transaction.
- **Marking `published_at` before the publish**: silent loss guaranteed on failure.
- **Outbox without an idempotent consumer**: silent duplication; the problem only changed places.
- **Dedupe outside the transaction of the effect**: dual write one level down.
- **DLQ as a retry**: transient error buried in a manual queue.
- **DLQ without an alarm**: messages expire with nobody knowing.
- **Global order as a requirement**: a single partition to solve with serialization what should be per-aggregate order.
- **Outbox table without cleanup**: polling degrades until the relay stops keeping up.

## Sources

- **Pattern: Transactional outbox** (microservices.io) — participants, problem "How to atomically update the database and send messages to a message broker?", rejection of 2PC. https://microservices.io/patterns/data/transactional-outbox.html
- **Pattern: Transaction log tailing** (microservices.io) — solution "Tail the database transaction log and publish each message/event inserted into the outbox"; MySQL binlog, Postgres WAL, DynamoDB streams. https://microservices.io/patterns/data/transaction-log-tailing.html (text read in a mirror of the same pattern: https://gallery.lanxiaohe.com/patterns/data/transaction-log-tailing.html)
- **Transactional outbox pattern — AWS Prescriptive Guidance** — "resolves the dual write operations issue"; outbox table or CDC. https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/transactional-outbox.html
- **Exactly-once processing in Amazon SQS** — "Unlike standard queues, FIFO queues don't introduce duplicate messages"; deduplication interval of 5 minutes. https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/FIFO-queues-exactly-once-processing.html
- **Using dead-letter queues in Amazon SQS** — redrive policy, same account/Region. https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html
- **Capturing problematic messages in Amazon SQS** — redrive policy "after the source queue fails to process a message a specified number of times". https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/capturing-problematic-messages.html
- **Understand the Amazon SQS dead-letter queue** (AWS re:Post) — `maxReceiveCount` default 10. https://repost.aws/knowledge-center/sqs-dead-letter-queue
- **Apache Kafka — Introduction** — partition as "an ordered, immutable sequence of records"; offset. https://kafka.apache.org/24/getting-started/introduction/
- **Red Hat Streams for Apache Kafka — consumer configuration tuning §4.3** — "Kafka only provides ordering guarantees for messages in a single partition". https://docs.redhat.com/en/documentation/red_hat_streams_for_apache_kafka/2.5/html/kafka_configuration_tuning/con-consumer-config-properties-str
- **draft-ietf-httpapi-idempotency-key-header-08** — active Internet-Draft (not an RFC), abstract of `Idempotency-Key`. https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/
- **Stripe API — Idempotent requests** — "saves the status code and body" of the first request per key. https://docs.stripe.com/api/idempotent_requests
- **Not confirmed**: exact retention window of the idempotency key in Stripe; literal semantics of `MessageGroupId` in SQS FIFO.
