# ADR — Secret guard cross-CLI (Pre+PostToolUse) com deny-list de paths

- **Status**: Proposto
- **Data**: 2026-09-25
- **Decisor**: agente + usuário (sessão de retomada pós-incidente D-83/A-63)
- **Fonte**: D-83 + A-63 + checkpoint pós-incidente
  `reference/checkpoint-incidente-seguranca-ses_f2a16545bffeeCcxIFwZ04VVDa-20260925`
  (memory-mcp, gravado em 2026-09-25).

## Contexto

`hooks/secret-guard.pretooluse.sh` e `hooks/secret-guard.posttooluse.sh`
existem desde a sessão original (`ses_f2a16545bffeeCcxIFwZ04VVDa`,
2026-09-25), mas:

1. **Não há wiramento declarado** em `internal/hooks/apply_table.go`
   (tabela `standardHooks`, 26 entradas em 2026-09-25 — nenhuma se
   chama `secret-guard`) nem em `cursorManagedHooks` (10 entradas em
   `hooks_cursor_apply.go`). O hook é órfão de runtime.
2. A regex do `secret-guard.pretooluse.sh` cobre **JWT, AWS access
   key, GitHub PAT**. O Discord BOT token (formato `<b64>.<b64>.<b64>`
   com ~70 chars) **não é coberto** — gap explícito registrado em
   `reference/discord-bot-token-formato-base64-base64-base64-...`.
3. O hook bloqueia *segredo literal nos args*. Não bloqueia *path
   apontando para arquivo com segredo* (caso `cat ~/.zshrc`, `Read
   .env` etc.) — o `secret-guard.posttooluse.sh` pega o output, mas o
   agente já viu o conteúdo em memória de trabalho antes da redação.
4. Push posterior foi rejeitado pelo GitHub secret scanning
   (registrado em checkpoint pós-incidente). Houve reset do projeto
   por parte do usuário.

## Decisão

### §1. Hooks ampliados — deny-list de path é defesa primária

**Princípio**: o objetivo não é catalogar todos os formatos de segredo
existentes (JWT, AWS, Discord, Slack, GCP, GitLab PAT, ...). O objetivo é
**garantir que o agente nunca leia o conteúdo de um arquivo que possa
conter segredo**, independentemente do formato. A deny-list de paths vira a
defesa primária; o regex de literal vira cinto + suspensório (defesa em
profundidade, mas não fonte de verdade).

`hooks/secret-guard.pretooluse.sh` reescrito em duas camadas:

**Camada 1 — deny-list de path (PRIMÁRIO)**: extrai `tool_name` do input
JSON e os campos de path conforme a tool, depois bloqueia quando o path
casa qualquer padrão da deny-list.

- Deny-list (case-insensitive; uma `grep -oE` unificada por padrão).
  Categorias:
  - **Variáveis de ambiente / dotenv**: `.env`, `.env.*`, `.envrc`,
    `*.env`
  - **Chaves privadas / certs**: `*.pem`, `*.key`, `*.p12`, `*.pfx`,
    `id_rsa*`, `id_ed25519*`, `id_ecdsa*`
  - **Credenciais nomeadas**: `credentials*`, `*credentials.json`,
    `secrets.*`, `*secret*.json`, `*secret*.yaml`, `*secret*.yml`
  - **Configs de tooling com token inline**: `~/.netrc`, `~/.npmrc`,
    `~/.pypirc`, `~/.pgpass`, `~/.aws/credentials`,
    `~/.aws/config`, `~/.config/gh/hosts.yml`,
    `~/.docker/config.json`, `~/.kube/config`, `~/.terraformrc`
  - **Shells com segredos exportados**: `~/.zshrc`, `~/.bashrc`,
    `~/.bash_profile`, `~/.profile`, `~/.zprofile`, `~/.zshenv`,
    `~/.bash_env`
  - **Pastas proibidas**: `~/.ssh/`, `~/.aws/`, `~/.gnupg/`,
    `~/.config/gh/`, `~/.docker/`, `~/.kube/`, `zscaler/` (em
    qualquer profundidade do path)
