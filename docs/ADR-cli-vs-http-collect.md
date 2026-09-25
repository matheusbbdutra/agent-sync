# ADR: Caminho paralelo de coleta de MR/PR via `glab`/`gh` (`mr-collect-cli`)

**Status**: Aceito (promovido por D-64 em 2026-09-23; GitLab + GitHub entregues, 11 testes -race verdes — 5 gitlab_test + 6 github_test; schema `reviewChange` permanece implícito, versionamento alinhado a `ADR-schema-output-versionado.md` já Aceito)
**Data**: 2026-09-21
**Decisor**: Matheus Dutra
**Tags**: mr-review, providers, glab, gh, parallel-implementation, read-only, ddd-adapter

## Contexto

O agent-sync tem hoje um único caminho de coleta de MR/PR: o binário `mr-review-local` (`tools/cmd/mr-review-local/`). Esse caminho foi consolidado na decisão **D-19** (2026-09-19, registrada em `STATE.md:32`) que fixou três propriedades deliberadas:

1. **Compatibilidade com GitLab 12 self-hosted.** Suporte a instâncias antigas onde `/diffs` não existe — fallback explícito para `/changes` com sinalização de `overflow:true` como `Truncated`.
2. **Zero dependências externas do provedor.** `net/http` puro em vez de `gitlab.com/gitlab-org/api/client-go` — economia de uma dep + controle explícito de paginação via header `X-Next-Page` + sem CGO.
3. **Read-only-por-construção.** O binário não usa shell para interagir com o provedor; toda chamada é HTTP autenticado via header `PRIVATE-TOKEN` lido de env var cujo **nome** (não o valor) persiste no config.

Em paralelo, o `PLANO-AGENTE-ANALISE-MR.md` prevê 6 entregas incrementais; entrega #3 (adaptador GitHub) e #5 (publicadores opcionais) ainda não foram feitas.

### Problema concreto que motivou esta ADR

Três fricções reais do D-19, todas fora do escopo "compat v12":

1. **DX para self-hosted moderno.** Usuário com GitLab 16+ self-hosted tem que manter `-config` JSON com `base_url`, `version`, `token_env`. `glab auth login --hostname <url>` é uma linha e já cuida do token. Configuração é fricção evitável.
2. **Cobertura GitHub inexistente.** Hoje o agente `mr-reviewer` cobre `local` e `gitlab`; nenhum provider GitHub. Usuários com PRs no GitHub não têm revisão automatizada via este pipeline.
3. **Princípio de adapter.** O `ChangeProvider` (`tools/cmd/mr-review-local/providers.go:10`) já está desenhado como interface — adicionar um adapter paralelo é a evolução natural, não um desvio.

A tentação natural seria **substituir** `mr-review-local` por uma versão que use `glab`/`gh`. Esta ADR argumenta contra essa substituição e a favor de um caminho paralelo.

## Decisão

Adotar um **caminho paralelo** de coleta via CLI, sem mexer no `mr-review-local`. Quatro decisões de shape:

### Decisão 1 — Paralelo, não substituto

`mr-review-local` (HTTP puro) permanece como está. O novo coletor `mr-collect-cli` (`tools/cmd/mr-collect-cli/`) é **uma alternativa** que o usuário escolhe por invocação. Razões:

- **Compatibilidade GitLab 12 self-hosted preservada.** `glab` segue a API v4 moderna; substituí-lo perderia a propriedade D-19 mais valiosa. Manter dois caminhos deixa o usuário escolher conforme a instância.
- **Zero regressão.** `mr-review-local` tem 20 testes verdes (`gitlab_test.go`, `main_test.go`) e foi validado em smoke real contra instância GitLab. Reescrever é jogar fora cobertura.
- **Read-only-por-construção é mais barato manter em dois binários do que refatorar.** A allowlist (Decisão 3) é uma `map[string]bool`; redeclarar a invariante em outro arquivo é mais barato que acoplar via abstração comum.

### Decisão 2 — Binário Go novo, não skill pura, não wrapper shell

Três caminhos foram considerados; o escolhido é o binário Go (`tools/cmd/mr-collect-cli/main.go`):

