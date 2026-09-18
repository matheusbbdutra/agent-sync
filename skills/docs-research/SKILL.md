---
name: docs-research
description: "Pesquisa eficiente em documentações oficiais com baixo consumo de tokens."
---

# Pesquisa de Documentação e Web

Você pesquisa em **fontes oficiais** antes de afirmar qualquer coisa sobre bibliotecas, APIs ou versões. Priorize precisão e economia de tokens.

## Use this skill when

- Consultar documentação oficial de um framework/biblioteca.
- Confirmar assinatura de API, comportamento, depreciação ou versão.
- Buscar um erro/stack trace e a solução canônica.
- Precisar citar a fonte de uma afirmação técnica.

## Do not use this skill when

- A informação está no próprio repositório (leia o código antes).
- Você pode responder com conhecimento verificável local (evite sair para a web).
- A tarefa é revisão de código/debug local sem dependência externa.

## Instructions

1. **Defina a pergunta** e a versão exata da stack (ex.: Symfony 7.2, Doctrine ORM 3, Go 1.24).
2. **Busque** com 2–3 variações; prefira domínios oficiais e versionados.
3. **Mapeie antes de ler**: use `docs-fetch -outline <url>` para ver a estrutura e escolher a seção.
4. **Extraia o mínimo**: `docs-fetch <url>` (texto) ou `-grep <termo>`; evite despejar páginas inteiras.
5. **Cite** URL + versão; separe o que foi verificado do que é suposição.
6. Se as fontes conflitarem, mostre as duas e o contexto da versão.

## Ferramentas

- **Busca nativa da CLI:** `websearch`/`webfetch` (OpenCode), `WebSearch`/`WebFetch` (Claude), web search (Codex), grounding (Gemini).
- **`docs-fetch`** (agent-sync): baixa e cacheia docs, extrai texto e títulos.
  - `docs-fetch -outline <url>` → títulos (h1-h6) para mapear a página.
  - `docs-fetch <url>` → texto limpo (sem script/style).
  - `docs-fetch -grep <termo> <url>` → só linhas relevantes.
  - `docs-fetch -raw <url>` → conteúdo bruto (ex.: markdown/RST).
  - Cache em `~/.cache/agent-sync/docs`; use `-refresh` para atualizar.
- **`search-specialist`** (skill) para pesquisa ampla/competitiva e verificação factual.

## Fontes oficiais por stack

### PHP / Symfony / Doctrine

- **Symfony:** `https://symfony.com/doc/current/` (troque por `/doc/<versão>/`, ex.: `/doc/7.2/`); API `https://api.symfony.com/`.
- **Symfony components:** `https://symfony.com/components`.
- **Doctrine ORM:** `https://www.doctrine-project.org/projects/doctrine-orm/en/current/` (versionado, ex.: `/en/3.3/`).
- **Doctrine DBAL / Migrations:** `.../projects/doctrine-dbal/en/current/`, `.../doctrine-migrations/en/current/`.
- **PHP:** `https://www.php.net/manual/en/`; RFCs `https://wiki.php.net/rfc`.
- **PSRs:** `https://www.php-fig.org/psr/`.
- **Composer:** `https://getcomposer.org/doc/`.
- **PHPUnit:** `https://docs.phpunit.de/`.

### Go

- **Guia/linguagem:** `https://go.dev/doc/` e `https://go.dev/ref/spec`.
- **Pacotes/API:** `https://pkg.go.dev/<pacote>`.
- **Release notes:** `https://go.dev/doc/devel/release`.

### JavaScript / TypeScript / Node

- **MDN:** `https://developer.mozilla.org/`.
- **TypeScript:** `https://www.typescriptlang.org/docs/`.
- **Node.js API:** `https://nodejs.org/api/`.
- **npm:** `https://docs.npmjs.com/`.

### Banco de dados e segurança

- **PostgreSQL:** `https://www.postgresql.org/docs/<versão>/` (ex.: `/docs/16/`).
- **OWASP Top 10:** `https://owasp.org/www-project-top-ten/`.
- **Git:** `https://git-scm.com/doc`.

## Regras de qualidade

- **Nunca invente** nomes de funções, flags, pacotes ou endpoints: confirme na fonte antes de citar.
- Prefira **versão versionada** da doc à `current` quando o projeto fixa uma versão.
- Diferencie claramente: **verificado na doc** vs **inferência**.
- Cite sempre a URL; para afirmações fortes, inclua o trecho exato.
- Não use blogs/fóruns como fonte primária; use como pista e confirme na doc oficial.
- Respeite `robots.txt`/limites: prefira páginas específicas em vez de crawling amplo.

## Referências do repositório

- Integração Symfony/Doctrine: `symfony`, `doctrine`, `phpunit-symfony`.
- Avaliação de MCP dedicado a docs: `mcp-advisor`.
