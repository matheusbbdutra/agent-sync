# Guia: `agent-sync doctor`

Diagnóstico automatizado e observabilidade de integridade do ambiente `agent-sync`.

## O que ele verifica

O comando `agent-sync doctor` agrega 6 verificações determinísticas de subsistemas:

| # | Check | Subsistema | Critério de Sucesso |
|---|---|---|---|
| 1 | `skills_index` | Catálogo de skills | `manifest.json` e `skills/` em paridade (sem órfãs nem ausentes) |
| 2 | `skills_lint` | Qualidade de skills | Todas as skills cumprem frontmatter YAML, `name` correto e gatilhos |
| 3 | `schemas` | JSON Schemas | 5 schemas compilam sem erro (`agent_tasks`, `precompact-snapshot`, `session-event`, `session-state`, `token-budget-status`) |
| 4 | `opencode_plugins` | Plugins OpenCode v2 | Todos os 11 plugins presentes em `~/.config/opencode/plugins/` e runtime Node/OpenCode OK |
| 5 | `binaries` | Ferramentas locais | 18 binários e scripts presentes e executáveis em `~/.local/bin/` |
| 6 | `agents_md` | Regras canônicas | `rules/global-rules.md` idêntico em SHA256 nos 5 targets (`claude`, `codex`, `antigravity`, `opencode`, `cursor`) |

## Uso

```bash
# Execução padrão (saída tabular para humanos)
agent-sync doctor

# Saída estruturada em JSON (para automações e hooks)
agent-sync doctor -json
```

## Códigos de Saída

- `0`: Todos os checks em `PASS` ou `WARN` (sem erros bloqueantes).
- `1`: Um ou mais checks em `FAIL`.
