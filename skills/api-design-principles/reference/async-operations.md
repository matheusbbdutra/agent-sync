# Asynchronous operations and optimistic concurrency

Source: Microsoft `api-guidelines`, `azure/Guidelines.md` (CC BY 4.0). Rewritten; citation by section.

Does not duplicate: pagination, generic versioning, idempotency and cache stay in
`rest-best-practices.md`; semver, deprecation window and `Deprecation`/`Sunset` stay in
`api-operations.md` and `api-lifecycle-headers.md`. Here only what **does not exist** in the skill today goes in:
`202 Accepted`, operation monitor, retry repeatability key and concurrency by `ETag`.

## When an operation becomes asynchronous

Declared cutoff rule: implement it as a long-running operation when the **p99 exceeds 1 second** and the
client needs to follow the progress (`azure/Guidelines.md#Long-Running Operations & Jobs`).

Consequences:

- **Never** make `PATCH` a long-running operation; if you need asynchronous update semantics,
  use the `POST` action (`#Long-Running Operations & Jobs`).
- Validate as much as possible **at the opening** of the operation, so the client receives the error early instead of
  finding it out in the polling (`#Long-Running Operations & Jobs`).
- A completed synchronous action returns `200-OK` (`#Performing an Action`).
- Do not use "action" for what is standard CRUD: create, read, update and delete have their own verb
  (`#Performing an Action`).

## Long-running operation contract

```http
POST /v1/exports            -> 202 Accepted
operation-location: https://api.example.com/v1/operations/8f3a
```

- `202` + `operation-location` header with the **absolute** URL of the status monitor
  (`#Long-Running Operations & Jobs`).
- The monitor is queried with `GET`, and its response carries the additional values the client
  needs to continue (`#Long-Running Operations & Jobs`).
- The client must never need to guess the state: either the operation is in the monitor, or the resulting
  resource already exists and is addressable.

Design checklist:

- [ ] The opening response tells **where** to follow it, not "try again later".
- [ ] Monitor states are finite and documented (running, completed, failed, canceled).
- [ ] A late failure returns a diagnostic error in the monitor **and** in the resource, not just a generic 500.
- [ ] A client that loses the operation URL can find it again by resource/idempotency key.
- [ ] Monitor lifetime declared (expiry does not take the client by surprise).

## Retry and repeatability key

For an action that needs to be idempotent under retry, the contract is by header
(`#Repeatability of requests`):

```http
Repeatability-Request-ID: 6f0d3c2a-...
Repeatability-First-Sent: Tue, 18 Nov 2026 10:00:00 GMT
```

- The repeatability window **MUST** be at least **5 minutes** (`#Repeatability of requests`).
- The repeatability key is different from a gateway `Idempotency-Key`: here the semantics is "this request
  is the repetition of that one", with a first-sent timestamp.
- Without a declared window, the client invents a timeout and the server treats it as a new operation — the
  duplicate effect shows up precisely on retry.

## Optimistic concurrency with `ETag`

- Honor every precondition received; if you cannot verify it, return the precondition error
  (`#Conditional Requests`).
- `If-Match` protects update and deletion from overwriting someone else's change; `If-None-Match` serves
  conditional reads and creation (`#Conditional Requests`).
- Each representation has its `ETag`: if the same resource is served with different encodings, the
  values **must not** be equal (`#Conditional Requests`).
- `ETag` is opaque to the client: do not carry a semantic version in it nor depend on its format.

Minimum flow:

```text
GET   /items/42          -> 200, ETag: "a1b2"
PUT   /items/42          If-Match: "a1b2"
                          -> 200 if it matched; 412 if the resource changed
```

Golden rule: 412 is a legitimate, expected response, not a server error. Whoever treats 412 as a bug
is, in practice, doing last-write-wins without declaring it.

## Paginated collections as a contract

- The listing response is an **object** with an array at the top, not a raw array — that way there is room for
  `nextLink` and metadata without breaking the contract (`#Collections`).
- `nextLink` is an absolute URL already carrying the parameters required by the API; on the last page the field **does not
  appear** (`#Collections`).
- Document explicitly whether items can be **skipped or duplicated** between pages; if the
  collection does not guarantee a snapshot, say so (`#Collections`).
- Each item carries `id` and, when there is optimistic concurrency, `etag`, so the client can
  change it without an extra GET (`#Collections`).

## Versioning as a precondition

If the API adopts a mandatory per-request version, the verifiable pattern is: an `api-version`
parameter required on every operation, values in `YYYY-MM-DD` with a `-preview` suffix for preview, and a
specific error when it is missing or not supported (`#API Versioning`).

The value for this repo is the error format: an error code of its own
(`MissingApiVersionParameter`, `UnsupportedApiVersionValue`) instead of a generic 400 — the client
finds out what to do without opening a ticket.

## Anti-patterns

| Anti-pattern | Effect |
|---|---|
| 202 without `operation-location` | client polls the wrong endpoint |
| `PATCH` as a long-running operation | partial update semantics broken |
| Retry with a new key on every attempt | duplication guaranteed |
| `ETag` derived from `updated_at` | collision and false 412 |
| relative `nextLink` | client behind a proxy does not follow it |
| Last page with an empty `nextLink` | polling loop |
