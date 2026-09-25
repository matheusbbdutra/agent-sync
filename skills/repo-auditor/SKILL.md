---
name: repo-auditor
description: "Audita o repositório em busca de débitos técnicos, oportunidades de refatoração e integridade do ambiente. Use when the user asks 'audite o repo', 'verifique débitos técnicos', 'audit repo', 'verifique a saúde do projeto', ou antes de grandes refatorações."
---

# Repo Auditor

Skill read-only para diagnóstico de integridade, identificação de débitos técnicos e checagem de paridade do ambiente `agent-sync`.

## Procedimento

1. **Integridade Geral**: Execute `agent-sync doctor` (ou `agent-sync doctor -json`) para inspecionar os 6 subsistemas canônicos (skills_index, skills_lint, schemas, opencode_plugins, binaries, agents_md).
2. **Qualidade de Skills**: Se houver alertas, execute `agent-sync skills lint` para isolar divergências em frontmatter, nomenclaturas ou gatilhos.
3. **Catálogo & Órfãs**: Execute `agent-sync skills index -orphans` para identificar skills no disco não registradas no catálogo curado.
4. **Histórico Recente**: Avalie `git log --oneline -20` para verificar se commits recentes introduziram pendências estruturais não documentadas.

## Saída Esperada

Apresente um resumo conciso em tabela Markdown com:

| Severidade | Área | Recomendação | Esforço | Evidência |
|---|---|---|---|---|
| Crítica / Alta / Média / Baixa | Subsistema afetado | Ação concreta recomendada | Baixo / Médio / Alto | Comando, path ou diff |

## Limitações (Read-Only)

- Não realize alterações automáticas sem confirmação explícita.
- Não commite nem execute comandos destrutivos.
- Mantenha o foco em diagnósticos acionáveis e pragmáticos.
