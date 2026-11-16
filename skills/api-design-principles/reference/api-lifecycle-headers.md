# API lifecycle headers (Sunset, Deprecation, traceparent)

> Reference loaded by `../SKILL.md` and by `reference/api-operations.md` §2 (lifecycle and deprecation window).
> Does not duplicate: Planned/Beta/Live/Deprecated/Retired states, compatibility gate in CI and semver stay in `reference/api-operations.md` §1-§5; tracing header propagation in release validation stays in §13 of the same file.
> What is here is the **wire mechanics** of the three headers: name, field format and semantics, always from the spec. Author inference marked as **[inference]**.

## 1. Three headers, three questions

| Header | Question it answers | Spec | Status | Value format |
| --- | --- | --- | --- | --- |
| `Deprecation` | when it stops being recommended | RFC 9745 | Standards Track, March/2025 | Structured Field Date: `@` + Unix seconds |
| `Sunset` | when it stops responding | RFC 8594 | Informational, May/2019 | HTTP-date |
| `traceparent` | in which trace/span this request is | W3C Trace Context | Recommendation / Level 2 | `version-trace-id-parent-id-trace-flags` |

Two confusions that are expensive in contract review:

- `Deprecation` and `Sunset` **do not use the same date format**. One is epoch with `@`, the other is textual HTTP-date.
- `traceparent` is not a lifecycle header; it is observability. It is here because it is what turns "we think nobody uses v1" into measurement (§7).

## 2. Sunset (RFC 8594)

Spec abstract: "This specification defines the Sunset HTTP response header field, which indicates that a URI is likely to become unresponsive at a specified point in the future. It also defines a sunset link relation type that allows linking to resources providing information about an upcoming resource or service sunset."

Semantics (§3):

- "The Sunset header field contains a single timestamp that advertises the point in time when the resource is expected to become unresponsive."
- "The Sunset value is an HTTP-date timestamp, as defined in Section 7.1.1.1 of [RFC7231], and SHOULD be a timestamp in the future."
- "It is safest to consider timestamps in the past mean the present time, meaning that the resource is expected to become unavailable at any time."
- ABNF: `Sunset = HTTP-date`.

Practical consequences:

- **`SHOULD` in the future, not `MUST`**: a past date is legal and must be read as "it may go down now". Do not treat a past date as an invalid header.
- **`likely` is not an SLA**: the RFC announces an expectation, it does not commit availability until the date nor guarantee a shutdown after it.
- **Informational**: it is a header specification, not a norm of obligation. A client that ignores `Sunset` violates nothing.
- Format: HTTP-date in the IMF-fixdate subset — fixed size, single zone, always `GMT` (RFC 7231 §7.1.1.1). Form: `Sunset: Sun, 06 Nov 1994 08:49:37 GMT`.
- The RFC describes **the header**, not the behavior after the date. Which status code the route returns after sunset (410, 404, redirect) is the provider's decision, which needs to be documented elsewhere. **[inference]**

## 3. Deprecation (RFC 9745)

Abstract: "The Deprecation HTTP response header field is used to signal to consumers of a resource (identified by a URI) that the resource will be or has been deprecated."

Semantics (§2.1), spec text:

- "describes the deprecation of the resource identified with the response it occurred within (see Section 6.4.2 of [HTTP])."
- "It conveys the deprecation date, which may be in the future (the resource in context will be deprecated at that date) or in the past (the resource in context was deprecated at that date)."
- "`Deprecation` is an Item Structured Header Field; its value MUST be a Date as per Section 3.3.7 of [RFC9651]."
- The document "uses 'Structured Field Values for HTTP' [RFC9651] to specify syntax and parsing of date values".

Example from the spec itself (resource deprecated on Friday, June 30, 2023, 23:59:59 UTC):

```http
Deprecation: @1688169599
```

Correct reading:

- `@` + seconds since epoch. It is **not** HTTP-date; `Deprecation: Fri, 30 Jun 2023 23:59:59 GMT` is wrong.
- Future date = advance notice. Past date = already deprecated.
- **Deprecation does not change behavior**: the resource keeps responding. The header only signals that migration must start.
- Comparison between the two specs: `Deprecation` answers "when it stops being recommended"; `Sunset` answers "when it stops responding". RFC 9745 dedicates §4 to `Sunset` precisely to mark the difference.
- How the two coexist: `Deprecation` <= `Sunset`. Deprecation after sunset is a contract contradiction. **[inference]**

## 4. Link relations: where the "why" and the "how to migrate" live

The two headers say **when**, not **what to do**. The complement is `Link`:

