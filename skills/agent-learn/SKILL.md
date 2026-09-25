---
name: agent-learn
description: "Analisa o histórico de execuções recentes em session-event.jsonl e decisões em STATE.md para identificar padrões recorrentes de falhas e propor melhorias pragmáticas nas regras e skills. Use when the user asks 'aprenda com os erros', 'extraia lições', 'learn from failures', 'quais padrões podemos melhorar', ou ao final de sessões longas."
---

# Agent Learn

Skill analítica para minerar histórico de sessões, refinar heurísticas e propor evolução contínua das regras globais (`rules/global-rules.md`) e ferramentas.

## Procedimento

1. **Leitura de Eventos**: Analise as últimas entradas de `.agent-sync/session-event.jsonl` (ou execute `agent-sync event stats`).
2. **Padrões de Erro**: Identifique:
   - Loops de ferramentas ou retries frequentes.
   - Hipóteses refutadas sem documentação de causa raiz.
   - Comandos bloqueados por guards (`bash-guardian`, `shell-validate`).
   - Gaps de suporte cross-CLI entre as 5 ferramentas.
3. **Decisões Recentes**: Cruze com `STATE.md` (seção Decisões D-N) para evitar propor o que já foi formalizado.
4. **Proposta Pragmática**: Formule 1-2 propostas enxutas (novo bullet em `rules/global-rules.md` ou ajuste de gatilho em skill existente).

## Formato de Saída

```markdown
### Padrões Detectados
- **Padrão X**: Ocorrência em sessões Y e Z. Causa raiz comprovada.

### Proposta de Ação (Pragmática)
- **Arquivo Alvo**: `rules/global-rules.md` (ou skill específica)
- **Diff Sugerido**:
  ```diff
  + <novo princípio conciso>
  ```
- **Justificativa**: Evita reincidência com mínimo impacto operacional.
```

## Diretrizes

- Nunca aplique mudanças silenciosamente; apresente a proposta e aguarde aprovação do usuário.
- Priorize sempre o pragmatismo e a simplicidade (mínimo texto funcional).
