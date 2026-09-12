# Agent-Sync 🚀

> 🇺🇸 [English](README.md) · 🇧🇷 [Português](README.pt-BR.md)

Repositório unificado para versionar, manter e sincronizar **Regras Globais**, **Skills**, **Agentes Especialistas** e **Ferramentas de Baixo Consumo de Tokens** em múltiplos ecossistemas de agentes de IA:
- **Claude Code** (`~/.claude`)
- **Codex / OpenAI** (`~/.codex`)
- **Google Antigravity CLI** (`~/.gemini`) — sucessora da Gemini CLI standalone, hoje descontinuada (encerrada em 18/06/2026 para contas não-enterprise)
- **OpenCode** (`~/.config/opencode`)

---

## 📦 Estrutura do Repositório

```text
agent-sync/
├── rules/
│   └── global-rules.md     # Regras globais (Clean Code, OWASP, Anti-Alucinação, Data Guardians)
├── skills/
│   ├── manifest.json       # Curadoria: IDs + origem (rmyndharis/antigravity-skills, MIT) usada por `-vendor`
│   ├── token-saving-toolkit/ # Instruções para leitura concisa via AST e poda de logs
│   ├── mcp-advisor/          # Avaliação de uso de MCPs
│   └── <41 skills>/          # Vendorizadas do catálogo (backend, segurança, banco, API, linguagens, testes, ops, contexto)
├── agents/                 # Agentes especialistas autorais (canônicos), gerados por CLI no `-apply`
│   ├── spec-planner.md, code-reviewer.md, security-auditor.md, debugger.md
│   └── architecture-reviewer.md, test-engineer.md, refactor-specialist.md, db-guardian.md, token-optimizer.md, sentry-debugger.md
├── tools/                  # Binários utilitários de alta velocidade em Go
│   ├── cmd/ast-outline/    # Extrai classes/métodos em vez de ler arquivos inteiros (Go, Python, TS, PHP)
│   ├── cmd/trace-strip/    # Remove ruídos de frameworks em logs de erro
│   ├── cmd/db-guardian/    # Valida SQL read-only (bloqueia mutações, injeta LIMIT); NÃO executa queries
│   └── cmd/docs-fetch/     # Baixa/cacheia docs e extrai texto ou outline de títulos (baixo token)
│   └── cmd/docs-mcp/       # Servidor MCP local (offline) sobre o cache de docs
├── mirror/sources.json     # Fontes oficiais curadas para sincronizar no cache local
├── templates/STATE.md      # Template de handoff de sessão (checkpoint/retomada)
├── cmd/agent-sync/         # Orquestrador de sincronização CLI (+ vendor de skills + gerador de agentes)
├── scripts/setup-go.sh     # Bootstrap do Go (>= 1.24) via mise ou tarball oficial
├── scripts/setup-mcp.sh    # Configura Context7, MCP local de docs e Sentry (opcional)
├── LICENSE / NOTICE        # Licença MIT e atribuição das skills de terceiros
├── Makefile                # Comandos de automação
└── README.md / README.pt-BR.md  # docs (EN padrão, PT-BR)
```

### Agentes especialistas

Definidos uma vez em `agents/*.md` (frontmatter `name`, `description` e `readonly` opcional) e **gerados no formato nativo** de cada CLI durante o `-apply`:

| Agente | Papel | Read-only |
| --- | --- | --- |
| `spec-planner` | Entende o pedido e planeja antes de codar | ✅ |
| `code-reviewer` | Clean Code, SOLID, Calisthenics e segurança | ✅ |
| `security-auditor` | OWASP, injeção, XSS, segredos | ✅ |
| `debugger` | Causa raiz com hipóteses e evidências | ❌ |
| `architecture-reviewer` | Fronteiras, DDD, acoplamento, ADRs | ✅ |
| `test-engineer` | TDD red-green-refactor | ❌ |
| `refactor-specialist` | Refatoração incremental guiada por testes | ❌ |
| `db-guardian` | SQL read-only, LIMIT, PII | ✅ |
| `token-optimizer` | Inspeção de baixo token (ast-outline/trace-strip) | ✅ |
| `sentry-debugger` | Triagem de issues/erros no Sentry e causa raiz | ✅ |

