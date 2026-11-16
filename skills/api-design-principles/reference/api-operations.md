# API Operations (gateway, traffic, compatibility, and observability)

> Load when the API already exists and the question is how to operate, evolve, or protect it: versioning, deprecation, contract testing, ingress, rate limiting, telemetry.
> Contract mechanics (resource name, pagination, rate limit header, idempotency key) are already in `reference/rest-best-practices.md`; here is the **policy**.
> Per-call resilience (retry, timeout, circuit breaker) has its own file: `error-handling-patterns/reference/resilience.md`.
> Citations use the `p.N` marker from the source digest.

## 1. Semver and the version contract

- Apply Major.Minor.Patch with operational semantics: major breaks, minor is a compatible addition, patch is a fix (p.50).
- Raising the major is an **active decision by the consumer**, not the producer — the consumer chooses when to migrate (p.50).
- Minor and patch can never require client action; if they do, they are a disguised major (p.50).
- Expose only the major to the consumer: in the URL (`/v1/attendees`) or in a header routed at the ingress (`Version: v1`) (p.160).
- Choose URL **or** header and keep it; switching between the two forces the consumer to support two mechanisms (p.160).
- A version is not a feature flag: the old version needs a declared exit path (§2) (p.159).

Change type to version number (p.50):

| Change | Number | Action required from the consumer | Source |
| --- | --- | --- | --- |
| new field/optional field, new endpoint | minor | none | p.50 |
| defect fix without changing the contract | patch | none | p.50 |
| removal, rename, type change, semantics change | major | active decision to migrate | p.50 |

## 2. Lifecycle and deprecation window

- Keep each version in an explicit state: Planned, Beta, Live, Deprecated, Retired (p.159).
- Beta can break without notice; Live cannot (p.159).
- A major change requires Live + Deprecated coexistence for weeks or months, with real time for the consumer to migrate (p.159).
- A minor change stays Deprecated only until the new version is validated in production (p.159).
- Retire only after closing the migration window; removing earlier is the root cause of consumer incidents (p.159).
- Answering "can I remove this field?" requires answering first: what state the version is in and how much time is left until the end of the window (p.159).
- An eternal Beta is not a contract: either it is promoted to Live, or it becomes Planned again (p.159).

State is what allows breaking or not (p.159):

| State | Can it break without notice? | Exit | Source |
| --- | --- | --- | --- |
| Planned | yes (there is no consumer) | Beta or Live | p.159 |
| Beta | yes | Live | p.159 |
| Live | no | Deprecated when there is a major | p.159 |
| Deprecated | no; window of weeks or months | Retired | p.159 |
| Retired | there is no contract anymore | — | p.159 |

## 3. Binary schema compatibility (protobuf)

- In `.proto`, only add a field, service, or method; never remove, rename, or change type (p.52).
- Never renumber a field number: the number is the field's identity on the wire (p.52).
- Every new field must be optional; a new required field breaks backward compatibility (p.52).
- The compiler does not catch this error: the corruption is silent and shows up at runtime (p.52).
- Versioning a proto follows the schema's own versioning rule, not the version of the corresponding REST API (§4) (p.52, p.58).

## 4. Do not derive one specification from the other

- Keep the REST spec (OAS) and the RPC schema (proto) as independent artifacts (p.58).
- Generating one from the other couples the two domains: a new field in the OAS reorders and renumbers the proto fields and breaks compatibility (p.56).
- If both exist, each one is versioned by its own rule (p.58).
- The proposal to "get both APIs for free" via `openapi2proto` is the trigger for this anti-pattern, not an optimization (p.56).

## 5. Compatibility gate in CI

- Run a spec diff against the version published in the pipeline and fail the build when the result is "broke backward compatibility" (p.51).
- The build must prevent the breaking change from entering without a conscious override (p.160).
- Treat this verification as an architecture fitness function: a unit/integration test for the architectural decision (p.227).
- An override is an explicit, recorded act; "someone reviewed the spec" is not a gate (p.160).
- The gate covers what the schema describes; behavior still needs a contract test (§6) (p.47).

Gate steps, in this order (p.51, p.160, p.227):

1. generate the spec from the code or validate the versioned spec (p.51).
2. run the diff against the spec of the last published version (p.51).
3. if the result is breaking, fail the build (p.160).
4. an override requires an explicit record with a reason (p.160).
5. treat the verification as an architecture test: it protects the decision, not the code (p.227).

## 6. Contract testing: a contract is not a schema

- OpenAPI describes form, not semantics: the spec does not model the expected behavior under different conditions (p.47).
- Contract testing is not the same as saying that the API conforms to a schema (p.68). The two tests are necessary and different.
- An interaction contract must have a sample request and response, and from those samples come tests and a stub server (p.68).
- Methodology by audience:
  - External API, broad audience: **producer contracts** — no individual consumer can ask for a change that affects the whole API (p.71).
  - Internal API between teams: **consumer-driven contracts (CDC)** — the consumer submits the contract and the producer accepts or rejects it (p.73).
