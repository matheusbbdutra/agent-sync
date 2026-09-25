# Padrão Arquitetural de Hooks Multi-CLI (Agent Harness)

> **Referência Teórica**: Taxonomia ETCLOVG & HarnessFix (arXiv:2606.06324v2)  
> **Escopo**: Padronização dos eventos de ciclo de vida nas 5 CLIs suportadas (`Claude Code`, `OpenAI Codex`, `Google Antigravity`, `Cursor`, `OpenCode`).

---

## 1. Visão Geral

No `agent-sync`, os hooks formam a camada de **Harness** que envolve os modelos de linguagem. Eles operam em 4 estágios essenciais do ciclo de vida:

1. **Pre-Tool (Execution & Governance)**: Intercepta antes da execução para bloquear comandos destrutivos ou validar sintaxe.
2. **Post-Tool (Observability & Tooling)**: Captura saídas, indexa passos da janela deslizante, identifica status de ferramentas e cacheia documentações.
3. **Pre-Compact / SessionStart (Context)**: Protege a integridade da memória de trabalho, gerando sumários estruturados e restaurando handoffs entre sessões.
4. **Stop / Turn-End (Lifecycle & Verification)**: Valida alegações de conclusão contra a evidência real de mutação ou testes no trace (*State-Effect Alignment*).

---

## 2. Matriz de Equivalência de Eventos por CLI

| Estágio de Harness | Claude Code (`~/.claude/settings.json`) | OpenAI Codex (`~/.codex/hooks.json`) | Google Antigravity (`~/.gemini/config/hooks.json`) | Cursor Agent (`~/.cursor/hooks.json`) | OpenCode (`~/.config/opencode/`) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Pre-Tool** | `PreToolUse` (matcher: `Bash`) | `PreToolUse` (matcher: `Bash`) | `PreToolUse` | `beforeShellExecution` | `permission.bash` / `tool.execute.before` |
| **Post-Tool** | `PostToolUse` (matcher: `*`) | `PostToolUse` (matcher: `*`) | `PostToolUse` | `postToolUse` | `tool.execute.after` (Plugin TS) |
| **Handoff (Início)**| `SessionStart` (matcher: `.*`) | `SessionStart` (matcher: `.*`) | `PreInvocation` (no turno 1) | `sessionStart` | `session.created` |
| **Pre-Compact** | `PreCompact` | `PreCompact` (observacional) | — | `preCompact` | `experimental.session.compacting` |
| **Fim de Turno** | `Stop` (matcher: `*`) | `Stop` (matcher: `*`) | `Stop` (flat handler) | `stop` | `session.idle` |

---

## 3. Padrão de Contratos de Entrada e Saída

### A. Pre-Tool (Governança & Segurança)
* **Objetivo**: Proteger contra ações destrutivas (`rm -rf`, `force push`, mutações em `.env`).
* **Comportamento**:
  - **Claude Code**: Regras nativas em `permissions.ask` no `settings.json`.
  - **Cursor**: Script `bash-guardian.cursor.sh` retorna `{"permission": "ask"}`.
  - **Antigravity**: Script `bash-guardian.antigravity.sh` retorna `{"decision": "ask"}`.
  - **OpenCode**: Mapeado diretamente em `opencode.json` via lista declarativa `permission.bash`.
  - **Codex**: Executa o validador estático [shell-validate](file:///home/matheusdutra/Projects/agent-sync/bin/shell-validate) em `PreToolUse`.

### B. Post-Tool (Observabilidade & Context Window)
* **Objetivo**: Injetar alertas periódicos (nudges) e indexar a memória de trabalho ($K$ passos).
* **Payload de Saída**:
  - **Claude Code / Codex**:
    ```json
    {
      "hookSpecificOutput": {
        "hookEventName": "PostToolUse",
        "additionalContext": "[agent-sync] ..."
      }
    }
    ```
  - **Cursor**:
    ```json
    {
      "additional_context": "[agent-sync] ..."
    }
    ```
  - **Antigravity CLI**:
    Executa via `PostToolUse` ou lê via `transcriptPath` JSONL no passo correspondente (`step_index + 1`).

### C. Session Handoff (Restauração de Contexto)
* **Objetivo**: Carregar `.agent-sync/summary.md` e working memory ao iniciar chat novo.
* **Comportamento**:
  - **Claude / Codex**: `ctx-window handoff <cli>` emite `additionalContext` no evento `SessionStart`.
  - **Cursor**: `ctx-window handoff cursor` emite `additional_context` no evento `sessionStart`.
  - **Antigravity**: `ctx-window handoff antigravity` detecta `invocationNum == 1` no evento `PreInvocation` e injeta `ephemeralMessage` (sem sujar o transcript).

### D. Stop / Fim de Turno (Harness Trace Guard)
* **Objetivo**: Prevenir falsos sucessos e alucinações de conclusão (*HarnessFix*).
* **Verificação**:
  1. Extrai `transcript_path` do evento.
  2. Inspeciona se os passos recentes contêm mutação (`write/edit/replace`) ou comandos de teste/validação (`go test`, `pytest`, `bash`).
  3. Checa se houve erro de ferramenta não tratado (`[TOOL_STATUS: FAILED]` ou `exit 1`).
* **Payload de Saída**:
  - **Claude Code / Codex**:
    ```json
    {
      "hookSpecificOutput": {
        "hookEventName": "Stop",
        "additionalContext": "[agent-sync] false-success-guard: ..."
      }
    }
    ```
  - **Antigravity CLI**:
    ```json
    {
      "decision": "continue",
      "reason": "[agent-sync] ..."
    }
    ```
  - **Cursor**:
    ```json
    {
      "followup_message": "[agent-sync] ..."
    }
    ```

---

## 4. Filosofia de Design dos Hooks

1. **Nunca quebrar a CLI hospedeira**: Se o hook falhar internamente (erro de I/O, payload corrompido), ele deve logar no `stderr` e retornar `{}` com exit code 0.
2. **Advisory First (Sem gates destrutivos desnecessários)**: O papel do hook é guiar e injetar contexto corretivo no modelo (*nudge*), evitando bloqueios arbitrários.
3. **Baixo consumo de tokens & latência zero**: Binários em Go compilados nativamente executam em < 5ms, sem chamadas a LLMs externos no caminho crítico do hook.