- Gate por `tool_name`:
  - **Read / Edit / Write / MultiEdit / NotebookEdit**: extrai
    `tool_input.file_path` e/ou `tool_input.notebook_path` e/ou
    `tool_input.path`. Bloqueia se o path casa.
  - **Grep / Glob**: extrai `tool_input.path` (cwd da busca) e
    `tool_input.pattern`. Bloqueia se o **cwd** casa; não tenta
    match dentro do pattern (falso-positivo alto).
  - **Bash**: extrai o primeiro argumento que pareça path (regex
    `(/|~|\.)[^ ]+`) e bloqueia se casa. Comando `cat ~/.zshrc` é
    coberto; `echo $SECRET` (sem path) escapa — coberto pela
    Camada 2.
- Saída: `{"hookSpecificOutput": {"hookEventName": "PreToolUse",
  "permissionDecision": "deny", "permissionDecisionReason":
  "path denylist: <padrao-casado>"}}`.

**Camada 2 — regex de literal nos args (SECUNDÁRIO)**: hook atual
preservado como está (JWT/AWS/GitHub PAT). Justificativa: secrets
passados inline em comando (`export X=...`, `curl -H 'Bearer ...'`)
não têm path; só Camada 2 pega. Falsos-positivos são aceitáveis
(agente corrige com env var de verdade).

`hooks/secret-guard.posttooluse.sh` ganha **redação por nome de
arquivo** além da redação de literal:

- Se `tool_name` for Read/Edit/Cat/etc. e o path do arquivo casa a
  deny-list, substituir **todo o `tool_output`** por
  `<REDACTED:FILE_IN_DENYLIST>` (em vez de tentar redacionar
  pedaços). Justificativa: se o arquivo está na deny-list, o agente
  não deveria ter visto o conteúdo (Camada 1 deveria ter barrado);
  redação total garante defesa em profundidade.
- Regex de literal (JWT/AWS/GitHub PAT) preservada como hoje para
  conteúdo de arquivos **fora** da deny-list.

Samples adicionados a `hooks/secret-guard.test.sh` (atualmente 111L):

- **Camada 1 path-deny** (8 casos): `Read .env`, `Read ~/.aws/credentials`,
  `Read ~/.ssh/id_rsa`, `Bash cat ~/.zshrc`, `Grep path=~/.ssh`,
  `Edit file_path=*.pem`, `Bash 'echo $TOKEN' (escapa Camada 1)`,
  `Bash chmod 600 ~/.ssh/id_rsa (NÃO bloqueia — operação legítima)`.
- **Camada 2 literal** (3 casos): preserva os 3 atuais
  (JWT/AWS/GitHub).
- **Post-tooluse redação total** (2 casos): `Read` de `.env` →
  `<REDACTED:FILE_IN_DENYLIST>`; `Read` de path fora da deny-list
  contendo JWT → `<REDACTED:JWT>`.
- Samples com literais **sintéticos** (RFC 7515 genérico
  `header.payload.signature` só com caracteres `[A-Za-z0-9+/=_-]` e
  comprimentos válidos, mas zeros substituindo bytes — não usar
  nenhum secret real como sample).

### §2. Wiramento cross-CLI (matriz 5xN, conforme ADR
   `docs/ADR-trilha-c-cobertura-cross-cli.md`)

| CLI | Pre-ToolUse | Post-ToolUse | Estado |
| --- | --- | --- | --- |
| Claude Code | wirar `PreToolUse` matcher `*` | wirar `PostToolUse` matcher `*` | ✅ |
| Codex | wirar `PreToolUse` matcher `*` (mesmo template do A-14 PreCompact) | wirar `PostToolUse` | ✅ |
| OpenCode v2 | **gap 🟡** — depende de plugin TS específico (A-64+) | **gap 🟡** | 🟡 |
| Antigravity | **gap 🟡** — PreInvocation proxy com `if`-filter (A-64+) | **gap 🟡** | 🟡 |
| Cursor | **gap ⛔ aceito** — `preToolUse` não é evento gerenciado pelo agent-sync; defesa ad-hoc via `.cursorrules` é responsabilidade do usuário | `postToolUse` wirar via `cursorManagedHooks` (mesmo template dos demais) | ⛔ |