- Starting with producer contracts and evolving to CDC is acceptable and explicitly recommended (p.73).
- A one-sided contract kept without a real tested consumer is the failure mode that CDC avoids (p.71, p.73).
- The choice of methodology is an architecture decision (Type 1): record it in an ADR (p.73; see `adr-guidelines.md`).

## 7. Ingress: proxy, load balancer, or gateway

- Choose the simplest solution that covers the known requirements, with attention to the immediate future (p.88).
- Reverse proxy/load balancer covers: one backend, TLS, multiple backends, and service discovery (p.88).
- These are API gateway territory: authorization, retry, rate limiting, logging/tracing, and circuit breaking (p.88).
- Over-provisioning (an enterprise gateway for two routes) and under-provisioning (a homemade proxy accumulating ad hoc auth, retry, and rate limit) are the same mistake in opposite directions (p.88).
- When reviewing the edge, list per requirement which of these functions is really necessary **today**; the rest does not go in (p.88).

Function matrix by component type (p.88):

| Function | Reverse proxy / LB | API gateway | Source |
| --- | --- | --- | --- |
| TLS | yes | yes | p.88 |
| one backend | yes | yes | p.88 |
| multiple backends | yes | yes | p.88 |
| service discovery | yes | yes | p.88 |
| authorization | no | yes | p.88 |
| retry logic | no | yes | p.88 |
| rate limiting | no | yes | p.88 |
| circuit breaking | no | yes | p.88 |
| logging / tracing | no | yes | p.88 |

- Use the matrix as a requirement checklist: each row marked "yes" on the gateway needs a project justification, not a vendor catalog (p.88).

## 8. Rate limiting and load shedding are distinct policies

- **Rate limiting** looks at the request's property: user, client, IP (p.189).
- **Load shedding** looks at the system state: database at its limit, no worker available (p.189).
- Defining one and calling it the other produces undefined behavior under load (p.189).
- Many applications, web servers, and gateways do not implement either of the two by default, and the corresponding failure mode stays undefined (p.189).
- Declare explicitly whether the component **fails open** (lets traffic through) or **closed** (blocks) (p.159).
- For a financial API, the book records fail closed as the expected default (p.159).
- A security component that fails open silently is one of the most expensive failure modes: it protects when tested and does not protect when it matters (p.110).

Symptom to policy (p.189):

| Symptom | Correct policy | Wrong policy | Source |
| --- | --- | --- | --- |
| one client consumes 80% of the quota | rate limiting by origin | load shedding | p.189 |
| database at its limit, no workers | load shedding | quota per user | p.189 |
| loop between two internal services | internal limit + error monitoring | limit only at the edge | p.189 |
| protection component failure | declared fail closed | undefined behavior | p.159 |

## 9. Choose the rate limiting algorithm by traffic pattern

- Name the algorithm before configuring: fixed window, sliding window, token bucket, or leaky bucket (p.190).
- **Fixed window**: quota per period; simple and allows a burst at the window boundary (p.190).
- **Sliding window**: moving window; smooths the boundary at the cost of state (p.190).
- **Token bucket**: allows a burst with replenishment at a rate (p.190).
- **Leaky bucket**: fixed output rate, absorbing the burst at the input (p.190).
- Selection rule: traffic with a burst under fixed window passes double the limit at the boundary — the limit stops being a limit (p.190).
- The policy must be justifiable to the consumer: named algorithm, declared quota and window (p.190).

Property that decides the choice (p.190):

| Traffic property | Algorithm | Accepted side effect | Source |
| --- | --- | --- | --- |
| simple quota per period, uniform traffic | fixed window | burst at the window boundary | p.190 |
| quota per period with bursty traffic | sliding window | more state to maintain | p.190 |
| burst allowed, with replenishment by rate | token bucket | momentary peak above the average rate | p.190 |
| output always at the same rate | leaky bucket | queue at the input | p.190 |

## 10. Limit and monitor the internal call as well

- Apply rate limiting and error monitoring to service-to-service calls, not only at the edge (p.189).
- The named risk is "friendly fire DoS": the systems themselves, in a call loop, take the platform down from the inside out (p.189).
- Circular dependency between services is the typical cause; the internal limit is what prevents the loop from consuming all capacity (p.189).
- A protected edge with an unprotected inside protects only the first hop (p.189).

## 11. Resilience in the common layer, not per library

- Concentrate retries, timeouts, circuit breakers, bulkheads, and fallbacks in the gateway/service mesh (p.129).
- Implementing per library in code leads to inconsistent behavior, especially across languages and platforms (p.129).
- Identical semantics across all services is the main gain; without it, divergent timeouts amplify cascading failure (p.129).
- Circuit breaker serves to allow recovery: failing fast frees the service to come back (p.129).
- The fallback is a product decision, made before the incident: the book's example is rendering the full event agenda instead of the personal agenda (p.129).
- Retry, timeout, backoff, and fallback details: `error-handling-patterns/reference/resilience.md`.

