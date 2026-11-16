# Resilience (retry, timeout, circuit breaker and degradation)

> Policy, not implementation. Circuit breaker, backoff and fallback code already exists in `implementation-playbook.md` (Patterns 1-3); what stays here are the decisions the code has to respect.
> Where resilience lives (gateway/mesh x code) is in `api-design-principles/reference/api-operations.md` §11.
> Citations use the `p.N` marker from the source digest.

## 1. Assumptions: treat the fallacies of distribution as design facts

- The book groups the eight fallacies of distributed computing on p.134 (Mastering API Architecture). The actionable rule is not to recite them, but to convert each one into a testable premise (p.134).
- Latency: measure average and p95/p99 before approving a flow with a remote call; 10 calls x 100 ms add 1 s to the flow (Fundamentals, p.137).
- Tail latency, not average, defines the viability of the response requirement (p.137).
- Bandwidth: a large payload on a frequent call is disguised coupling — 2.000 req/s of a 500 kb contract becomes 1 Gb/s (p.138).
- Topology: start synchronous; asynchrony enters only with a concrete requirement (scale mismatch, consumer fault tolerance, peak buffer) (p.277).
- Asynchrony brings its own cost: data synchronization, deadlock, race condition and hard debugging (p.277).
- Consequence: "the network is fast" and "the service responds" are not valid premises; each one needs a number measured in your own environment (p.137).

## 2. Retry: only when the operation is safe to repeat

- Before configuring retry, decide the 5xx semantics per operation: was the effect applied or not? (Mastering API, p.45)
- The book's case is the test: in a payment system, does a 500 mean the payment went through? Without a documented answer, blind retry duplicates the effect (p.45).
- A non-idempotent operation without an idempotency key gets no automatic retry; the retry becomes the client's decision with an idempotency key (p.45; mechanics in `api-design-principles/reference/rest-best-practices.md`).
- An idempotent operation (read, PUT, DELETE) tolerates repetition, but automatic retry stays conditional on the documented 5xx semantics (p.45).
- A 4xx error is not retryable: repeating client behavior only multiplies invalid calls (p.170).
- Retry at the edge and in the mesh, not in each service: per-language behavior diverges and amplifies cascading failure (p.129).
- Retry without an upper bound on attempts turns a dependency incident into a platform incident (p.129).

Decision table per response (p.45, p.170):

| Response | Idempotent operation | Non-idempotent operation | Source |
| --- | --- | --- | --- |
| 5xx with effect not applied, documented | retry with limit | retry with idempotency key | p.45 |
| 5xx with effect applied, documented | do not retry; treat as partial success | do not retry | p.45 |
| 4xx | do not retry | do not retry | p.170 |
| timeout | retry with limit (uncertain effect) | do not retry; reconcile state | p.45 |

## 3. Timeout: flow budget, not an isolated number

- A timeout is a budget: multiply the p95 latency of each hop by the number of chained calls in the flow before choosing the value (p.137).
- The caller's timeout must not exceed the remaining time of the flow; otherwise whoever waits has already given up when the response arrives (derived from p.137).
- Chained hops with a uniform timeout add up; defining it per hop without looking at the total reproduces the calculation of p.137 backwards.
- A timeout larger than the client's is useless: the response reaches no one (derived from p.137).
- A timeout does not replace a circuit breaker: one protects the caller from waiting, the other protects the callee from load (§4) (p.129).

Derive the timeout in this order (p.137):

1. list the hops of the critical flow;
2. measure p95 (and p99) of each hop in the real environment;
3. add the p95s to get the total budget of the flow;
4. distribute the budget per hop, with slack for the slowest hop;
5. reject the design if the sum blows the response requirement of the user.

## 4. Circuit breaker: open to allow recovery

- Stated goal: open the circuit to fail fast and allow the service to recover (Mastering API, p.129).
- Trip on repeated failure of a critical call, not on an isolated error (p.129).
- While open, the response is degraded by product decision (§6), never a generic error when a cheap alternative existed (p.129).
- Do not combine a circuit breaker with retry without a limit: infinite retry prevents the recovery the circuit tries to guarantee (p.129).
- Implement it in the shared layer (gateway/mesh) to have identical semantics in every service (p.129).
- Reference code for the mechanism (closed/open/half-open states) already exists in `implementation-playbook.md`; do not reimplement it per service (p.129).

States and what each one requires (mechanism in `implementation-playbook.md`, Pattern 1):

| State | Behavior | What has to be defined | Source |
| --- | --- | --- | --- |
| closed | calls the service normally | failure threshold | p.129 |
| open | fails fast, without calling | degraded response per scenario | p.129 |
| half-open | tests recovery | number of successes to close | p.129 |

- The transition needs an objective criterion (failure threshold, number of successes); a circuit without a criterion opens and closes on noise (p.129).

## 5. Cascading failure: the three vectors

- **Amplified retry**: N callers retrying against a slow dependency multiply load by N (p.129).
- **Divergent timeout**: services with different timeouts keep the dependency under load for longer than necessary (p.129).
- **Internal call without a limit**: a loop between services takes the platform down from the inside out — "friendly fire DoS" (p.189).
- Countermeasure for the three: limit and error monitoring on internal calls too, not only at the edge (p.189).
- Absence of protection is not neutral: many applications and gateways do not implement a limit by default and the failure mode stays undefined (p.189).

## 6. Fallback and degradation: product decision, made before the peak

