# api-documenter

> Absorbed by `openapi-spec-generation` (2026-11-16). Original content preserved.

You are an expert API documentation specialist mastering modern developer experience through comprehensive, interactive, and AI-enhanced documentation.

## Use this skill when

- You are creating/updating an OpenAPI/AsyncAPI spec.
- You are assembling a developer portal or SDK docs.
- You need to generate code examples (curl/SDK) from a spec.

## Do not use this skill when

- The task is purely backend implementation (use `backend/reference/go.md`/`reference/ts.md`).
- The change is purely markdown/changelog (no new spec).
- There is no API surface or endpoint involved.

## Mechanics in this repo

1. **Canonical spec.** OpenAPI 3.1 in `openapi.yaml` or `openapi.json`. YAML preferred (diff-friendly).
2. **Validation.** Use `redocly lint` or `swagger-cli validate` in CI. Failure = blocks merge.
3. **Examples.** Every public endpoint has an `example` in the schema. No example = ambiguous request/response.
4. **Auth.** `securitySchemes` schemes declared once; applied per operation via `security:`. Bearer/JWT via OAuth2.
5. **Versioning.** Path-based (`/v1/`, `/v2/`); breaking change = new major + ADR linking it.
6. **Errors.** RFC 7807 (Problem Details). Status code + `type` + `detail` + `instance`.

## Minimum templates

```yaml
openapi: 3.1.0
info: { title, version, contact, license }
servers: [{ url, description }]
paths:
  /resource/{id}:
    get:
      summary, parameters, responses, security
components:
  securitySchemes: { bearerAuth: { type: http, scheme: bearer, bearerFormat: JWT } }
  schemas: { Resource: { type: object, required, properties } }
```

## Anti-patterns

- Endpoint without `operationId` (generates a broken SDK).
- Wrong status code (200 for an error, 404 for auth failure).
- Schema without `additionalProperties: false` when the object is closed.
- Spec without concrete examples (description only).
- Versioning via header (prefer path).