> "Read-only" vira `permission.edit=deny` no OpenCode e `sandbox_mode=read-only` no Codex; nos demais, é reforçado pelo prompt.

### Skills vendorizadas do catálogo

Curadas por domínio no `skills/manifest.json` e importadas de [rmyndharis/antigravity-skills](https://github.com/rmyndharis/antigravity-skills) (MIT):

| Domínio | Skills |
| --- | --- |
| Backend/qualidade | `error-handling-patterns`, `code-refactoring-refactor-clean`, `dependency-management-deps-audit`, `codebase-cleanup-tech-debt`, `legacy-modernizer`, `nodejs-backend-patterns`, `code-reviewer`, `debugging-strategies` |
| Arquitetura | `architecture-patterns`, `architect-review`, `architecture-decision-records` |
| Arq. distribuída | `microservices-patterns`, `cqrs-implementation`, `event-sourcing-architect` |
| Segurança | `auth-implementation-patterns`, `security-auditor`, `sast-configuration`, `backend-security-coder`, `frontend-security-coder` |
| Banco de dados | `sql-optimization-patterns`, `database-optimizer`, `database-migrations-sql-migrations`, `postgresql` |
| API/Docs | `openapi-spec-generation`, `api-documenter`, `api-design-principles` |
| Linguagens | `php-pro`, `python-pro`, `golang-pro`, `go-concurrency-patterns`, `javascript-pro`, `typescript-pro` |
| Testes | `python-testing-patterns`, `javascript-testing-patterns`, `e2e-testing-patterns`, `tdd-orchestrator` |
| Ops/Infra | `incident-response-smart-fix`, `postmortem-writing` |
| Pesquisa web | `search-specialist` |
| Contexto | `context-manager`, `context-management-context-save` |

Além das vendorizadas, há **9 skills autorais em PT-BR** (não existem no catálogo):
`ddd`, `design-patterns`, `object-calisthenics`, `symfony`, `doctrine`, `phpunit-symfony`, `docs-research`, `context-guard` e `sentry`.

### Contexto e sessões longas

- **`context-guard`** (skill): zonas de saúde, sinais de drift, reancoragem após compaction e checkpoint. **Obrigatória por padrão** em tarefas multi-etapa/sessões longas (referenciada nas `global-rules`, no topo e na reafirmação final).
- **`STATE.md`**: fonte de verdade para retomar sessões. Base em `templates/STATE.md`; mantenha no projeto.
- Ferramentas de baixo token: `ast-outline`, `trace-strip`, `docs-fetch`, `docs-mcp`, `db-guardian` (ver `token-saving-toolkit`).
- **Hook do `context-guard`** (`hooks/context-guard-nudge.sh`): como a skill acima depende só da autodisciplina do modelo, o `-apply` também instala um lembrete real no nível do harness, disparado a cada N chamadas de ferramenta (padrão 40, `AGENT_SYNC_NUDGE_THRESHOLD`) cobrando o carregamento do `context-guard` e a atualização do `STATE.md`. Cobertura por CLI:
  - **Claude Code**: hook `PostToolUse` mesclado em `~/.claude/settings.json` (preserva hooks/chaves existentes, reaplicação idempotente).
  - **Codex**: hook `PostToolUse` mesclado em `~/.codex/hooks.json`.
  - **Antigravity CLI**: hook `PreInvocation` mesclado em `~/.gemini/config/hooks.json` — formato de topo diferente (sem chave `hooks` envolvente: `{"<nome-do-hook>": {"<Evento>": [...]}}`) e payload diferente da antiga Gemini CLI. Usa o contador nativo `invocationNum` do evento em vez de manter arquivo de estado próprio.
  - **OpenCode**: plugin `tool.execute.after` copiado para `~/.config/opencode/plugins/context-guard-nudge.ts`. **Best-effort**: o OpenCode tem uma issue aberta upstream ([anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574)) em que mutações de output desse hook nem sempre são refletidas de volta ao modelo — o lembrete pode não chegar de forma confiável.
- **`bash-guardian`** (`hooks/bash-guardian-patterns.txt`): pede confirmação antes de rodar comandos que batem padrões conhecidos de risco (`rm -rf`, `dd` pra device, `chmod -R 777`, `curl | sh`, `git push --force`, `git reset --hard`, `shutdown`, etc.). Comportamento padrão é sempre **ask**, nunca bloqueio silencioso. Cobertura por CLI:
  - **Claude Code**: padrões adicionados à lista nativa `permissions.ask` do `settings.json` (ex.: `Bash(rm -rf *)`).
  - **OpenCode**: mesclado em `permission.bash` no `~/.config/opencode/opencode.json`, cada um marcado `"ask"`. Como o OpenCode resolve `permission.bash` pela **última regra que casa** (ordem importa), o merge preserva a ordem original das chaves já existentes no arquivo e só acrescenta/reordena as próprias entradas do guard ao final — nunca reordena chaves alheias. **Atenção**: se um padrão já existia com valor diferente (ex.: um `"allow"` definido pelo usuário), o guard sobrescreve para `"ask"` — é o propósito do guardrail, mas vale saber antes de rodar `-apply` sobre uma config já existente.
  - **Antigravity CLI**: hook `PreToolUse` (`hooks/bash-guardian.antigravity.sh`) devolvendo `{"decision":"ask"}` quando bate.
  - **Codex**: **não implementado**. O `PreToolUse` dele só suporta `allow`/`deny` binário — `"ask"` está documentado explicitamente como "parsed but not supported yet". Em vez de degradar silenciosamente pra `deny` (bloqueando trabalho legítimo) ou `allow` (sem proteção nenhuma), o Codex fica de fora do guard até o suporte a confirmação interativa em hooks ser lançado.
- **`docs-cache`** (`tools/cmd/docs-cache-write` + `hooks/docs-cache*`): cacheia passivamente docs que o agente já consultou via `WebFetch`/`read_url_content` ou `context7` (só `query-docs` — `resolve-library-id` é metadado, não conteúdo de doc), sem refazer requisição de rede. Faz o cache offline do `docs-fetch` crescer organicamente conforme bibliotecas são consultadas, complementando o `mirror/sources.json` curado manualmente. Resultados do `context7` são cacheados sob uma chave sintética (`context7:/<libraryId>/<query>`). Cobertura por CLI:
  - **Claude Code / Codex**: hook `PostToolUse` compartilhado (`hooks/docs-cache.sh`), matcher `WebFetch|mcp__context7__.*`. Codex não tem tool de fetch de página inteira, então só a metade do `context7` se aplica lá.
  - **Antigravity CLI**: hook `PostToolUse` (`hooks/docs-cache.antigravity.sh`) casando `read_url_content|call_mcp_tool`. O payload do hook do Antigravity não inclui o resultado da tool — o script lê do `transcriptPath` documentado (JSONL), localizando a entrada em `step_index + 1`, confirmado testando ao vivo contra uma sessão real do `agy`. O nome do argumento do `read_url_content` (`Url`) é uma inferência razoável pela convenção PascalCase dessa tool, **não confirmada ao vivo** (o teste de permissão pra isso foi bloqueado pelo próprio classificador de segurança desta sessão antes da confirmação).
  - **OpenCode**: plugin `tool.execute.after` (`hooks/docs-cache.opencode.ts`), mesma ressalva de best-effort do `context-guard-nudge.ts`.

### Pesquisa e documentação

- **`docs-research`** (skill): padrão de pesquisa em fontes oficiais (Symfony, Doctrine, PHP, Go, JS/TS…) com citação e baixo token.
- **`docs-fetch`** (tool): baixa e cacheia docs; consulta online ou offline.
  ```bash
  make mirror                    # sincroniza mirror/sources.json (~65 fontes oficiais) no cache
  docs-fetch -outline https://www.doctrine-project.org/projects/doctrine-orm/en/current/reference/basic-mapping.html
  docs-fetch -search "lazy loading"   # busca offline no cache
  docs-fetch -list                    # docs cacheadas
  ```
  Fontes curadas em `mirror/sources.json`: Symfony, Doctrine (ORM/DBAL/Collections/Migrations), PHP, PSR, PHPUnit, Composer, Go, TypeScript, JavaScript/MDN, Node, React, Vue, Next.js, Tailwind, Vite, Laravel, Rails, Python, Django, FastAPI, Spring Boot, .NET/C#, Rust, Elixir, Vitest, Jest, Playwright, PostgreSQL, SQLite, MariaDB, MongoDB, Redis, Kafka, Elasticsearch, RabbitMQ, Docker, Kubernetes, Terraform, Nginx, Git, ESLint, OWASP.
- **MCPs** nos 4 CLIs:
  - `context7` — docs atualizadas de bibliotecas. Padrão **remoto**; **local/stdio** com `CONTEXT7_LOCAL=1`.
    ```bash
    make mcp                                   # remoto
    CONTEXT7_LOCAL=1 make mcp                  # local (stdio, via npx — ainda precisa de internet)
    CONTEXT7_API_KEY=xxx make mcp              # limites maiores (não fica no repo)
    ```
  - `docs` — **MCP local offline** (`docs-mcp`) que busca no cache do `docs-fetch`. Funciona sem internet após o `make mirror`.
  - `sentry` — erros/performance do Sentry (**opcional**, OAuth). Defina a URL com org/projeto:
    ```bash
    SENTRY_MCP_URL=https://mcp.sentry.dev/mcp/<org>/<proj> make mcp
    ```

> Context7 é hospedado: mesmo em modo local (stdio) o servidor consulta a API do context7.com — não é offline. Para offline de verdade, use o MCP `docs` + `make mirror`.

---

## ⚡ Como Usar em Qualquer Máquina

### 1. Clonar o repositório
```bash
git clone <seu-repo-url> ~/Documentos/agent-sync
cd ~/Documentos/agent-sync
```

### 2. Garantir o Go (>= 1.24)
```bash
make setup
```
Verifica o `go` disponível e, se ausente/antigo, instala a versão exigida pelos `go.mod` via `mise` (se disponível) ou pelo tarball oficial em `~/.local` — sem `sudo`. O `toolchain` em `go.mod` fixa a versão mínima sugerida (`go1.24.13`).

### 3. Compilar e Instalar tudo
```bash
make install
```
Isso compilará os binários em Go (`agent-sync`, `ast-outline`, `trace-strip`, `db-guardian`, `docs-fetch`, `docs-mcp`) e os colocará em `~/.local/bin/`.

### 4. Sincronizar com todas as CLIs
```bash
make sync
# ou diretamente:
agent-sync -apply
```

### 5. MCPs de documentação (opcional)
```bash
make mirror                    # baixa docs oficiais para o cache offline
make mcp                       # Context7 (remoto) + MCP local offline 'docs'
CONTEXT7_LOCAL=1 make mcp      # Context7 em modo local (stdio)
```

### 6. Verificar Status das CLIs
```bash
agent-sync -status
```

### 7. Reimportar as skills curadas do catálogo
```bash
make vendor
# ou diretamente (origem configurável com -source):
agent-sync -vendor
```

> A resolução da raiz do repositório pode ser forçada com a variável `AGENT_SYNC_HOME`.

---

## 🛡️ Ferramentas Inclusas
- **`ast-outline <arquivo>`**: Gera a estrutura de classes e funções com linhas correspondentes, economizando até 90% dos tokens de contexto.
- **`trace-strip <arquivo_ou_pipe>`**: Oculta frames irrelevantes de stack traces de bibliotecas externas.
- **`db-guardian -list`** / **`-profile <banco> -query "<sql>"`**: Valida queries em modo read-only (bloqueia mutações, avisa sobre `SELECT *` e injeta `LIMIT`). **Não abre conexão nem executa a query**; o perfil serve apenas para contexto. Credenciais aceitam indireção por ambiente (`${DB_PASSWORD}`).
- **`docs-fetch <url>`**: Baixa e cacheia uma página de docs e extrai texto (`-outline`, `-grep`, `-raw`, `-refresh`) para consulta de baixo token.
