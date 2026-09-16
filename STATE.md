# STATE — agent-sync

> Fonte de verdade para retomar o trabalho entre sessões/compactions.

## Diagnóstico de hooks — 2026-09-16

- Status: implementação concluída no repositório; configuração global pendente de ressincronização.
- Sintoma: duas falhas `invalid pre-tool-use JSON output` e duas `invalid post-tool-use JSON output` ao usar ferramentas; sem stack trace no relato.
- Evidência: `~/.codex/config.toml:213` e `:225` habilitam protect-mcp e review-agent-governance. Ambos os `hooks/hooks.json` no cache dos plugins chamam protect-mcp@0.7.4 evaluate/sign diretamente, sem adaptação ao protocolo Codex.
- Causa: CLI instalado em `~/.npm/_npx/98c3fd9639547f82/node_modules/protect-mcp/dist/cli.js:11150` (evaluate) e `:11196` (sign) emite JSON de domínio, não a resposta de hook esperada. Reprodução isolada com Node, telemetria desativada e recibos temporários: evaluate retorna exit 0 e `{"allowed":true,"reason":"no_policy_configured"}`; sign retorna exit 0 e campos signed/artifact_type/request_id. JSON sintaticamente válido não garante compatibilidade de schema.
- Segurança: protect.cedar e review-governance.cedar ausentes no projeto; comandos usam --fail-on-missing-policy false. Não confiar nesses hooks como gates de política neste estado.
- Achado adicional: hooks/context-guard-nudge.sh:22, hooks/memory-nudge.sh:23 e hooks/agent-react-nudge.sh:22 omitem hookSpecificOutput.hookEventName nos lembretes; documentação oficial exige identificar o evento. Isso é separado do par recorrente dos plugins.
- Implementação: `hooks/codex-protect-mcp-adapter.sh` traduz evaluate para `permissionDecision` e silencia a saída de sign; `cmd/agent-sync/hooks.go` reescreve comandos dos plugins ao sincronizar o Codex. Nudges agora informam `hookEventName`.
- Próximo passo: executar `make sync` (ou sincronização equivalente) para atualizar a configuração global existente; validar cenários permitidos/negados e pós-execução.
- Fonte do contrato: https://developers.openai.com/codex/hooks (PreToolUse/PostToolUse).
- Verificação: leitura de configuração/código e execução isolada dos dois subcomandos; não reproduzida a sessão original completa. Durante a análise, receipts/ e review-receipts/ apareceram como não rastreados; não foram removidos.
- Regras ativas: não consultar .env; não expor segredos; não alterar gates de segurança nem criar commits nesta análise; preservar evidências e validar antes de concluir.

## Meta anterior

- Skill `agent-react` + hook `agent-react-nudge` + regra hipótese ≠ fato.
- Status: concluído (commit pendente de push se desejado)

## Decisões tomadas

- Nudge (não gate): threshold 15, `AGENT_SYNC_REACT_NUDGE_THRESHOLD`
- Garantia semântica plena exige camada extra (ex. `stop`/`afterAgentResponse` no Cursor) — não implementada ainda
- Hook global Cursor = `~/.cursor/hooks.json` (local); Cloud Agents só leem `.cursor/hooks.json` do repo

## Próximos passos

1. Push se desejado
2. Se barulho: subir threshold ou filtrar só tools de mutação
3. Avaliar hook `stop` no Cursor para reforço no fim do turno

## Proposta de observabilidade — 2026-09-16

- Estado atual: recibos JSONL existem, mas não há eventos estruturados próprios, duração, resultado normalizado, correlação ou métricas agregadas.
- Direção sugerida: logger JSONL local com redaction de input/output, campos `timestamp`, `event`, `cli`, `tool`, `session_id`, `status`, `duration_ms`, `error_code`; contadores derivados por uma CLI de inspeção, sem rede e sem dependência de Prometheus.
- Fases: (1) contrato de evento + redaction e escrita atômica; (2) instrumentar adaptador Codex e sincronização; (3) comando `agent-sync observability` para resumo de falhas/latência; (4) exportação opcional Prometheus/OpenTelemetry somente se houver necessidade operacional.
- Erros de hook: podem ser persistidos no mesmo fluxo, capturando exit code, stderr resumido, etapa (`PreToolUse`/`PostToolUse`), CLI, ferramenta, sessão e duração. Não registrar stdin bruto, comandos, argumentos, tool output ou segredos; aplicar limite de tamanho e rotação ao arquivo.
- Implementação adicional: `hooks/observe-error.sh` persiste eventos redacted em `~/.cache/agent-sync/hooks/errors.jsonl` (limite aproximado de 5 MiB/10 mil linhas); `agent-sync -observability` resume estágio e código. O adaptador registra exit codes e JSON inválido.