| Relation | Spec | Points to |
| --- | --- | --- |
| `deprecation` | RFC 9745 §3 | documentation of the deprecation and of how to deal with it |
| `sunset` | RFC 8594 | sunset policy of the resource or of the service |

- `Link` format comes from RFC 8288 (Web Linking): URI between `<>`, followed by `;`-separated parameters, with `rel` mandatory.
- RFC 9745 §3 is explicit about the purpose: to inform "where to find more information about deprecation of the context", and that holds "before the actual deprecation to make a deprecation policy discoverable or after deprecation when there may be documentation about the deprecation and how to manage it".
- Example of the combined form (structure per the two specs, illustrative values):

```http
Deprecation: @1688169599
Sunset: Sun, 06 Nov 1994 08:49:37 GMT
Link: <https://developer.example.com/deprecation>; rel="deprecation"; type="text/html"
```

- A header without a link is a notice without instructions: the consumer knows it is going to break and does not know what to do. Publishing the link is part of the notice, not decoration. **[inference]**

## 5. traceparent (W3C Trace Context)

The header has four fields, in this order: `version`, `trace-id`, `parent-id`, `trace-flags`. Spec ABNF (version 00):

```abnf
version-format = trace-id "-" parent-id "-" trace-flags
trace-id      = 32HEXDIGLC   ; 16 bytes. All zeroes forbidden
parent-id     = 16HEXDIGLC   ; 8 bytes. All zeroes forbidden
trace-flags   = 2HEXDIGLC    ; 8 bit flags
```

Rules the spec states:

- `version` is 1 byte in two hex characters; **`ff` is invalid**; the spec defines version `00`.
- `trace-id` and `parent-id` **all zero are prohibited** — they are not a "valid empty id".
- Hex is **lowercase** (`HEXDIGLC`). Uppercase is invalid.
- `trace-flags` has one bit in use: the `sampled`.
- Valid value, from the spec example: `00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`.
- Size: for version 00, 32 + 16 + 2 hex characters plus 3 hyphens = **55 characters**. **[inference]** arithmetic over the ABNF above, not a spec sentence — the size is fixed only in that version.
- Pass-through **must not** interpret the version: the spec instructs forwarding services "should not analyze the version" and "should expect that headers may have larger size limits in the future and only disallow prohibitively large headers". Parsing a future version as an error breaks the trace of whoever only forwards.

`tracestate` is the second header, which carries what does not fit in `traceparent`:

- the value is a `list` of comma-separated `list-members`; each member is `key=value`;
- "There can be a maximum of 32 `list-members`";
- when exceeded, "the right-most `list-member` should be removed";
- spaces and tabs around the member are ignored; empty members are allowed.

## 6. What to propagate between services

Normative Trace Context text, which separates the two cases:

- With a header received: "A vendor receiving a `traceparent` request header MUST send it to outgoing requests. It MAY mutate the value of this header before passing it to outgoing requests."
- And the chaining with `tracestate`: "If the value of the `traceparent` field wasn't changed before propagation, `tracestate` MUST NOT be modified as well" — whoever only forwards does not touch `tracestate`.
- Without a header received: "A vendor receiving a request without a `traceparent` header SHOULD generate `traceparent` headers for outbound requests, effectively starting a new trace." The spec notes that generating even without sampling is the expected behavior, to propagate the sampling decision downstream.

Operational rule (**[inference]** from the excerpts above):

1. `trace-id` identifies the whole trace: it is **never** regenerated mid-path. Regenerating it splits the tree in two.
2. `parent-id` identifies the caller's span. The span created in the service becomes the new `parent-id` on the next hop.
3. `trace-flags` propagates the sampling decision downstream; rewriting it to "sampled" without sampling produces half a trace.
4. `tracestate` only changes by whoever has an entry to add, and respecting the limit of 32.
5. A header does not cross a queue, topic or background job by itself: the context needs to be copied into the message envelope, otherwise the trace dies at the producer. **[inference]**

What this solves in an API contract: with `traceparent` propagated, "who still calls `/v1/attendees`" stops being a question for the product team and becomes a query in the trace collector. See also `api-operations.md` §13 (header propagation and release validation), which covers the rollout side.

## 7. How the three fit into the deprecation policy

Sequence, with the header that carries each step:

| Step | Header | What changes for the consumer |
| --- | --- | --- |
| announcement | `Deprecation: @<future>` + `Link rel="deprecation"` | nothing works differently; migration starts |
| measurement | `traceparent` + log per route/version | nothing; the provider finds out who still calls |
| final notice | `Sunset: <date>` | nothing works differently until the date |
| retirement | — (the window closed) | response changes or stops existing |

Decisions that need to be written down, not assumed:

