# STATE — mr-collect-cli (glab/gh provider paralelo)

## Status

**ENTREGA #3 CONCLUÍDA** — `github.go` + `github_test.go` entregues; `-provider=github` wirado em `main.go`; smoke real com `gh` autenticado confirma fluxo de erro previsível.

## Objetivo

Adicionar um coletor de MR/PR alternativo ao `mr-review-local` que delega para
`glab` (GitLab) e `gh` (GitHub) em vez de fazer HTTP puro. **Sem mexer no
`mr-review-local` / D-19** — é um caminho paralelo, o usuário escolhe.

## Decisões já alinhadas (confirmadas pelo usuário)

- **Paralelo, não substituto.** `mr-reviewer` (HTTP puro, compat GitLab 12, zero
  deps, read-only-por-construção) continua intocado. D-19 preservado.
- **Binário Go novo** `tools/cmd/mr-collect-cli/` — não skill pura nem wrapper
  shell. Justificativa: reusa `internal/secretscan` e o contrato JSON
  `reviewChange` existente.
- **Self-hosted GitLab via parse de `git remote get-url origin`.** Zero config
  extra. Se `glab` não estiver autenticado nesse host, falha com mensagem clara
  apontando `glab auth login --hostname <url>`.
- **Read-only por construção via allowlist de subcomandos no Go.** Hardcoded:
  `glab mr view`, `glab mr diff`, `glab mr list`, `gh pr view`, `gh pr diff`,
  `gh pr list`. Qualquer outro subcomando → erro antes do `exec.Command`.
- **Mesmo JSON `reviewChange`** na saída — assim `mr-reviewer` (ou novo agente)
  consome sem alteração de prompt.

## Estrutura prevista

```
tools/cmd/mr-collect-cli/
  main.go          # CLI: -provider, -repo, -mr-iid|pr-iid, -host opcional, -timeout
  gitlab.go        # exec glab via allowlist + parse de remote
  github.go        # exec gh via allowlist (análogo)
  gitlab_test.go   # testes com httptest? não — testes de allowlist/parse via fake exec
  github_test.go   # idem
```

## Mudanças aplicadas neste turno

- **`tools/cmd/mr-collect-cli/main.go`** — CLI: `-provider=gitlab|github`, `-repo` (path do checkout), `-mr-iid`, `-host` opcional, `-max-bytes` (default 256KB), `-timeout` (default 30s). Emite JSON `reviewChange` (mesmo schema de `mr-review-local`) em stdout, erros em stderr com exit code 2.
- **`tools/cmd/mr-collect-cli/gitlab.go`** — provider GitLab:
  - Allowlist hardcoded: `view`, `diff` (validação pura via `validateGlabSubcommand`).
  - Parse de remote via 2 regex: SCP (`git@host:path`) e HTTPS (`https?://host/...`).
  - `git -C <repo> remote get-url origin` para descobrir host; `-host` força override.
  - `glab mr view --output json` para metadados + `glab mr diff` para patch.
  - Redaction via `internal/secretscan` antes de truncar (evita cortar chave de API no meio).
- **`tools/cmd/mr-collect-cli/gitlab_test.go`** — 4 testes verdes com `-race`:
  - `TestParseRemoteHost_SCP` (3 casos).
  - `TestParseRemoteHost_HTTPS` (3 casos, incluindo porta).
  - `TestParseRemoteHost_Rejeitados` (4 entradas malformadas).
  - `TestValidateGlabSubcommand_Allowlist` (rede de proteção contra regressão silenciosa da allowlist).
  - `TestGitLabSubcommands_OnlyReadOnly` (fail se alguém adicionar sub sem atualizar lista esperada).