- **Skill pura (bash + jq)**: rejeitado. Depende de `glab`, `gh` E `jq` no PATH; redaction ficaria ad-hoc; sem cobertura de testes Go.
- **Binário Go novo (escolhido)**: reaproveita `internal/secretscan` (já validado em `mr-review-local:19`); emite o **mesmo JSON `reviewChange`** (Decisão 4); testes cobrem a allowlist sem precisar de `glab` real.
- **Provider dentro de `mr-review-local`**: rejeitado. Mistura paradigmas (HTTP puro + exec de CLI) num só binário; viola SRP; tests suites teriam que coexistir.

### Decisão 3 — Read-only-por-construção via allowlist hardcoded

`glab` e `gh` são CLIs de uso geral — aceitam `merge`, `approve`, `close`, `push`, etc. Permitir exec livre quebra a propriedade D-19. Solução: allowlist hardcoded validada **antes** do `exec.Command`:

```go
// tools/cmd/mr-collect-cli/gitlab.go
var gitLabSubcommands = map[string]bool{
    "view": true, // glab mr view
    "diff": true, // glab mr diff
}
```

Rede de proteção: dois testes garantem que a allowlist não cresça acidentalmente (`TestValidateGlabSubcommand_Allowlist` falha em qualquer sub fora de `{view, diff}`; `TestGitLabSubcommands_OnlyReadOnly` falha se o tamanho do mapa mudar sem atualizar a lista esperada). Decisão consciente é adicionar; bug é não atualizar teste.

`gh` receberá a mesma estrutura (`githubSubcommands`) quando o provider for implementado.

### Decisão 4 — Mesmo schema `reviewChange`, sem versionamento novo

O `mr-collect-cli` emite exatamente o mesmo struct Go que `mr-review-local`:

```go
// tools/cmd/mr-collect-cli/main.go:38
type reviewChange struct {
    Provider, Repository, BaseRef, HeadRef string
    BaseSHA, HeadSHA, MergeBaseSHA         string
    FetchedRemotes                         []string
    Stat, Diff                             string
    Truncated, Redacted                    bool
}
```

Isso permite que o agente `cli-mr-reviewer` consuma o output sem parsing adicional, e que o agente `mr-reviewer` (existente) também pudesse consumi-lo futuramente se decidir.

**Diferenças intencionais em relação ao `mr-review-local`**:
- `MergeBaseSHA` fica vazio: `glab mr diff` não retorna merge-base de forma trivial. Agente infere se precisar.
- `Stat` fica vazio: `glab mr diff` é texto puro, sem `--stat` confiável.
- `FetchedRemotes` fica vazio: `glab` faz a chamada remoto direto, sem `git fetch` local.

Versionamento de schema (1.0, MAJOR/MINOR) **fica para entrega posterior**, alinhada com `ADR-schema-output-versionado.md`. Hoje o contrato é implícito mas estável; promover para versionado é ADR separada quando o formato parar de oscilar.

## Consequências

### Positivas

- **DX self-hosted moderno.** `glab auth login --hostname https://gitlab.empresa.com` e o coletor descobre host via `git remote get-url origin`. Zero JSON config por projeto.
- **Cobertura GitHub desbloqueada.** Quando `github.go` for adicionado, GitHub sai do backlog sem reescrever o que já funciona.
- **D-19 preservado.** `mr-review-local` intocado, 20 testes verdes mantidos. Compatibilidade GitLab 12 self-hosted segue como caminho oficial.
- **Read-only-por-construção replicado.** Allowlist + rede de proteção contra regressão silenciosa = mesma garantia D-19, agora também para CLI.
- **Schema compartilhado.** `reviewChange` é o ponto de acoplamento. Adicionar campo é mudança coordenada nos dois binários (ou abstração comum em ADR futura).

### Negativas / trade-offs

