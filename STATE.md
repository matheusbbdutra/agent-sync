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