- **In which responses the header appears**: only 2xx or all, including errors. Both choices are defensible; omission is not — a consumer that only sees the header on 200 will conclude it does not exist.
- **`Deprecation` on the first day of the window, `Sunset` when the date is already known.** Inverting the order takes away from the consumer the only actionable information (how much time it has).
- **Who monitors usage**: without `traceparent` propagated and a log per route, the decision to retire the version is a guess with a date. The compatibility gate of `api-operations.md` §5 protects the schema, it does not reveal consumers.
- **Sunset without a response plan**: the date arrives and the route stays up "to be safe". That teaches the consumer that the header means nothing. **[inference]**
- **Header on a deprecated version x header on a Live version**: only the version under deprecation carries `Deprecation`/`Sunset`; emitting it on Live turns notice into noise.

## 8. Checklist

- [ ] `Sunset` is HTTP-date IMF-fixdate, in GMT, with a date in the future or in the present (past date = "it may go down now").
- [ ] `Deprecation` is Structured Field Date: `@` + Unix seconds, never HTTP-date.
- [ ] `Deprecation` <= `Sunset`; the two never contradict each other.
- [ ] `Link` with `rel="deprecation"` (and `rel="sunset"` when there is a public policy) pointing to real doc.
- [ ] Written policy defines in which responses the headers appear.
- [ ] `traceparent` generated when absent and propagated when present, without regenerating `trace-id`.
- [ ] `tracestate` only modified by whoever adds an entry, respecting the limit of 32 members.
- [ ] Hex lowercase; `trace-id`/`parent-id` never all zero; version `ff` rejected.
- [ ] Trace context copied to the message envelope in an asynchronous flow.
- [ ] Usage of the deprecated version measured per route/version before fixing the `Sunset` date.

## Sources

- **RFC 8594 — The Sunset HTTP Header Field** (E. Wilde, May/2019, Informational): abstract, §3 semantics, `Sunset = HTTP-date`, `SHOULD` in the future, reading of a past date, link relation `sunset`. https://www.rfc-editor.org/rfc/rfc8594.html and https://www.rfc-editor.org/info/rfc8594/
- **RFC 9745 — The Deprecation HTTP Response Header Field** (S. Dalal, E. Wilde, March/2025, Standards Track): abstract, §2.1 (description, past/future date, "Item Structured Header Field; its value MUST be a Date as per Section 3.3.7 of [RFC9651]"), example `Deprecation: @1688169599`, §3 link relation `deprecation`, §4 `Sunset`. https://www.rfc-editor.org/rfc/rfc9745.html
- **RFC 7231 §7.1.1.1 — Date/Time Formats**: HTTP-date, IMF-fixdate as the preferred format, fixed size and single zone. https://www.rfc-editor.org/rfc/rfc7231.html (excerpt read at https://stackoverflow.com/questions/75235516/does-googles-http-server-return-a-header-with-the-wrong-date-time-format)
- **RFC 9651 — Structured Field Values for HTTP**: Date syntax used by RFC 9745 §3.3.7. https://www.rfc-editor.org/rfc/rfc9651.html (referenced by RFC 9745; not opened in full)
- **RFC 8288 — Web Linking**: `Link` format and `rel` parameter. https://www.rfc-editor.org/rfc/rfc8288.html (referenced; format also read at https://apiguide.dev/specifications/web-linking)
- **W3C Trace Context (Recommendation, 2021-11-23)**: `traceparent` fields, `version-format` ABNF, `trace-id`/`parent-id` 32/16 HEXDIGLC with zero prohibited, `ff` invalid, example `00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`, pass-through rule, "MUST send it to outgoing requests", `tracestate` unchanged rule, processing model without a header received. https://www.w3.org/TR/trace-context/
- **W3C Trace Context Level 2 (2024-03-28)**: `tracestate` as a list of at most 32 `list-members`, removal of the right-most, `traceparent` generation text for outbound. https://www.w3.org/TR/trace-context-2/
- **Trace Context — spec/30-processing-model.md (repo w3c/trace-context)**: step by step of the case "No traceparent Received". https://github.com/w3c/trace-context/blob/main/spec/30-processing-model.md
- **Trace Context — spec/20-http_request_header_format.md (repo w3c/trace-context)**: ABNF and `traceparent` fields. https://github.com/w3c/trace-context/blob/main/spec/20-http_request_header_format.md
- **Not confirmed**: Trace Context section number for `traceparent`/`tracestate` (I cited by section title, not by number); literal text asserting the total of 55 characters (the value was derived from the ABNF); status code behavior after `Sunset` (neither of the two RFCs fixes it).
