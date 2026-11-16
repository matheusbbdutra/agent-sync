# Integration boundary — where the model meets your system

## Provider adapter

The product owns the contract; the vendor owns an implementation of it.

```text
internal/ai/
  provider.go        // type Provider interface { Complete(ctx, Req) (Resp, error) }
  openai/            // vendor A
  anthropic/         // vendor B
  local/             // fallback option
```

Rules:

- **No vendor type crosses the boundary.** Requests and responses are your structs. If a status code, header or SDK error type leaks into domain code, swapping providers becomes a refactor of the product.
- **Model name is configuration, not constant.** Per-task setting, with the measurement that justified it recorded next to it (p.337).
- **Secrets live in the environment of the adapter only** — never in the caller, never in logs, never in the prompt.
- **Timeouts and cancellation are part of the interface.** A `ctx` that ignores deadline turns a slow vendor into an outage of your service.

## Order of components

Add in this order, and only when the pain appears (p.470):

1. `query → model → response`
2. enhance context (retrieval)
3. guardrails
4. model router + gateway
5. caching for latency/cost
6. multi-step / agentic logic and write actions

A pipeline built before observability exists cannot be debugged when it fails.

**Router** is a cheap, fast classifier that picks the model per request. It earns its place when you have more than one model tier and the cost difference pays for the classifier. **Gateway** earns its place when several products share the vendor: centralized auth, rate limit, retries, quotas, audit (p.470).

## Streaming

- Stream when **time-to-first-token is what the user feels**; batch when throughput matters more.
- Streaming and schema-validated output conflict: you cannot validate a partial JSON. Choose per endpoint — stream prose, buffer structured output.
- Emit a terminal event (`done` / `error`) so the client never renders a truncated answer as complete.
- A stream that dies mid-way is a failure the UI must show; silence looks like success.

## Retry and idempotency

LLM calls are non-deterministic: a retry can return a different answer.

| Failure | Retry? | Note |
|---|---|---|
| 429 / rate limit | yes, with backoff + jitter | respect `Retry-After` |
| 5xx / timeout | yes, bounded (2-3) | do not retry a 4xx that is a request bug |
| invalid schema | yes, bounded, with the validation error in the retry prompt | 1 retry usually fixes; 3 is the ceiling |
| refusal / safety block | no | surface it; retrying to "get past" a block is abuse |
| truncated (`finish_reason=length`) | no (retry with bigger budget) | count it, do not silently accept |

- Write actions need an **idempotency key** owned by your system, because a retried "create ticket" must not create two.
- Log the attempt count per request; retry storms are invisible until you count them.

## Failure modes

Write the user-visible behavior for each before shipping:

| Situation | User sees |
|---|---|
| provider down | degraded path or explicit "unavailable", never a blank screen |
| timeout | partial result if streaming, otherwise retry once then error |
| invalid output after retries | fallback template + telemetry event, not raw garbage |
| guardrail triggered | reason the user can act on ("pedido fora do escopo"), not "error" |
| cost budget exceeded | request rejected before the call, not after |

## Telemetry

Per request, log: endpoint, model + version, sampling settings, prompt template version, input/output tokens, cache hit, latency TTFT/TPOT, finish reason, guardrail decisions, and the final output (p.490). Without this, a regression cannot be attributed to prompt, retrieval or model swap.

Metrics come from failure modes, not from what the dashboard has room for (p.487).

## Latency measurement

- `total ≈ TTFT + TPOT × output_tokens` (p.433).
- Report p50/p90/p99 — **never the average** (p.434).
- Count **goodput**: requests that met the SLO (p.435). Batching that doubles throughput while pushing p99 out of SLO is a regression with good press.

## Cache

- Cache **invariant segments**: system prompt, base document, fixed context. Measure hit rate and memory; a 1.000-token system prompt in 1M calls/day is ~1 billion repeated tokens (p.463-464).
- Answer cache must be keyed by everything that can change the answer — user, time, model, template. A cached answer leaking across users is a security incident, not a performance bug.
- Semantic cache needs the same discipline: threshold, invalidation and a measured false-hit rate, otherwise it silently returns the wrong answer fast.

## Rollout

1. Shadow: run the new model/prompt on real input, discard output, compare.
2. Canary: small traffic slice with the gate from `llm-integration` checked per slice.
3. Full, with the old path reachable by flag for one release.
4. Delete the flag — a rollout mechanism left behind becomes permanent branching.
