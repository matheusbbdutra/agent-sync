---
name: agent-delegate
description: Critérios para decidir se e para qual CLI/modelo delegar uma tarefa (Claude Code, Codex, Antigravity/agy, OpenCode, Cursor), usando o modo adequado ao perfil de permissão de cada harness e a memória compartilhada (`memory` MCP). Use quando o usuário pedir para "mandar isso pra outro agente/modelo", ao avaliar se uma tarefa é barata/mecânica o suficiente para rodar num modelo mais econômico, ou ao decidir se vale delegar em vez de executar você mesmo.
---

# Delegação entre agentes/CLIs

Responda em PT-BR, objetivo (CLAUDE.md global). Esta skill não cria orquestração automática — é um checklist para decidir se/para onde/como delegar. Infra de suporte: memória compartilhada (`memory-mcp`) + modos print/session que cada CLI já expõe.

## Modos de execução

| Modo | Quando | Como |
| --- | --- | --- |
| **`print`** | tarefa mecânica, fácil de auditar, alvo permissivo, sem expectativa de prompt de permissão | `claude -p`, `opencode run`, `agent -p`, e só excepcionalmente `agy -p` |
| **`session`** | alvo pedirá permissão, tarefa longa, ou você pode precisar intervir | CLI **interativo** (ex.: `agy -i` / `--prompt-interactive`, ou sessão tmux detachable). O orquestrador **não** bloqueia no TTY — avisa o usuário a attachar se precisar aprovar |

Não trate todos os CLIs como equivalentes no passo “rodar via Bash”. O critério decisivo é o **perfil de fricção de permissão** do harness, não só o modelo.

## Perfil de permissão por alvo

| Alvo | Perfil | Comando print | Uso em delegação |
| --- | --- | --- | --- |
| **OpenCode** | permissivo (deny-list; só padrões perigosos pedem `ask`) | `opencode run "<mensagem>"` | **default** para headless / trabalho mecânico |
| **Cursor Agent** | médio (`--force` / `--yolo` só se o usuário autorizar) | `agent -p "<prompt>"` (`--workspace <path>` se precisar) | ok em workspace confiado |
| **Claude Code** | médio/alto + classificador de segurança | `claude -p "<prompt>"` | ok em print; **não** orquestrar `agy` a partir dele |
| **Antigravity (agy)** | allow-list estreita + sandbox; aprovar uma vez **não** generaliza (matching quase literal) | `agy -p` / `--print` | **não** usar print por padrão → modo `session`. Print só se a allowlist já cobrir exatamente as tools/comandos da tarefa |
| **Codex** | médio | modo non-interactive do Codex | ok se a tarefa couber no sandbox dele |

### Por que o agy “pede de novo”

No Antigravity CLI, `permissions.allow` costuma ser lista de entradas estreitas (`command(ls)`, comando literal longo, MCP específico). Variação de flag/path/tool → novo prompt. Em `--print` não há humano → nega e quebra a delegação. OpenCode, por contraste, deixa passar o que não está na deny-list — por isso é o alvo padrão de delegação automática.

### Limitação: Claude Code → agy headless

Testado em 2026-09-12: orquestrar `agy --print` (ou com `--dangerously-skip-permissions` / `--mode accept-edits`) a partir do Claude Code esbarra no classificador do próprio Claude (“Create Unsafe Agents”), não só no agy. Não insistir com variações de flag na mesma sessão — o classificador escala o bloqueio. Se o usuário quiser agy nessa situação: abrir agy interativo (modo `session`) ou delegar via OpenCode.

`--dangerously-skip-permissions` / `--mode accept-edits` no agy: só com o usuário na frente e consciente — **nunca** como padrão desta skill.

## Antes de delegar

1. Consultar `memory-mcp` (`search_memory` / `list_memories`) e embutir o contexto no prompt do alvo.
2. Escolher alvo pelo **perfil de permissão** (tabela acima), depois pelo modelo/custo.
3. Escolher modo `print` vs `session` conforme a tabela.
4. Default quando a tarefa é mecânica e o alvo não importa: **`opencode run`**.

Ao gravar memórias: `agent` = `claude-code` | `codex` | `antigravity` | `opencode` | `cursor`.

## Critérios para decidir se delega

1. **Risco/reversibilidade.** Mecânica e fácil de verificar → pode ir para modelo mais barato (de preferência OpenCode em print). Arquitetura, segurança, irreversível → mantenha no CLI da sessão.
2. **Custo de verificação.** Só delegue se checar o resultado for mais barato do que fazer você mesmo.
3. **Capacidade necessária.** Tooling exclusivo de um CLI restringe o alvo (independente de custo).
4. **Contexto na memória.** Sem contexto suficiente no `memory-mcp`, o alvo barato reconstrói do zero — resolva a lacuna antes de delegar.
5. **Escolha explícita.** Você ou o usuário escolhe alvo/modo na hora; não automatizar “qual modelo pra qual tarefa”.

## Fluxo de uma delegação

1. Consultar `memory-mcp` pelo contexto relevante.
2. Escolher **alvo** (perfil de permissão) + **modo** (`print` / `session`).
3. Montar o prompt já com o contexto embutido.
4. Executar via **`delegate-run`** (preferível ao Bash cru):

```bash
# default mecânico
delegate-run start --target opencode --mode print --prompt "…contexto + tarefa…"

# agy / precisa de aprovação humana
delegate-run start --target agy --mode session --prompt "…" --workspace /path/do/proj --name slug
delegate-run attach <id>          # se pedir permissão
delegate-run tail <id> -f         # observabilidade
delegate-run status <id>          # stalled_hint se log parado (default 600s)
delegate-run watch <id> --interval 5 --timeout 1800   # poll até done/failed
delegate-run watch <id> --fail-on-stall               # sai 2 se stalled
delegate-run result <id>          # lê DELEGATE_RESULT do log
delegate-run result <id> --raw    # JSON completo
```

   - **`print`**: bloqueia até terminar; revise o log/`run.log` antes de aceitar.
   - **`session`**: sobe tmux detached (`delegate-<id>`); o orquestrador **não** fica no TTY — avise o usuário a `attach` se o alvo pedir permissão.
5. O `delegate-run` **injeta** no prompt o contrato de saída. O filho deve imprimir:

```text
=== DELEGATE_RESULT ===
{"ok":true,"summary":"…","artifacts":[],"notes":""}
=== END_DELEGATE_RESULT ===
```

   Opcional: `store_memory(name="delegate-<id>", …)` para persistir além do log.
6. Pai: `delegate-run result <id>` (não parsear prosa do modelo). Se `has_result=false`, trate como incompleto.
7. Se nascer decisão/aprendizado útil além do resultado → `store_memory` permanente com proveniência correta. Rascunho/teste → `scratch: true`.

Dados em `~/.cache/agent-sync/delegates/<id>/` (`manifest.json`, `prompt.txt`, `run.log`, `result.json`, `exit_code`). Script: `scripts/delegate-run.sh` (`make install` → `~/.local/bin/delegate-run`).

## Apagar memória

`delete_memory` só remove o que foi gravado com `scratch: true`. Permanentes exigem remoção manual deliberada.

## Quando NÃO delegar

- Overhead (montar prompt + rodar + revisar) maior que fazer direto.
- Decisão irreversível/sensível — não terceirize a decisão em si.
- Alvo = agy e a tarefa claramente vai disparar tools fora da allowlist **e** ninguém pode ficar no modo `session` → mude o alvo (OpenCode) ou faça você mesmo.
- Sem memória/contexto suficiente e produzi-lo já é caro.
