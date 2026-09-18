# Agent-Sync 🚀

> 🇺🇸 [English](README.md) · 🇧🇷 [Português](README.pt-BR.md)

Repositório unificado para versionar, manter e sincronizar **Regras Globais**, **Skills**, **Agentes Especialistas** e **Ferramentas de Baixo Consumo de Tokens** em múltiplos ecossistemas de agentes de IA:
- **Claude Code** (`~/.claude`)
- **Codex / OpenAI** (`~/.codex`)
- **Google Antigravity CLI** (`~/.gemini`) — sucessora da Gemini CLI standalone, hoje descontinuada (encerrada em 18/06/2026 para contas não-enterprise)
- **OpenCode** (`~/.config/opencode`)
- **Cursor** (`~/.cursor`) — IDE + Agent CLI (`agent` / `cursor-agent`)

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
│   └── architecture-reviewer.md, test-engineer.md, refactor-specialist.md, db-guardian.md, token-optimizer.md, sentry-debugger.md, mr-reviewer.md
├── tools/                  # Binários utilitários de alta velocidade em Go
│   ├── cmd/ast-outline/    # Extrai classes/métodos em vez de ler arquivos inteiros (Go, Python, TS, PHP)
│   ├── cmd/trace-strip/    # Remove ruídos de frameworks em logs de erro
│   ├── cmd/db-guardian/    # Valida SQL read-only (bloqueia mutações, injeta LIMIT); NÃO executa queries
│   └── cmd/docs-fetch/     # Baixa/cacheia docs e extrai texto ou outline de títulos (baixo token)
│   └── cmd/docs-mcp/       # Servidor MCP local (offline) sobre o cache de docs
│   └── cmd/memory-mcp/     # Servidor MCP de memória compartilhada entre CLIs (libSQL local, CGO)
├── mirror/sources.json     # Fontes oficiais curadas para sincronizar no cache local
├── templates/STATE.md      # Template de handoff de sessão (checkpoint/retomada)
├── cmd/agent-sync/         # Orquestrador de sincronização CLI (+ vendor de skills + gerador de agentes)
├── scripts/setup-go.sh     # Bootstrap do Go (>= 1.24) via mise ou tarball oficial
├── scripts/setup-mcp.sh    # Configura Context7, MCP local de docs, memória compartilhada e Sentry (opcional)
├── LICENSE / NOTICE        # Licença MIT e atribuição das skills de terceiros
├── Makefile                # Comandos de automação
└── README.md / README.pt-BR.md  # docs (EN padrão, PT-BR)
```

### Agentes especialistas

Definidos uma vez em `agents/*.md` (frontmatter `name`, `description` e `readonly` opcional) e **gerados no formato nativo** de cada CLI durante o `-apply`:

| Agente | Papel | Read-only |
| --- | --- | --- |
| `spec-planner` | Entende o pedido e planeja antes de codar | ✅ |
| `mr-reviewer` | Revisa refs Git locais com evidência de regressões, segurança e impacto | ✅ |
| `code-reviewer` | Clean Code, SOLID, Calisthenics e segurança | ✅ |
| `security-auditor` | OWASP, injeção, XSS, segredos | ✅ |
| `debugger` | Causa raiz com hipóteses e evidências | ❌ |
| `architecture-reviewer` | Fronteiras, DDD, acoplamento, ADRs | ✅ |
| `test-engineer` | TDD red-green-refactor | ❌ |
| `refactor-specialist` | Refatoração incremental guiada por testes | ❌ |
| `db-guardian` | SQL read-only, LIMIT, PII | ✅ |
| `token-optimizer` | Inspeção de baixo token (ast-outline/trace-strip) | ✅ |
| `sentry-debugger` | Triagem de issues/erros no Sentry e causa raiz | ✅ |

> "Read-only" vira `permission.edit=deny` no OpenCode, `sandbox_mode=read-only` no Codex e `readonly: true` no Cursor; nos demais, é reforçado pelo prompt.

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

Além das vendorizadas, há **12 skills autorais em PT-BR** (não existem no catálogo):
`ddd`, `design-patterns`, `object-calisthenics`, `symfony`, `doctrine`, `phpunit-symfony`, `docs-research`, `context-guard`, `sentry`, `agent-delegate`, `arch-context-check` e `agent-react`.

### Memória compartilhada entre CLIs

- **`memory-mcp`** (`tools/cmd/memory-mcp`): servidor MCP local sobre libSQL (`~/.cache/agent-sync/memory.db`, com sincronização opcional de memórias persistentes via Turso Cloud; busca FTS5 local) que dá a Claude Code, Codex, agy, OpenCode e Cursor acesso ao mesmo histórico de decisões/feedback/contexto de projeto. Requer CGO (`go-libsql`) — assumido aceitável para uso pessoal (gcc/clang já é pré-requisito do `make`).
- Busca hoje é **FTS5/BM25** (relevância por texto), sem embedding real — o schema já reserva uma coluna vetorial (`embedding_json`) para uma fase futura de busca semântica.
- Ferramentas MCP expostas: `store_memory` (aceita `scratch: true|false`), `search_memory`, `get_memory`, `list_memories`, `delete_memory` (só remove memórias gravadas com `scratch: true` — permanentes são recusadas por design). O enum de proveniência inclui `cursor`.
- **Hook `memory-nudge`** (`hooks/memory-nudge.sh` / `.antigravity.sh` / `.opencode.ts` / `.cursor.sh`): a gravação de memória hoje depende só da disciplina do modelo (nenhum gatilho automático), então o `-apply` também instala um lembrete em nível de harness que dispara a cada N chamadas de ferramenta/invocações (padrão 25, `AGENT_SYNC_MEMORY_NUDGE_THRESHOLD`) perguntando se algo da sessão deveria ser salvo via `store_memory`. Mesmo mecanismo de instalação do `context-guard-nudge` (contador/threshold separados), cobrindo os 5 alvos:
  - **Claude Code**: hook `PostToolUse` mesclado em `~/.claude/settings.json`.
  - **Codex**: hook `PostToolUse` mesclado em `~/.codex/hooks.json`; comandos `protect-mcp` são adaptados ao schema nativo (`PreToolUse`/`PostToolUse`) durante a sincronização.
  - Falhas dos hooks são persistidas localmente em `~/.cache/agent-sync/hooks/errors.jsonl` (com redaction e rotação); `agent-sync -observability` exibe um resumo.
  - **Antigravity CLI**: hook `PreInvocation` mesclado em `~/.gemini/config/hooks.json`.
  - **OpenCode**: plugin `tool.execute.after` copiado para `~/.config/opencode/plugins/memory-nudge.ts`. Mesma ressalva best-effort do `context-guard-nudge` ([anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574)).
  - **Cursor**: hook `postToolUse` em `~/.cursor/hooks.json` (script em `~/.cursor/hooks/`, saída nativa `additional_context`).
- **`agent-delegate`** (skill): critérios para decidir se/para qual CLI-modelo delegar uma tarefa, usando perfil de fricção de permissão (`print` vs `session`; default headless: OpenCode). Prefira `delegate-run` (`scripts/delegate-run.sh`, instalado no `make install`) para log/manifesto/tmux em vez de Bash cru. Sempre consulte o `memory-mcp` antes de montar o prompt. Limitação documentada: Claude Code não orquestra `agy` em headless.
- **`agent-react`** (skill): disciplina o loop ReAct (Thought → Action → Observation) em tarefas multi-etapa — anti-loop, budget de tools e evidência citada; complementa `context-guard` / `debugging-strategies` sem substituí-los.
- **Hook `agent-react-nudge`** (`hooks/agent-react-nudge.sh` / `.antigravity.sh` / `.opencode.ts` / `.cursor.sh`): lembrete a cada N tool calls/invocações (padrão 15, `AGENT_SYNC_REACT_NUDGE_THRESHOLD`) cobrando validação de hipóteses ativas antes de concluir/implementar. Mesmo mecanismo dos outros nudges; OpenCode best-effort ([anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574)). Não garante semanticamente — só reforça o contrato hipótese ≠ fato.
- **`arch-context-check`** (skill): checklist obrigatório antes de sugerir arquitetura/Clean Code/DDD/padrões de projeto — cruza os skills especializados (`ddd`, `design-patterns`, `object-calisthenics`, `architecture-patterns`) com decisões anteriores no `memory-mcp` e com o código real, antes de dar uma sugestão.

### Contexto e sessões longas

- **`context-guard`** (skill): zonas de saúde, sinais de drift, reancoragem após compaction e checkpoint. **Obrigatória por padrão** em tarefas multi-etapa/sessões longas (referenciada nas `global-rules`, no topo e na reafirmação final).
- **`STATE.md`**: fonte de verdade para retomar sessões. Base em `templates/STATE.md`; mantenha no projeto.
- Ferramentas de baixo token: `ast-outline`, `trace-strip`, `docs-fetch`, `docs-mcp`, `memory-mcp`, `db-guardian` (ver `token-saving-toolkit`).
- **Hook do `context-guard`** (`hooks/context-guard-nudge.sh`): como a skill acima depende só da autodisciplina do modelo, o `-apply` também instala um lembrete real no nível do harness, disparado a cada N chamadas de ferramenta (padrão 40, `AGENT_SYNC_NUDGE_THRESHOLD`) cobrando o carregamento do `context-guard` e a atualização do `STATE.md`. Cobertura por CLI:
  - **Claude Code**: hook `PostToolUse` mesclado em `~/.claude/settings.json` (preserva hooks/chaves existentes, reaplicação idempotente).
  - **Codex**: hook `PostToolUse` mesclado em `~/.codex/hooks.json`.
  - **Antigravity CLI**: hook `PreInvocation` mesclado em `~/.gemini/config/hooks.json` — formato de topo diferente (sem chave `hooks` envolvente: `{"<nome-do-hook>": {"<Evento>": [...]}}`) e payload diferente da antiga Gemini CLI. Usa o contador nativo `invocationNum` do evento em vez de manter arquivo de estado próprio.
  - **OpenCode**: plugin `tool.execute.after` copiado para `~/.config/opencode/plugins/context-guard-nudge.ts`. **Best-effort**: o OpenCode tem uma issue aberta upstream ([anomalyco/opencode#13574](https://github.com/anomalyco/opencode/issues/13574)) em que mutações de output desse hook nem sempre são refletidas de volta ao modelo — o lembrete pode não chegar de forma confiável.
  - **Cursor**: `postToolUse` em `~/.cursor/hooks.json` devolvendo `additional_context` (schema nativo; scripts em `~/.cursor/hooks/` referenciados como `./hooks/...`).
- **`bash-guardian`** (`hooks/bash-guardian-patterns.txt`): pede confirmação antes de rodar comandos que batem padrões conhecidos de risco (`rm -rf`, `dd` pra device, `chmod -R 777`, `curl | sh`, `git push --force`, `git reset --hard`, `shutdown`, etc.). Comportamento padrão é sempre **ask**, nunca bloqueio silencioso. Cobertura por CLI:
  - **Claude Code**: padrões adicionados à lista nativa `permissions.ask` do `settings.json` (ex.: `Bash(rm -rf *)`).
  - **OpenCode**: mesclado em `permission.bash` no `~/.config/opencode/opencode.json`, cada um marcado `"ask"`. Como o OpenCode resolve `permission.bash` pela **última regra que casa** (ordem importa), o merge preserva a ordem original das chaves já existentes no arquivo e só acrescenta/reordena as próprias entradas do guard ao final — nunca reordena chaves alheias. **Atenção**: se um padrão já existia com valor diferente (ex.: um `"allow"` definido pelo usuário), o guard sobrescreve para `"ask"` — é o propósito do guardrail, mas vale saber antes de rodar `-apply` sobre uma config já existente.
  - **Antigravity CLI**: hook `PreToolUse` (`hooks/bash-guardian.antigravity.sh`) devolvendo `{"decision":"ask"}` quando bate.
  - **Cursor**: hook `beforeShellExecution` (`hooks/bash-guardian.cursor.sh`) devolvendo `{"permission":"ask"}` — o Cursor suporta confirmação interativa nativamente aqui.
  - **Codex**: **não implementado**. O `PreToolUse` dele só suporta `allow`/`deny` binário — `"ask"` está documentado explicitamente como "parsed but not supported yet". Em vez de degradar silenciosamente pra `deny` (bloqueando trabalho legítimo) ou `allow` (sem proteção nenhuma), o Codex fica de fora do guard até o suporte a confirmação interativa em hooks ser lançado.
- **`docs-cache`** (`tools/cmd/docs-cache-write` + `hooks/docs-cache*`): cacheia passivamente docs que o agente já consultou via `WebFetch`/`read_url_content` ou `context7` (só `query-docs` — `resolve-library-id` é metadado, não conteúdo de doc), sem refazer requisição de rede. Faz o cache offline do `docs-fetch` crescer organicamente conforme bibliotecas são consultadas, complementando o `mirror/sources.json` curado manualmente. Resultados do `context7` são cacheados sob uma chave sintética (`context7:/<libraryId>/<query>`). Antes de cachear, o conteúdo passa pelo `tools/internal/secretscan` (só regex, sem chamada de LLM) que redige segredos óbvios (chaves AWS/GitHub/Slack, JWTs, blocos de chave privada) — ver a orientação de `STATE.md` na skill `context-guard` para a disciplina equivalente onde não há guardrail automático. Cobertura por CLI:
  - **Claude Code / Codex**: hook `PostToolUse` compartilhado (`hooks/docs-cache.sh`), matcher `WebFetch|mcp__context7__.*`. Codex não tem tool de fetch de página inteira, então só a metade do `context7` se aplica lá.
  - **Antigravity CLI**: hook `PostToolUse` (`hooks/docs-cache.antigravity.sh`) casando `read_url_content|call_mcp_tool`. O payload do hook do Antigravity não inclui o resultado da tool — o script lê do `transcriptPath` documentado (JSONL), localizando a entrada em `step_index + 1`, confirmado testando ao vivo contra uma sessão real do `agy`. O nome do argumento do `read_url_content` (`Url`) é uma inferência razoável pela convenção PascalCase dessa tool, **não confirmada ao vivo** (o teste de permissão pra isso foi bloqueado pelo próprio classificador de segurança desta sessão antes da confirmação).
  - **OpenCode**: plugin `tool.execute.after` (`hooks/docs-cache.opencode.ts`), mesma ressalva de best-effort do `context-guard-nudge.ts`.
  - **Cursor**: `postToolUse` matcher `WebFetch` + `afterMCPExecution` matcher `query-docs` (`hooks/docs-cache.cursor.sh` / `docs-cache-mcp.cursor.sh`).
- **`context-window-strategy`** (skill + tool `tools/cmd/ctx-window/`): infraestrutura de controle de janela de contexto baseada em *Sliding Window + Incremental Summary*, desenhada para sessões longas de desenvolvimento com agentes de IA.
  - **Objetivo**: Evitar o fenômeno *lost in the middle* (onde o modelo ignora decisões tomadas no meio de contextos gigantescos) e erradicar o desperdício de tokens em resumos automáticos contínuos. Mantém uma memória de trabalho (*working memory*) dos últimos $K$ passos de alta fidelidade e consolida decisões em sumários compactos sob demanda.
  - **Referência do Artigo (Microsoft Research)**: Baseado no paper *"Less Context, Better Agents: Efficient Context Engineering for Long-Horizon Tool-Using LLM Agents"* (Lodha, Pahlavikhah Varnosfaderani, Chakraborty, Mithal — 2026, [arXiv:2606.10209v1](https://arxiv.org/html/2606.10209v1)). O estudo comprovou que limitar a janela ativa aos últimos 5 tool calls acompanhados de um sumário incremental (**C4**) atinge **91.6% de sucesso** na resolução de tarefas complexas contra **71.0%** do contexto completo acumulado (**C2**), reduzindo o consumo de tokens em **63.9%**.
  - **Como funciona nas 5 CLIs**:
    - **Registro contínuo (zero LLM extra)**: A cada execução de ferramenta, os hooks capturam os parâmetros e saídas truncadas, indexando os últimos $K$ passos (default $K=5$) localmente sem invocar LLM em segundo plano.
    - **Sumarização sob demanda (`.agent-sync/summary.md`)**: Quando o contexto fica cheio (ou ao final de uma fase), o comando `ctx-window summarize` é disparado. Ele aciona o modelo da sessão para gerar um YAML estruturado com 6 seções (*decisions, active_hypotheses, artifacts, resolved_errors, next_steps, constraints*) e grava em `<projectRoot>/.agent-sync/summary.md` (protegido por `.gitignore` automático).
    - **Handoff automático entre sessões**: Ao abrir um novo chat ou reiniciar a CLI na pasta do projeto, o resumo e as últimas interações são restaurados automaticamente no contexto inicial:
      - **Claude Code**: Hook `SessionStart` injeta via `hookSpecificOutput.additionalContext`. Nudge por medição real de tokens do transcript via `PostToolUse`.
      - **Codex**: Hook `SessionStart` injeta via `hookSpecificOutput.additionalContext`. Nudge por medição de tokens dos rollouts locais via `PostToolUse`.
      - **Cursor**: Hook `sessionStart` injeta via `additional_context`. Hook `preCompact` observacional orienta a compactação manual.
      - **Antigravity CLI**: Hook oficial `PreInvocation` injeta no primeiro turno (`invocationNum == 1`) como `ephemeralMessage` (sem poluir o transcript).
      - **OpenCode**: Plugin TypeScript (`~/.config/opencode/plugins/ctx-compact.ts`) registra passos em `tool.execute.after` e injeta o snapshot local via `output.context.push()` no evento nativo `experimental.session.compacting`.
  - **Comandos principais**:
    ```bash
    ctx-window summarize          # Gera o resumo do projeto atual e grava em .agent-sync/summary.md
    ctx-window show [sessão]      # Inspeciona o working memory e os resumos salvos
    ctx-window set-k <sessão> <N> # Ajusta o K da janela deslizante (default: 5)
    ctx-window doctor             # Valida o ambiente, cache e provedores de sumarização
    ```
    Veja o [ADR completo](docs/ADR-context-window-strategy.md).

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
- **MCPs** nos 5 alvos:
  - `context7` — docs atualizadas de bibliotecas. Padrão **remoto**; **local/stdio** com `CONTEXT7_LOCAL=1`.
    ```bash
    make mcp                                   # remoto
    CONTEXT7_LOCAL=1 make mcp                  # local (stdio, via npx — ainda precisa de internet)
    CONTEXT7_API_KEY=xxx make mcp              # limites maiores (não fica no repo)
    ```
  - `docs` — **MCP local offline** (`docs-mcp`) que busca no cache do `docs-fetch`. Funciona sem internet após o `make mirror`.
  - `memory` — memória compartilhada local (`memory-mcp`) em todos os CLIs, inclusive Cursor (`~/.cursor/mcp.json`).
  - `sentry` — erros/performance do Sentry (**opcional**, OAuth). Defina a URL com org/projeto:
    ```bash
    SENTRY_MCP_URL=https://mcp.sentry.dev/mcp/<org>/<proj> make mcp
    ```

### Especificidades do Cursor

No `-apply` / `-target cursor`, o agent-sync grava:

| Artefato | Caminho |
| --- | --- |
| Regras globais | `~/.cursor/rules/agent-sync-global.mdc` (`alwaysApply: true`) — regras em arquivo local; **não** confundir com User Rules da UI (sincronizam pela conta) |
| Skills | `~/.cursor/skills/<name>/` (symlinks) — **nunca** `~/.cursor/skills-cursor/` (built-ins da Cursor) |
| Subagentes | `~/.cursor/agents/<name>.md` (`model: inherit`, `readonly: true` opcional) |
| Hooks | `~/.cursor/hooks.json` + scripts em `~/.cursor/hooks/` |
| MCP | via `make mcp` → `~/.cursor/mcp.json` (`mcpServers`) |

Ressalvas: hooks de usuário **não** rodam em Cloud Agents; Cloud Agents só pegam `.cursor/hooks.json` do projeto. User Rules da UI sincronizam pela conta e não têm path de arquivo — o `.mdc` em `~/.cursor/rules/` é o equivalente versionável.

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
Isso compilará os binários em Go (`agent-sync`, `ast-outline`, `trace-strip`, `db-guardian`, `docs-fetch`, `docs-mcp`, `memory-mcp`, `mr-review-local`, `memory-sync`) e os colocará em `~/.local/bin/`, junto com `agent-sync-session`.

### 4. Sincronizar com todas as CLIs
```bash
make sync
# ou diretamente:
agent-sync -apply
```

Use o `mr-reviewer` em qualquer CLI após `make sync`. A coleta local também pode ser executada diretamente:

```bash
mr-review-local -repo /caminho/do/checkout -base upstream/main -head origin/branch-teste
# adicione -fetch para atualizar os remotos dessas referências
```

O comando gera JSON com os SHAs e o patch para o agente analisar; `-fetch` não faz checkout nem merge. O patch é omitido quando ultrapassa o limite e o relatório indica revisão parcial.

### Sincronização entre dois PCs

Em cada PC, rode `memory-sync -init`. O comando cria `~/.config/agent-sync/config.json` caso não exista e imprime o caminho. Preencha `turso.url` com a URL `libsql://...` do mesmo banco Turso Cloud e `turso.token` com o token deste PC. O arquivo é criado com permissão `0600`, não é sobrescrito e deve permanecer fora do repositório. Para projetos sem remoto Git, configure um ID estável no mapa `projects`, usando o caminho local de cada PC como chave:

```json
{
  "turso": { "url": "libsql://seu-banco.turso.io", "token": "seu-token" },
  "projects": {
    "/home/voce/projetos/app": "app-principal"
  }
}
```

Projetos com `origin` ou `upstream` usam automaticamente um ID derivado do remoto. O MCP grava PC, caminho local e ID do projeto; buscas podem usar `project_dir` para resolver o mesmo projeto em PCs com caminhos diferentes.

```json
{
  "turso": {
    "url": "libsql://seu-banco.turso.io",
    "token": "seu-token"
  }
}
```

```bash
memory-sync -init
agent-sync-session ~/Documentos/agent-sync codex
# ou: agent-sync-session ~/Documentos/agent-sync claude
```

O wrapper exige checkout limpo, faz `git pull --ff-only`, atualiza a instalação das cinco CLIs quando o repositório muda e baixa as memórias antes de abrir a CLI. Ao sair, envia memórias persistentes e executa `git push` para commits feitos na sessão. Alterações sem commit não são enviadas; o wrapper não cria commits. O SQLite local fica em `~/.cache/agent-sync/memory.db`; memórias `scratch` não são sincronizadas. Para uso manual, rode `memory-sync -phase start` antes e `memory-sync -phase end` depois. Em caso de conflito, escolha a versão com `memory-sync -phase resolve-local -conflict <id>` ou `memory-sync -phase resolve-remote -conflict <id>` e repita a sincronização. A conexão com Turso Cloud ainda precisa ser validada na sua conta.

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

### Variáveis de ambiente

| Variável | Default | Onde aplicar | O que faz |
|---|---|---|---|
| `AGENT_SYNC_HOME` | (heurística) | `agent-sync`, `agent-sync-session` | Força o caminho da raiz do repositório. Útil quando o binário é chamado de fora do repo (CI, symlinks, testes). Tem prioridade sobre o `cwd` e o `os.Executable()`. |
| `AGENT_SYNC_DRY_RUN=1` | `0` | `agent-sync -apply` | Habilita modo dry-run (equivalente à flag `-dry-run`/`-n`): mostra o que seria feito sem escrever em disco. Útil para `make apply DRY_RUN=1`. |
| `AGENT_SYNC_PRETOOLUSE_VALIDATE=1` | (não persistido) | `shell-validate` hook | Liga o hook `shell-validate` (sinaliza comandos shell provavelmente inválidos antes da execução). `make apply` persiste automaticamente em `~/.zshrc`/`~/.bashrc`; sem essa env, o hook é no-op silencioso. |

---

## 🛡️ Ferramentas Inclusas
- **`ast-outline <arquivo>`**: Gera a estrutura de classes e funções com linhas correspondentes, economizando até 90% dos tokens de contexto.
- **`trace-strip <arquivo_ou_pipe>`**: Oculta frames irrelevantes de stack traces de bibliotecas externas.
- **`db-guardian -list`** / **`-profile <banco> -query "<sql>"`**: Valida queries em modo read-only (bloqueia mutações, avisa sobre `SELECT *` e injeta `LIMIT`). **Não abre conexão nem executa a query**; o perfil serve apenas para contexto. Credenciais aceitam indireção por ambiente (`${DB_PASSWORD}`).
- **`docs-fetch <url>`**: Baixa e cacheia uma página de docs e extrai texto (`-outline`, `-grep`, `-raw`, `-refresh`) para consulta de baixo token.