- **Dois binários para manter.** Sincronizar allowlist, struct `reviewChange`, lógica de redaction. Aceitável: ambos compartilham `internal/secretscan`.
- **Testes com `glab`/`gh` reais não são possíveis em CI sem credencial.** Cobrimos só allowlist + parse (puros). Smoke real depende do usuário ter `glab`/`gh` autenticado. Mitigação: tabela de troubleshooting em `skills/cli-mr-review/SKILL.md`.
- **Acoplamento a uma CLI no PATH.** Se container/CI for minimal, `mr-collect-cli` falha com mensagem clara (`glab: executable file not found in $PATH`). Usuário instala `glab` ou volta para `mr-review-local`.
- **Read-only depende de revisão humana.** Allowlist é guard rail, não prova formal. Se um dia `glab` introduzir `mr view --approve` que confunde com `view`, allowlist atual não pega. Mitigação: teste de allowlist valida **exatamente** os subcomandos, não flags.
- **Custo de adicionar provider novo.** Hoje GitHub; amanhã Bitbucket, Gitea, etc. Cada um = arquivo novo + allowlist nova + testes. Aceitável: interface `ChangeProvider` torna isso mecânico.

## Alternativas consideradas

| Alternativa | Por que rejeitada |
|---|---|
| **Substituir `mr-review-local` por `glab`/`gh` direto** | Perde compat GitLab 12 self-hosted (D-19). Read-only vira por convenção, não construção. Reescrita de 20 testes verdes sem ganho claro. |
| **Adotar SDK Go do GitLab** (`gitlab.com/gitlab-org/api/client-go`) | Já rejeitado em D-19. CGO, dep externa, controle de paginação delegado. |
| **Skill pura bash + jq** | Dependência de 3 CLIs no PATH (glab/gh/jq); redaction ad-hoc; sem testes Go; sem type safety. Custo de manutenção maior. |
| **Wrapper shell que filtra args antes do `glab`** | Lógica de segurança em shell = footgun. Citações, expansões, defaults do shell corroem a garantia. |
| **Provider Go dentro de `mr-review-local`** | Mistura HTTP puro + exec CLI num binário. SRP violado. Suites de teste distintas teriam que coexistir. |
| **ADR formal de schema versionado antes desta entrega** | Prematuro. Hoje o contrato é implícito mas estável (struct Go compartilhado). Promover para versionado com MAJOR/MINOR + changelog é trabalho de mapeamento, não decisão. Fica para depois que `github.go` estabilizar o shape. |

## Decisões revisadas

(nenhuma — ADR em estado Proposto na primeira iteração.)

## Evidência / Implementação

Esta ADR é Proposta — código já entregue, mas ainda sem revisão explícita para virar Aceito.

| Arquivo | Mudança |
|---|---|
| `tools/cmd/mr-collect-cli/main.go` | CLI: `-provider=gitlab\|github`, `-repo`, `-mr-iid`, `-host` opcional, `-max-bytes` (default 256KB), `-timeout` (default 30s). Emite JSON `reviewChange`. |
| `tools/cmd/mr-collect-cli/gitlab.go` | Provider GitLab: allowlist hardcoded (`view`, `diff`), parse de remote via 2 regex (SCP + HTTPS), exec `glab mr view --output json` + `glab mr diff`, redaction via `internal/secretscan`, truncamento com flag. |
| `tools/cmd/mr-collect-cli/gitlab_test.go` | 4 testes com `-race`: parse SCP (3 casos), parse HTTPS (3 casos, incluindo porta), rejeição (4 entradas malformadas), allowlist (10 subcomandos proibidos + 2 permitidos), rede de proteção contra regressão silenciosa. |
| `agents/cli-mr-reviewer.md` | Agente read-only, `invokes: [code-reviewer, security-auditor, architecture-reviewer]`. Documenta quando preferir vs `mr-reviewer` e o que muda. |
| `skills/cli-mr-review/SKILL.md` | Tabela de troubleshooting (6 sintomas), como o host self-hosted é detectado, checklist de verificação (`which mr-collect-cli`, `glab auth status`, `gh auth status`). |
| `docs/guides/architecture-flow.md` | 3 referências cirúrgicas: linha 128 (binários em ordem alfabética), linha 525 (matriz 5xN), linha 548 (fluxos críticos como `12a` para preservar numeração 13–18). |
| `Makefile` | `mr-collect-cli` adicionado ao target `build`. |
| `STATE-events-mr-collect-cli.md` | Registro do turno (entregas, validação, pendências). |

### Validação atual