- Fallback is a product decision: the book's example is rendering the full agenda of the event instead of the personal agenda (p.129).
- Write the degraded response per scenario (dependency down, data unavailable, quota exceeded) before the incident, not during it (p.129).
- Declare whether the component fails open (releases) or closed (blocks); a financial API tends to fail closed (p.159).
- A security component that fails open silently protects in the test and does not protect on failure (p.110).
- Fallback also needs a limit: applying the same internal limit principle of p.189 prevents degradation from becoming the second incident (derived from p.189).
- Degradation has to be observable as a state; if the fallback does not appear in the metric, the team does not know it is in it (§8) (p.170).

## 7. Application with a model: failure, fallback and degradation

- A model gateway/router exists to degrade: route to an alternative model, retry after a short wait or handle the failure with elegance (AI Engineering, p.479).
- The router must be fast and cheap; a router as expensive as the model is not a degradation layer (p.477).
- Add the gateway layer when the pain appears (provider rate limit, multiplication of models), not on the first day (p.470).
- Enumerate the failure modes before choosing the degradation: invalid tool, invalid parameter, wrong value, goal not reached, reflection error (the agent declares it is done without being done), absence of a tool for the domain (p.318, p.319).
- Each failure mode calls for a different response: retry solves a transient failure, it does not solve a goal not reached (p.318).
- Log every tool call with input and output; without that you do not know which failure mode dominated (p.320).
- Do not treat "it worked" as a metric: measure the frequency of each failure mode (p.318).

### 7.1 Guardrail and validation degradation

- Keep guardrails on the input **and** on the output even in degraded mode: removing both widens the risk at exactly the worst time (p.271).
- Track violation rate together with false refusal rate: guardrail degradation moves both numbers, and only one of them shows on the dashboard (p.268).
- A guardrail that refuses everything has violation rate zero and a useless system — a single metric hides that state (p.268).
- The pair of numbers only makes sense read together: violation rate falls and false refusal rises when the guardrail gets more aggressive (p.268).
- Guardrail degradation needs a deadline: turning it off without a return date turns an exception into permanent configuration (derived from p.271).

## 8. Measure degradation, not only failure

- Design the metrics from the failure modes you want to catch, not from what the tool exposes by default (p.487).
- Log configuration, final prompt, output and tool calls with IDs and reconstruct the trace per request with time and cost per step (p.490).
- A silent change of model, of provider or of template is a failure without an error: without a configuration log it does not appear (p.492).
- Report latency per percentile; the average hides the tail that defines the experience (p.434).
- Count goodput (requests that meet the SLO) instead of raw throughput; "we doubled the throughput" with the SLO violated is not an improvement (p.435).
- For a model, total latency = TTFT + TPOT x output tokens; measure the two parts separately (p.433).
- On the API side, segment the error by class (5xx service, 4xx client, a series of 401/403 attack) and tune the alert per activity window (Mastering API, p.170, p.171).

- Track MTTD (time to detect), MTTR (time to recover) and CFR (customer failure rate): these are the numbers that say whether the degradation worked (p.486).
- High MTTD with low MTTR means a detection problem, not a correction problem (p.486, p.487).

## 9. Anti-patterns

- **Blind retry on a non-idempotent operation**: duplicated effect (payment, transfer) (p.45).
- **Retry without a limit**: prevents the recovery the circuit breaker would try (p.129).
- **Uniform timeout per hop**: the sum of the hops exceeds the flow budget (p.137).
- **Circuit breaker per service**: semantics diverging across languages (p.129).
- **Fallback decided in the incident**: generic error where there was a cheap alternative (p.129).
- **Silent fail open**: protection that disappears on failure (p.110, p.159).
- **Degradation without a metric**: nobody knows the system is in degraded mode (p.170, p.487).
- **Average latency**: long tail invisible (p.434).
- **Throughput without SLO**: a big number with a worse experience (p.435).
- **"It worked" as a test**: an unmeasured failure mode keeps happening (p.318).
- **Asynchrony by default**: cost of deadlock, race and debugging with no requirement that pays for it (p.277).

## 10. Checklist

- [ ] The 5xx semantics per operation are documented (effect applied or not) (p.45).
- [ ] Every operation with retry is idempotent or has an idempotency key (p.45).
- [ ] Retry has an attempt limit and lives in the shared layer (p.129).
- [ ] The timeout of each hop respects the total budget of the flow (p.137).
- [ ] The circuit breaker opens on repeated failure and has a defined degraded response (p.129).
- [ ] The internal call has a limit and error monitoring (p.189).
- [ ] Fail open/fail closed is declared per component (p.159).
- [ ] The fallback is a written decision, with its own quota and observable state (p.129, p.189).
- [ ] Failure modes enumerated and measured by frequency (p.318).
- [ ] Latency reported per percentile and goodput against the SLO (p.434, p.435).
- [ ] Configuration and trace logged enough to reconstruct the request (p.490, p.492).

## Sources

- **Mastering API Architecture — Design, Operate, and Evolve API-Based Systems** (James Gough, Daniel Bryant, Matthew Auburn), ch. 1 "Error Handling" (p.45), ch. 3 "API Gateways" (p.110), ch. 4 "Reliability" and "The 8 Fallacies of Distributed Computing" (p.129, p.134), ch. 5 "Deploying and Releasing APIs" (p.159, p.170, p.171), ch. 6 "Operational Security" (p.189, p.190).
- **Fundamentals of Software Architecture — An Engineering Approach** (Mark Richards, Neal Ford), ch. 9 "Foundations" (p.137), ch. 17 "Microservices" (p.277).
- **AI Engineering — Building Applications with Foundation Models** (Chip Huyen), ch. 6 "Agent Failure Modes and Evaluation" (p.318, p.319, p.320), ch. 9 "Inference Performance Metrics" (p.433, p.434, p.435), ch. 10 "AI Engineering Architecture" (p.470, p.477, p.479) and "Monitoring and Observability" (p.487, p.490, p.492).