- **`agents/cli-mr-reviewer.md`** — agente read-only, `invokes: [code-reviewer, security-auditor, architecture-reviewer]`. Workflow documenta quando preferir este agente vs `mr-reviewer` e o que muda.
- **`skills/cli-mr-review/SKILL.md`** — troubleshooting table, detecção de host self-hosted, checklist de verificação.
- **`Makefile`** — `mr-collect-cli` adicionado ao target `build`.
- **`docs/guides/architecture-flow.md`** (turno 2, após commit 04301c8 do usuário) — 3 referências cirúrgicas adicionadas:
  - Linha 128 (tabela de binários): `mr-collect-cli` em ordem alfabética, antes de `mr-review-local`.
  - Linha 525 (matriz 5xN): linha própria marcada ✅ em todas as 5 CLIs.
  - Linha 548 (fluxos críticos): linha `12a` sem renumerar 13–18 (sufixo preserva numeração original do commit do usuário).
- **`tools/cmd/mr-collect-cli/github.go`** (turno 3) — provider GitHub análogo ao `gitlab.go`:
  - Allowlist hardcoded: `view`, `diff` (paridade com GitLab — mesma forma de proteção read-only-por-construção).
  - Parse de remote via 2 regex (SCP + HTTPS) cobrindo github.com E self-hosted GitHub Enterprise (`git@gh.empresa.com:...`).
  - `gh pr view <num> --json number,baseRefName,headRefName,baseRefOid,headRefOid,url` para metadados.
  - `gh pr diff <num> --color never` para patch.
  - Redaction via `internal/secretscan` antes de truncar (mesmo padrão).
  - Self-hosted: documentado como dependência de `GH_HOST` env var (`gh` não tem flag `--hostname`).
- **`tools/cmd/mr-collect-cli/github_test.go`** (turno 3) — 5 testes verdes com `-race`:
  - `TestParseGithubRemoteHost_SCP` (3 casos, incluindo self-hosted).
  - `TestParseGithubRemoteHost_HTTPS` (3 casos, incluindo self-hosted).
  - `TestParseGithubRemoteHost_Rejeitados` (4 entradas malformadas).
  - `TestValidateGithubSubcommand_Allowlist` (13 subcomandos proibidos + 2 permitidos).
  - `TestGithubSubcommands_OnlyReadOnly` (rede de proteção contra regressão silenciosa).
  - `TestParseGithubRemoteHost_NaoAceitaGitlab` (sentinela contra mix-up de providers).
- **`tools/cmd/mr-collect-cli/main.go`** — wire-up: `-provider=github` agora chama `collectGitHub` em vez de retornar erro.
- **`STATE-events-mr-collect-cli.md`** — este arquivo.

## Validação

- `cd tools && go test -race ./...` — **verde** (todos os pacotes, incluindo `mr-review-local` intacto).
- `make build` — **verde** (16 binários compilados).
- Smoke gitlab: `mr-collect-cli -provider gitlab -repo . -mr-iid 1` no repo atual (remote `github.com`) → erro previsível: `glab mr view falhou (host=github.com): exec: "glab": executable file not found in $PATH`.
- Smoke gitlab parse: `mr-collect-cli -provider gitlab -repo <tmp com remote git@gitlab.empresa.com:...>` → detectou `gitlab.empresa.com` corretamente.
- Smoke github (turno 3): `mr-collect-cli -provider github -repo . -mr-iid 42` → `gh pr view falhou: exit status 1 — stderr gh: GraphQL: Could not resolve to a PullRequest with the number of 42.` (`gh` está autenticado no repo, erro chega via stderr conforme projetado).
- Smoke github erro repo: `mr-collect-cli -provider github -repo /tmp/nonexistent -mr-iid 1` → `git remote get-url origin falhou: exit status 128` (erro previsível).

## Pendente pós-turno

- README.pt-BR.md + README.md: documentar `mr-collect-cli` na seção de binários disponíveis.
- ADR `docs/ADR-cli-vs-http-collect.md`: revisar e mover de Proposto para Aceito.
- Smoke real com `glab` instalado contra um repo GitLab de teste (não factível neste ambiente sem credenciais).