**PLAYBOOK-V** (reavaliação do ⛔ Cursor): reler
`https://docs.cursor.com/agent-hooks` a cada 30 dias (ou quando o
usuário informar mudança no Cursor); testar se Cursor expõe
`preToolUse`/`read`/`write` como evento wirado. Mudança upstream →
esta ADR vira filha com cronograma de wirar.

## Consequências

**Positivas:**

- Defesa em 2 camadas (path-deny em Pre, redação em Post) reduz
  drasticamente chance de novo vazamento por leitura acidental de
  arquivo com segredo nas 2 CLIs wiradas.
- Wiramento cross-CLI garante que o secret-guard funciona em
  Claude Code + Codex, que são wirados hoje via bash.

**Negativas:**

- 2 wiramentos bash adicionais em `internal/hooks/apply_table.go`
  (hoje 26 entries). Risco de regressão em `agent-sync -apply` é
  baixo (mesmo padrão testado em A-14 / A-16).

**Trade-offs assumidos:**

- Discord BOT regex é permissiva (pode casar JWTs com `+/-_` no
  payload sem prefixo `eyJ`); tratável no test runner com caso
  adicional (input JWT não falso-positiviza pelo prefixo `eyJ`).
- Cobertura completa de provedores (Slack, Notion, AWS secret
  access key `aws_secret_*`, GCP service-account JSON) fica para
  `A-64` (S-0.3). Não é objetivo desta entrega.
- Wiramento de OpenCode v2 / Antigravity / Cursor também fica
  para `A-64+`, na forma de plugin TS novo, PreInvocation proxy ou
  via `.cursorrules` (decisão ad-hoc do usuário).

## Implementação

Sequência de commits granulares (1 commit por peça, ordem importa):

1. `docs(adr) ADR-secret-guard-cross-cli.md Proposto` — este arquivo.
2. `feat(hooks) secret-guard.pretooluse.sh deny-list by path` +
   `feat(hooks) secret-guard.posttooluse.sh discord regex`.
3. `test(hooks) secret-guard.test.sh casos sintéticos deny-list +
   discord` (com chamamento explícito no `main` do runner, lição do
   checkpoint).
4. `feat(apply) syncSecretGuardHook em apply_table.go para Claude e
   Codex` (calls em `hooks_apply.go` usando
   `syncHookCommandAtEvent` no evento `PreToolUse` e `PostToolUse`;
   matcher `*`).
5. `docs(state) D-83 + A-63 wrap-up` (STATE.md +
   `.agent-sync/session-state.json`). Wrap-up apenas.

**Reversibilidade:** todos os commits acima podem ser revertidos
isoladamente sem quebrar wiramentos existentes (o hook bash atual
não interfere em nada).

## Promoção Proposto → Aceito

| # | Critério | Estado |
| --- | --- | --- |
| 1 | Matriz 5xN preenchida com 2 aceitos e 2 gaps e 1 impossível documentados (Claude ✅, Codex ✅, OpenCode v2 🟡, Antigravity 🟡, Cursor ⛔ com PLAYBOOK-V) | ✅ (nesta ADR) |
| 2 | `bash hooks/secret-guard.test.sh` verde (4/4 = pre e post cobrem deny-list + discord sintético) | ❓ pendente |
| 3 | `go test ./...` verde (regressão nos wiramentos) | ❓ pendente |
| 4 | Wiramento real instalado via `make install` em Claude Code e Codex | ❓ pendente |
| 5 | Smoke real: hook bloqueia path sintético `~/.zshrc`-like em runtime de uma CLI | ❓ pendente |

Quando 4/5 critérios passarem em 2/2 smoke sessions → Proposto vira
**Aceito**.