## Agente de análise de MR/PR — proposta — 2026-09-16

- Escopo: análise local, GitHub, GitLab SaaS e GitLab self-hosted v12.
- Arquitetura: contrato único `ChangeProvider`/`ReviewPublisher`, com adaptadores Local (git diff), GitHub REST e GitLab REST; o agente `code-reviewer` permanece host-agnostic e recebe um pacote normalizado de diff/metadados.
- Compatibilidade GitLab v12: usar API REST v4 e endpoint de diffs compatível, evitando GraphQL e recursos recentes; comentários devem usar Notes API. Token somente via variável de ambiente/credential helper, nunca em logs ou STATE.
- Segurança: publicação de comentários fica opt-in; modo padrão somente leitura. Redaction antes de enviar diff a qualquer modelo/serviço.
- Plano: MVP local + pacote normalizado; depois GitHub/GitLab com dry-run; por fim publicação idempotente e CI.

## Bloqueios / perguntas abertas

- OpenCode: nudge best-effort até issue upstream #13574

## Revisão do plano de análise de MR/PR — 2026-09-16

- Estado: análise concluída; plano e implementação não alterados.
- Evidência: `/vox/dev-tools/go.mod:16` usa `gitlab.com/gitlab-org/api/client-go` v0.128.0. `/vox/dev-tools/app/services/git_service_remote.go:2027-2063` usa `ListMergeRequestDiffs` (`/diffs`) com fallback HTTP manual para `/changes`; o SDK local já fornece `GetMergeRequestChanges` para `/changes` (`merge_requests.go:543-548`).
- Correção necessária no plano: `docs/PLANO-AGENTE-ANALISE-MR.md:36` chama `/diffs` de rota legada do GitLab 12. O endpoint novo foi implementado em 2022/2023 (https://gitlab.com/gitlab-org/gitlab/-/merge_requests/104561); usar `/changes` para v12 e `/diffs` quando disponível, com detecção de capacidade.
- Riscos do exemplo: somente primeira página de `/diffs` (`git_service_remote.go:2027-2045`); erro do fallback pode virar lista vazia (`:2048-2073`); TLS desativado no cliente (`:28-31`). Não reproduzir no novo adaptador.
- Limite de verificação: não houve consulta à instância GitLab 12 nem execução de testes. Próximo passo, se solicitado: corrigir o plano e definir testes de compatibilidade.
- Regras ativas: não consultar `.env`, não expor segredos, não criar commits; citar fontes e validar afirmações.

## Configuração genérica do GitLab no plano — 2026-09-16

- Decisão do usuário: o agent-sync é geral; a configuração da instância GitLab não deve conter valores da Vox fixados no código.
- `docs/PLANO-AGENTE-ANALISE-MR.md` agora exemplifica JSON com `base_url`, `version` (`auto` ou versão explícita), `token_env` e `timeout_seconds`. Projeto e IID são parâmetros por análise; token não fica no JSON.
- Estratégia prevista: SDK Go isolado no adaptador; `/diffs` em versões com suporte e `/changes` para GitLab 12; fallback somente para rota inexistente, sem ocultar erros de autenticação ou rede. TLS verificado e paginação obrigatória.
- Estado: apenas plano alterado; não há implementação de agente de MR no código atual. Próximo passo: implementar quando o escopo completo do agente for definido.
- Regras ativas: não consultar `.env`, não expor segredos, não criar commits; verificar antes de afirmar funcionamento.

## Prioridade do MVP local e critérios de revisão — 2026-09-16

- Direção do usuário: implementar primeiro a comparação Git local, inclusive fork/upstream, com base e head explícitos como `upstream/branch-teste` e `origin/branch-teste`; buscar remotos quando solicitado.
- `docs/PLANO-AGENTE-ANALISE-MR.md` detalha seleção de refs, fetch opcional sem checkout/merge, merge-base, leitura de contexto adjacente e análise de segurança/impacto.
- Critérios: distinguir defeito preexistente, regressão introduzida e risco ainda não confirmado; exigir condição de disparo, antes/depois e evidência `arquivo:linha` antes de afirmar quebra de fluxo; declarar revisão parcial se faltar contexto.
- Estado: plano atualizado; não há implementação do agente de análise de MR/PR. Próximo passo: construir provider local e CLI conforme escopo do plano.
- Regras ativas: não consultar `.env`, não expor segredos, não criar commits; não executar código não confiável da branch durante a coleta; testes apenas quando houver implementação real.

## Agente de revisão Git local — implementação — 2026-09-16

- Estado: MVP local implementado em `agents/mr-reviewer.md` e `tools/cmd/mr-review-local/`; `Makefile` inclui o binário. `syncAgents` existente gera o agente para Claude, Codex, Antigravity, OpenCode e Cursor. Ainda não há coleta GitHub/GitLab nem publicação.
- Coleta: refs base/head explícitas, `fetch` opcional dos remotos indicados, SHA de base/head/merge-base, `git diff` sem diff externo ou textconv, timeout, limite de patch, redaction regex existente, JSON. Patch truncado é omitido e marcado como revisão parcial.
- Falha encontrada em teste: `TestCollectRejectsInvalidRefAndReportsTruncation` esperava truncamento, mas o `limitedBuffer` incorporava `bytes.Buffer`; `os/exec` podia usar o `ReadFrom` promovido e contornar `Write`. Corrigido por composição com campo privado `buffer`; teste passou. Lição: buffers limitados não devem expor interfaces de cópia que ignorem o limite.
- Verificação: `go test ./...` e `go vet ./...` passaram nos módulos raiz e `tools`; build do novo binário em `/tmp` passou. `git diff --check` passou. Instalação global nas CLIs não executada.
- Turso: memória atual usa `go-libsql` local (`tools/internal/agentmemory/store.go:105-115`). A documentação oficial informa que embedded replicas de `go-libsql` enviam escritas ao remoto imediatamente; sincronização somente no início/fim exige avaliar `tursogo` com `Push`/`Pull` ou outro fluxo. Ainda não alterado. Pergunta pendente ao usuário: sincronizar arquivos do repositório, memórias do MCP ou ambos?
- Regras ativas: não consultar `.env`, não expor segredos, não criar commits; não executar código não confiável da branch durante coleta; não tratar revisão parcial como completa.

## Avaliação Turso Cloud — 2026-09-16

- Esclarecimento ao usuário: Turso Cloud seria o banco remoto das memórias; SQLite/libSQL local permanece em cada PC. Código, agentes, regras e skills são arquivos e continuam sob Git.
- Fonte oficial: https://docs.turso.tech/sdk/go/reference — embedded replica `go-libsql` envia escritas ao remoto; `tursogo` oferece `Push`/`Pull` explícitos. Porém o schema atual usa SQLite FTS5 (`tools/internal/agentmemory/store.go:69-80`) e a compatibilidade Turso Database informa FTS5 não suportado: https://github.com/tursodatabase/turso/blob/main/COMPAT.md. Portanto migração direta do banco local para `tursogo` não está validada.
- Opção a detalhar: manter busca FTS5 local e sincronizar apenas registros de memória persistentes com uma tabela remota Turso/libSQL no início/fim da sessão, com conflitos detectados; não sincronizar o arquivo SQLite bruto. Ainda não implementado nem testado contra Turso Cloud; faltam decisão de escopo e credenciais remotas para validação final.

## Sincronização entre dois PCs — 2026-09-16

- Decisão confirmada pelo usuário: código/regras/skills via Git; memórias persistentes via banco remoto somente no começo/fim da sessão. Turso é transporte/armazenamento, sem pesquisa remota; SQLite FTS5 continua local em cada PC. SQLiteCloud é alternativa possível, mas integração da extensão SQLite-Sync com o driver Go atual e FTS5 não foi validada.
- Fonte Turso: https://docs.turso.tech/libsql (libSQL é fork compatível); https://github.com/tursodatabase/turso/blob/main/COMPAT.md (Turso Database novo não implementa FTS5, usa Tantivy). Isso não impede enviar registros para tabela comum, pois o remoto não usa FTS. Fonte SQLiteCloud: https://docs.sqlitecloud.io/docs/sqlite-sync-getting-started.
- Implementação local: `tools/internal/agentmemory/sync.go` sincroniza registros `scratch=0`, detecta conflito por hash/base local, usa transação e atualização condicionada no push, impede envio de padrões conhecidos de segredo; `tools/cmd/memory-sync` expõe fases start/end e resolução explícita; `scripts/agent-sync-session.sh` faz pull Git, aplica configurações das cinco CLIs, baixa memórias, executa a CLI e depois envia memórias/push Git. `Makefile` instala os novos comandos; READMEs documentam o uso. O banco local permanece em `~/.cache/agent-sync/memory.db`.
- Verificação local: `go test ./...` nos módulos raiz e tools passou; `bash -n scripts/agent-sync-session.sh` passou. Teste de integração usa dois SQLite locais e um libSQL local como remoto; conexão com Turso Cloud real não foi testada por falta de conta/credenciais. Não ler `.env` nem registrar tokens.
- Verificação adicional: `go vet ./...` nos módulos raiz e tools, `git diff --check` e `gofmt -l` dos arquivos Go alterados passaram. Próximo passo externo: validar em conta Turso real antes de afirmar operação remota concluída. Sem commit solicitado.
- Regras ativas: não consultar `.env`; não expor segredos; não fazer commit; não tratar teste local como teste da nuvem; manter `STATE.md` atualizado.

## Configuração Turso em JSON local — 2026-09-16

- Pedido do usuário: ler a configuração do Turso em arquivo sob a pasta de configuração do sistema e sempre criar o modelo quando estiver ausente; o usuário preencherá o token.
- Implementação: `tools/internal/agentmemory/config.go` usa `os.UserConfigDir()/agent-sync/config.json`, cria diretório `0700` e arquivo `0600` com `turso.url` e `turso.token` vazios, sem sobrescrever arquivo existente. Recusa arquivo não regular ou legível por grupo/outros. `memory-sync -init` cria e informa o caminho sem conectar ao banco. `memory-sync -phase ...` chama `LoadRemoteConfig`; wrapper não exige mais variáveis de ambiente.
- Arquivo local criado por `go run ./cmd/memory-sync -init`: `/home/matheusdutra/.config/agent-sync/config.json`, permissão verificada `600`; conteúdo não inspecionado para preservar credenciais. O token ainda precisa ser preenchido pelo usuário. O JSON fica fora do repositório.
- Testes: `go test ./...` nos módulos raiz e tools, `go vet ./...` em tools, `bash -n` do wrapper e `git diff --check` passaram. Testes de config cobrem criação, permissão, preservação e leitura. Conexão Turso real não testada.
- Regras ativas: não consultar `.env`, não expor token, não commitar segredo, não afirmar validação remota sem teste.

## Teste Turso real — falha de autorização — 2026-09-16

- Gatilho: `go run ./cmd/memory-sync -phase start` com a configuração local preenchida pelo usuário. Saída capturada e token redigido antes de mostrar. A fase `end` não foi executada.
- Erro resumido: Turso retornou HTTP 401 `Unauthorized: unauthorized access attempt on database: token does not have the permissions to access the database` durante `CREATE TABLE IF NOT EXISTS agent_sync_memories` em `Store.Pull`; não houve envio de memórias.
- Evidência: URL tem esquema `libsql`, token está presente e tem formato JWT, sem prefixo `Bearer`; arquivo tem permissão 0600. O servidor recebeu a tentativa e recusou autorização. A causa exata (token de outro banco, escopo/permissão insuficiente, token inválido para esta URL) não foi determinada; não tratar como bug de sincronização confirmado.
- Próximo passo: gerar um token de banco com acesso de escrita para o banco correspondente à URL (`turso db tokens create <nome-do-banco>`, sem `--read-only`), substituir `turso.token` no JSON local e repetir start/end. Não registrar token em logs, STATE ou repositório.
- Lição: validar token e URL do mesmo banco antes de concluir que a sincronização remota funciona; teste local com libSQL não cobre autenticação da nuvem.

## Teste Turso real — confirmação de persistência — 2026-09-16

- Após o usuário atualizar o token, `memory-sync -phase start` passou e `-phase end` informou 45 envios. Na segunda passagem, `start` informou 0 e `end` falhou com conflito anônimo `ca369a16454b`.
- Diagnóstico somente leitura, sem imprimir conteúdo: havia 45 memórias persistentes locais e 45 baselines, mas 0 registros na tabela remota. Uma transação de diagnóstico separada, inclusive com 45 inserts e leitura em outro processo, persistiu normalmente. Portanto o motivo específico do desaparecimento inicial dos 45 registros não foi comprovado; não atribuir falsamente ao driver/COMMIT.
- Falha comprovada no nosso fluxo: `Store.Push` marcava o baseline local após `Commit` sem reler o remoto; isso permitia reportar sucesso mesmo quando os registros não estavam lá. Corrigido com verificação remota dos hashes após commit e antes de atualizar baselines. Se o remoto perdeu um registro e o conteúdo local ainda coincide com o baseline, o envio agora o restaura. Teste `TestPushRestoresMissingRemoteRecord` adicionado.
- Verificação real após correção: primeira passagem `start=0`, `end=45`; segunda passagem `start=0`, `end=0`, ambas com exit 0. Nenhum token ou conteúdo de memória foi exibido. `go test ./...`, `go vet ./...`, `git diff --check` e `gofmt -l` passaram.
- Lição: resposta de sucesso do `Commit` não substitui confirmação de leitura remota antes de registrar o estado sincronizado. Causa da perda original permanece incerta; monitorar reincidência.
- Regras ativas: não consultar `.env`; não expor token/conteúdo de memória; não criar commit sem pedido; distinguir fato comprovado de hipótese.

- Instalação local após a correção: `make install` passou; `memory-sync` e `agent-sync-session` estão em `~/.local/bin`. Execução dos comandos instalados `memory-sync -phase start` e `-phase end` passou com 0 alterações nas duas fases.

## Escopo de projeto nas memórias — 2026-09-16

- Decisão do usuário: projetos sem remoto Git usam ID de projeto configurável. Projetos com `origin` ou `upstream` usam ID determinístico derivado do remoto normalizado.
- Implementação: `Memory` agora registra `PC`, `ProjectPath` e `ProjectID`; chave local/remota é `project_id + type + name`. `memory_sync_state_v2` e `agent_sync_memories_v2` preservam a tabela legada e migram registros antigos com projeto vazio. O MCP aceita `project_path`, `project_dir`, `project_id`, `pc` e `global`; exibe origem nos resultados e impede ambiguidade de nomes.
- Configuração sem remoto: `projects` no `~/.config/agent-sync/config.json`, com caminho local como chave e ID comum como valor. `project_dir` resolve o ID no PC atual.
- Verificação: testes de identidade Git em caminhos diferentes, ID configurável, colisão de nomes, filtros, sincronização entre dois PCs e migração remota passaram. `make install` passou. `memory-sync -phase start/end` no Turso real passou com 0 alterações após a migração.
- Limitação: memórias antigas recebem `project_id` vazio; não foram atribuídas automaticamente a um projeto para evitar associação incorreta. A ferramenta `store_memory` passa a exigir um projeto Git válido, ID configurado ou `global=true` para novas memórias.

## Diagnóstico de `Hook failed` código 127 — 2026-09-16

- Sintoma relatado: `Hook failed / hook exited with code 127`.
- Evidência: os hooks Codex ativos apontam para scripts no checkout (`~/.codex/hooks.json`); execução direta com payload vazio retornava exit 0 e não havia evento 127 no log redigido. O código 127 ocorre antes do corpo do script quando `#!/usr/bin/env bash` não encontra `bash` em um PATH restrito da CLI; por isso `observe-error.sh` não consegue registrar o erro. O adaptador `codex-protect-mcp-adapter.sh` também foi reproduzido localmente e retornou exit 0.
- Correção aplicada: hooks Codex `context-guard-nudge.sh`, `memory-nudge.sh`, `agent-react-nudge.sh`, `docs-cache.sh` e `codex-protect-mcp-adapter.sh` usam `#!/usr/bin/bash`, removendo a dependência do PATH para localizar o interpretador. Não alterado o conteúdo dos payloads nem os gates.
- Verificação: `bash -n` e execução dos quatro hooks com payload vazio passaram; `go test ./cmd/agent-sync` passou; `git diff --check` passou. A reprodução exata depende do ambiente da CLI, que não foi capturado no relato.
- Próximo passo: repetir a operação que gerou o erro. Se persistir, capturar o nome do hook e stderr da CLI; o próximo suspeito será um comando interno ausente no PATH restrito.
