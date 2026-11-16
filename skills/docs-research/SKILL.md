---
name: docs-research
description: "Use when researching official documentation for a library/framework/API with low token cost. Efficient research in official documentation with low token consumption."
---

# Documentation and Web Research

You research **official sources** before stating anything about libraries, APIs or versions. Prioritize precision and token economy.

## Use this skill when

- Consulting the official documentation of a framework/library.
- Confirming API signature, behavior, deprecation or version.
- Searching for an error/stack trace and the canonical solution.
- Needing to cite the source of a technical statement.

## Do not use this skill when

- The information is in the repository itself (read the code first).
- You can answer with verifiable local knowledge (avoid going out to the web).
- The task is code review/local debug without an external dependency.

## Instructions

1. **Define the question** and the exact version of the stack (e.g.: Symfony 7.2, Doctrine ORM 3, Go 1.24).
2. **Search** with 2–3 variations; prefer official and versioned domains.
3. **Map before reading**: use `docs-fetch -outline <url>` to see the structure and choose the section.
4. **Extract the minimum**: `docs-fetch <url>` (text) or `-grep <term>`; avoid dumping entire pages.
5. **Cite** URL + version; separate what was verified from what is assumption.
6. If the sources conflict, show both and the version context.

## Tools

- **CLI native search:** `websearch`/`webfetch` (OpenCode), `WebSearch`/`WebFetch` (Claude), web search (Codex), grounding (Gemini).
- **`docs-fetch`** (agent-sync): downloads and caches docs, extracts text and titles.
  - `docs-fetch -outline <url>` → titles (h1-h6) to map the page.
  - `docs-fetch <url>` → clean text (without script/style).
  - `docs-fetch -grep <term> <url>` → only the relevant lines.
  - `docs-fetch -raw <url>` → raw content (e.g.: markdown/RST).
  - Cache in `~/.cache/agent-sync/docs`; use `-refresh` to update.
- **[`search-specialist`](reference/search-specialist.md)** (absorbed into this skill) for broad/competitive research and factual verification.

## Official sources per stack

### PHP / Symfony / Doctrine

- **Symfony:** `https://symfony.com/doc/current/` (replace with `/doc/<version>/`, e.g.: `/doc/7.2/`); API `https://api.symfony.com/`.
- **Symfony components:** `https://symfony.com/components`.
- **Doctrine ORM:** `https://www.doctrine-project.org/projects/doctrine-orm/en/current/` (versioned, e.g.: `/en/3.3/`).
- **Doctrine DBAL / Migrations:** `.../projects/doctrine-dbal/en/current/`, `.../doctrine-migrations/en/current/`.
- **PHP:** `https://www.php.net/manual/en/`; RFCs `https://wiki.php.net/rfc`.
- **PSRs:** `https://www.php-fig.org/psr/`.
- **Composer:** `https://getcomposer.org/doc/`.
- **PHPUnit:** `https://docs.phpunit.de/`.

### Go

- **Guide/language:** `https://go.dev/doc/` and `https://go.dev/ref/spec`.
- **Packages/API:** `https://pkg.go.dev/<package>`.
- **Release notes:** `https://go.dev/doc/devel/release`.

### JavaScript / TypeScript / Node

- **MDN:** `https://developer.mozilla.org/`.
- **TypeScript:** `https://www.typescriptlang.org/docs/`.
- **Node.js API:** `https://nodejs.org/api/`.
- **npm:** `https://docs.npmjs.com/`.

### Database and security

- **PostgreSQL:** `https://www.postgresql.org/docs/<version>/` (e.g.: `/docs/16/`).
- **OWASP Top 10:** `https://owasp.org/www-project-top-ten/`.
- **Git:** `https://git-scm.com/doc`.

## Quality rules

- **Never invent** function, flag, package or endpoint names: confirm in the source before citing.
- Prefer the **versioned** doc to `current` when the project pins a version.
- Clearly differentiate: **verified in the doc** vs **inference**.
- Always cite the URL; for strong statements, include the exact excerpt.
- Do not use blogs/forums as a primary source; use them as a clue and confirm in the official doc.
- Respect `robots.txt`/limits: prefer specific pages instead of broad crawling.

## Repository references

- Symfony/Doctrine integration: `backend/reference/php.md`.
- Evaluation of an MCP dedicated to docs: `mcp-advisor`.
- [search-specialist](reference/search-specialist.md) — Search operators, domain filtering and factual verification.
