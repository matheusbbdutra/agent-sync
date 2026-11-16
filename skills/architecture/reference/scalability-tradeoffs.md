# Scale trade-offs

Source: `donnemartin/system-design-primer` (CC BY 4.0). Rewritten; citation per section.

This reference does not teach a new pattern — it fixes **the cost of each choice** and the symptom that indicates
which one to choose. It does not duplicate: architectural styles stay in `architectural-styles.md`,
characteristics and fitness functions in `architecture-characteristics.md`, DDD in `domain-modeling.md`.

## Diagnosis before scaling

| Question | Answer | What it means |
|---|---|---|
| Slow for **one** user? | **performance** problem | optimize critical path, index, cache |
| Fast with 1 user and slow under load? | **scalability** problem | partition, replicate, distribute |
| High time for one action? | **latency** problem | bring data/execution closer to the point of use |
| Low actions per second? | **throughput** problem | parallelism, batching, queue |

Source's operational definition: a service is scalable if performance grows **proportionally**
to the resources added; the goal is maximum throughput with acceptable latency
(`README.md#Performance vs scalability`, `README.md#Latency vs throughput`).

Practical corollary: adding resources without changing the architecture improves performance and does not scale — and
trading for scale without measuring latency produces high throughput with the user waiting.

## Availability in numbers

| Level | Unavailability/year | /month | /week | /day |
|---|---|---|---|---|
| 99,9% (3 nines) | 8h 45min 57s | 43m 49,7s | 10m 4,8s | 1m 26,4s |
| 99,99% (4 nines) | 52min 35,7s | 4m 23s | 1m 5s | 8,6s |

(`README.md#Availability in numbers`)

Composition:

- **In series**: `A_total = A(a) × A(b)`. Two components of 99,9% in series give **99,8%** — availability
  **drops**.
- **In parallel**: `A_total = 1 − (1 − A(a)) × (1 − A(b))`. Two of 99,9% in parallel give **99,9999%**
  (`README.md#Availability in parallel vs in sequence`).

A consequence that is almost always ignored: each synchronous dependency on the critical path **multiplies**
unavailability. Redundancy only pays off in parallel, with a failover that really switches.

## CAP: the choice that is not optional

In a distributed system you sustain two of the three guarantees — consistency (every read sees the
last write or an error), availability (every request answers, possibly outdated) and
partition tolerance (`README.md#CAP theorem`).

The network is not reliable, so partition tolerance is a premise: **the real decision is CP or AP**.

| Choice | Behavior during the partition | When it serves |
|---|---|---|
| **CP** | a partitioned node may time out | atomic read/write is a business requirement |
| **AP** | answers the most available version, possibly old | the system must keep running and accepts eventual consistency |

(`README.md#CAP theorem`)

## Consistency

| Pattern | After the write | Where it appears | Serves for |
|---|---|---|---|
| Weak | a read may or may not see it | memcached | tolerant real time (voice, game) |
| Eventual | a read sees it later (typically ms) | DNS, e-mail | high availability |
| Strong | a read sees it immediately (synchronous replica) | FS, RDBMS | needs a transaction |

(`README.md#Consistency patterns`)

Common mistake: calling "eventual" what is "weak". Eventual promises convergence; weak promises
nothing.

## Failover

| Pattern | How it works | Cost |
|---|---|---|
| **Active-passive** | heartbeat; the standby takes over when the active one goes silent | needs a warm standby so as not to pay the boot; data loss if the active one falls before replication |
| **Active-active** | both serve traffic and split the load | requires DNS with both IPs or an internal load balancer; write conflict |

(`README.md#Fail-over`)

Cold standby is not failover — it is restoration. The unavailability time is decided by the state of the
passive one (`#Fail-over`).

## Scaling the database

Order of increasing cost, from the least to the most invasive:

| Technique | Gain | Real cost |
|---|---|---|
| Master-slave replica (read) | distributes reads | outdated replica; critical read must go to the master |
| **Federation** (by function) | less traffic per database, more cache hit | application logic decides the database; join between databases becomes a `server link`; more hardware |
| **Sharding** (by data) | less traffic, smaller index, more cache hit | shard logic in the application; uneven distribution (power user in one shard); rebalancing requires consistent hashing; join between shards is complicated |
| **Denormalization** | fast reads | more expensive writes; a constraint to keep the copy in sync increases schema complexity |

(`README.md#Federation`, `README.md#Sharding`, `README.md#Denormalization`)

Fact that justifies denormalization: reads can outnumber writes by **100:1** or **1000:1**, and an
expensive join consumes disproportionate time (`#Denormalization`).

Rule: shard by a key that appears in **every** hot query. If the query needs to cross shards,
you bought latency and complexity and you will pay on both.

## Cache: choose by the failure, not by the speed

| Strategy | How it writes | Characteristic failure |
|---|---|---|
| **Cache-aside** | the app reads the cache, on a miss loads from the database and populates it | 3 trips per miss; stale data until the TTL; new node starts empty |
| **Write-through** | the cache writes synchronously to the store | a new node does not cache until the data is updated; it may write what will never be read |
| **Write-behind** | the cache confirms and writes asynchronously | data loss if the cache falls before the flush; more complex implementation |
| **Refresh-ahead** | the cache updates before expiring | a wrong prediction worsens performance; it requires invalidation coherent with the source |

(`README.md#Cache-aside`, `README.md#Write-through`, `README.md#Write-behind (write-back)`, `README.md#Refresh-ahead`)

Decision test: "if the cache falls now, what happens to the data?" If the answer is "I lose the
write", you chose write-behind and you must declare it.

## Asynchronism and back pressure

A queue decouples producer from consumer; it does **not** eliminate insufficient capacity: if the queue grows
without a limit, its size exceeds memory and the system degrades through swap/disk reads
(`README.md#Back pressure`).

Rules:

- A queue needs a limit and a policy for when it fills (reject, degrade, drop) — declared before the incident.
- Back pressure is a response to the slow consumer, not an increase in the queue.
- An idempotent consumer is a prerequisite: at-least-once delivery duplicates.

## Decision table

| Measured symptom | First attempt | If it is not enough |
|---|---|---|
| high p99, single user | index, N+1, cache-aside | read replica |
| high p99 under load | L7 balancer + horizontal scale | sharding |
| saturated write | batching / queue | write-behind with declared loss |
| database too large for 1 host | federation | sharding by hot key |
| read-dominant with an expensive join | cache + denormalization | materialized view / CQRS |
| a dependency takes the system down | redundancy in parallel | graceful degradation with local cache |

## Trade-off checklist

- [ ] Was the symptom classified (performance, scalability, latency, throughput)?
- [ ] Is the target availability written in nines, with the corresponding downtime accepted?
- [ ] Is it explicit whether the system is CP or AP during the partition?
- [ ] Was the availability multiplied for each synchronous dependency on the critical path?
- [ ] Does the chosen scale technique have its cost named (logic in the application, join, rebalancing)?
- [ ] Does the cache strategy declare what is lost when the cache falls?
- [ ] Does the queue have a limit and a discard policy?
- [ ] Is there a fitness function measuring what the decision promised?