## 12. Observability: RED/golden signals with error class

- Measure rate, errors, and duration (RED) and cover the Four Golden Signals (p.170).
- Segment errors by class: 5xx is service failure, 4xx is client behavior (p.170).
- A series of 403/401 is a sign of attack or leaked credential, not client noise (p.171).
- An alert without window context generates a false positive: a traffic drop on a holiday or weekend is not an incident (p.171).
- Adjust the alert with activity context (active logins, business day) before turning on the pager (p.171).
- A metric without a segment hides the guilty stage: an error aggregate does not say whether the problem is the edge, the service, or the client (p.170, p.171).

Reading the signal (p.170, p.171):

| Signal | Interpretation | Action | Source |
| --- | --- | --- | --- |
| 5xx increase | service failure | investigate service and dependency | p.170 |
| 4xx increase | client behavior | document the contract, check the SDK | p.170 |
| series of 401/403 | attack or leaked credential | review authz and rotation | p.171 |
| traffic drop | possible holiday/weekend | only alert with activity context | p.171 |

## 13. Header propagation and release validation

- Every service that ends one request and creates another must copy the tracing/observability headers to the downstream request (p.172).
- Without that, the trace breaks at the first hop (p.172).
- There must be an explicit policy on forwarding authentication headers or not: forwarding allows one service to impersonate another service or a user (p.172).
- In canary/release validation, the test client sends `Cache-Control: no-cache, no-store` (p.172).
- Without a cache bypass, the rollout looks healthy because the response came from the cache (p.172).
- Header policy is a recorded decision, not an implementation detail (p.172).

Release validation, in order (p.172):

1. the test client sends `Cache-Control: no-cache, no-store`;
2. the test request trace appears complete in the collector;
3. the auth headers going down the stream comply with the written policy;
4. only after that does the rollout proceed.

## 14. Anti-patterns

- **Minor that requires client action**: a broken version with a compatible number (p.50).
- **Removing a version without a window**: the consumer finds out in production (p.159).
- **Renumbering a field in the proto**: silent corruption at runtime (p.52).
- **Spec generated from spec**: one field in the OAS renumbers the proto (p.56).
- **Green spec, divergent behavior**: the schema validates, the semantics are untested (p.47, p.68).
- **CDC for an external audience**: no consumer represents the whole API (p.71).
- **Gateway by fashion**: enterprise functions without a requirement (p.88).
- **Rate limit as load shedding**: a per-request policy used to contain saturation (p.189).
- **Silent fail open**: protection that disappears exactly at the failure (p.110, p.159).
- **Auth header forwarded without policy**: impersonation between services (p.172).
- **Alert without a window**: pager firing on a holiday (p.171).

## 15. Operations checklist

- [ ] Each version has a declared lifecycle state (p.159).
- [ ] Major exposed to the consumer by URL or header, with a single mechanism (p.160).
- [ ] Deprecation window defined for the next major change (p.159).
- [ ] Spec diff in CI fails the build on a breaking change (p.51, p.160).
- [ ] Contract with examples, tested; schema and contract are distinct tests (p.68).
- [ ] Contract methodology chosen by audience (producer x CDC) (p.71, p.73).
- [ ] Ingress is the simplest that meets the known requirements (p.88).
- [ ] Rate limiting by origin and load shedding by saturation exist as separate policies (p.189).
- [ ] Fail open/closed is declared for each edge component (p.159).
- [ ] Rate limiting algorithm named and justified (p.190).
- [ ] Internal call has a limit and error monitoring (p.189).
- [ ] Resilience lives in the common layer, with a defined product fallback (p.129).
- [ ] Alerts use RED/golden signals with error class and window (p.170, p.171).
- [ ] Tracing headers propagated; auth header policy written (p.172).
- [ ] Release smoke sends `Cache-Control: no-cache, no-store` (p.172).

## Sources

- **Mastering API Architecture — Design, Operate, and Evolve API-Based Systems** (James Gough, Daniel Bryant, Matthew Auburn), ch. 1 "Design, Build, and Specify APIs" (p.47, p.50, p.51, p.52, p.56, p.58), ch. 2 "Testing APIs" (p.68, p.71, p.73), ch. 3 "API Gateways: Ingress Traffic Management" (p.88, p.110, p.116), ch. 4 "Service Mesh: Service-to-Service Traffic Management" (p.129), ch. 5 "Deploying and Releasing APIs" (p.159, p.160, p.170, p.171, p.172), ch. 6 "Operational Security" (p.189, p.190), ch. 8 "Redesigning Applications to API-Driven Architectures" (p.227).