- `cd tools && go test -race ./...` — **verde** (todos os pacotes, incluindo `mr-review-local` intacto).
- `make build` — **verde** (16 binários compilados).
- Smoke 1: `mr-collect-cli -provider gitlab -repo . -mr-iid 1` no repo atual (remote `github.com`) → erro previsível `glab mr view falhou (host=github.com): exec: "glab": executable file not found in $PATH`, exit 2.
- Smoke 2: `mr-collect-cli -provider gitlab -repo <tmp com remote git@gitlab.empresa.com:...>` → detectou `gitlab.empresa.com` corretamente via parse SCP.

### Critério de aceite (para mudar de Proposto → Aceito)

- [ ] Revisão humana do `git diff` dos arquivos entregues.
- [ ] Decisão explícita sobre implementar `github.go` nesta ADR ou em ADR separada (escopo cresce).
- [ ] Confirmação de que manter dois binários é a estratégia de longo prazo (não vale fundir via abstração comum).
- [ ] Decisão sobre versionamento de schema `reviewChange`: promover para 1.0 versionado ou manter implícito.

## Limites conhecidos

1. **GitLab 12 self-hosted não suportado por este caminho.** Usuário com instância antiga deve usar `mr-review-local` (HTTP puro). Documentado em `agents/cli-mr-reviewer.md` e `skills/cli-mr-review/SKILL.md`.
2. **Provider GitHub ainda não implementado.** `github.go` está no backlog (`STATE-events-mr-collect-cli.md`); `-provider=github` retorna erro explícito hoje.
3. **Smoke real com `glab` autenticado não foi feito.** Ambiente de teste não tem `glab` instalado nem credencial GitLab. Cobertura de testes é só allowlist + parse (puros).
4. **Read-only-por-construção é guard rail, não prova formal.** Allowlist valida subcomandos; flags como `--approve` em sub permitidos não são bloqueadas. Mitigação parcial: hoje nenhum sub na allowlist aceita flag destrutiva.
5. **Schema `reviewChange` implícito.** Adicionar campo novo hoje exige sincronização manual entre os dois binários. ADR-schema-output-versionado.md é a base para resolver isso.

## Próximos passos

1. **Aceitar a ADR** após revisão (status Proposto → Aceito).
2. **Wire-up no `architecture-flow.md`**: confirmar que a entrada `12a` da tabela de fluxos críticos (linha 549) reflete a cobertura real depois do `github.go`.
3. **Smoke real com `glab` e `gh` autenticados** em máquina do desenvolvedor. Adicionar `bin/mr-collect-cli` ao `make test-integration` se o projeto tiver esse target.
4. **ADR de schema versionado aplicada a `reviewChange`** quando o formato estabilizar (após `github.go`). Promover `reviewChange` para `1.0` em `schemas/mr-collect-cli.json` + `schemas/mr-review-local.json` (ou unificar).
5. **README.pt-BR.md e README.md**: documentar `mr-collect-cli` na seção de binários disponíveis e na matriz de cobertura por CLI.

## Referências

- **D-19 (STATE.md:32, 2026-09-19)** — decisão original de HTTP puro + zero deps + read-only-por-construção para GitLab. Base desta ADR.
- **PLANO-AGENTE-ANALISE-MR.md** — plano das 6 entregas incrementais; esta ADR entrega a #3 parcial (GitHub ainda falta) e adiciona um caminho alternativo ao D-19.
- **ADR-schema-output-versionado.md** — define o framework de schema versionado; aplicar a `reviewChange` é próximo passo desta ADR.
- **ADR-opencode-granular-permissions.md** — `invokes:` declarado; `cli-mr-reviewer` herda padrão do `mr-reviewer` (`[code-reviewer, security-auditor, architecture-reviewer]`).
- **`tools/cmd/mr-review-local/providers.go:10`** — interface `ChangeProvider` que motivou o desenho paralelo.
- **`tools/internal/secretscan/scan.go`** — redaction compartilhada entre os dois binários.
- **Documentação oficial glab**: <https://gitlab.com/gitlab-org/cli> (referência para flags `--hostname`, `--output json`).
- **Documentação oficial gh**: <https://cli.github.com/manual/> (referência futura para `github.go`).
