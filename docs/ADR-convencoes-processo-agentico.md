# ADR — Convenções de processo agêntico (Proposta-Primeiro, Regra 3X, Modo Sticky)

- **Status**: Aceito
- **Data**: 2026-10-15
- **Decisor**: agente + usuário (sessão de consolidação pós-`bash-sudo-agents`)
- **Fonte**: `docs/ADR-reuso-bash-sudo-agents.md` §2 (origem: RULE-10/13/14 do repo
  externo `nickarab/bash-sudo-agents`); `rules/global-rules.md §3`
- **Tags**: processo, governança, convenção, sem-runtime, policy

## Contexto

O repo externo `nickarab/bash-sudo-agents` declara 15 regras constitucionais
(`security/policies.json`), mas só ~5 têm enforcement em código. Três delas tratam de
**processo** (não de segurança de comando) e capturam dor real que o `agent-sync` também
enfrenta:

1. **RULE-14 (Rito Proposta-Primeiro)** — o agente edita a própria regra quando não
   deveria; proposta e aplicação ficam indistinguíveis no chat.
2. **RULE-10 (Regra 3X)** — o agente insiste num caminho recusado, repetindo a mesma
   tentativa sem escalar.
3. **RULE-13 (Modo Sticky)** — troca de agente mid-sessão perde continuidade.

`rules/global-rules.md §3` já exige "não criar agent/hook/skill sem pedido explícito"
(anti-overengineering), o que é o mesmo espírito do RULE-14 — mas não estava nomeado
como convenção reutilizável.

## Decisão

Adotar os 3 como **convenção documentada, zero runtime** (nenhum hook, nenhum binário,
nenhum estado):

| Convenção | Regra | Já praticado? |
|---|---|---|
| **Proposta-Primeiro** | Propor regra/refactor = só texto no chat. Escrever em disco só com imperativo explícito (`aplique`, `faça`, `execute`). | Sim (espelha `global-rules.md §3`) |
| **Regra 3X** | 3 recusas consecutivas do mesmo guardrail → congela, resume o histórico e escala ao usuário em vez de insistir numa 4ª tentativa. | Parcial (não nomeado) |
| **Modo Sticky** | Sessão continua no agente corrente até `exit`/troca explícita. | N/A hoje (agent-sync não opera agente de sessão) |

**Aplicação**: as 3 entram em `rules/global-rules.md` como §7 "Convenções de Processo
Agêntico" (referência curta), com este ADR como fonte de detalhe. Não há código a escrever.

## Consequências

### Positivas

- Comportamento já praticado fica **nomeado e auditável** — onboarding e revisão deixam de
  depender de memória de sessão.
- Zero custo de runtime: nenhum hook novo, nenhum fork por tool call.

### Negativas / limitações

- Convenção sem enforcement é **honor-system**; só funciona porque o modelo lê
  `AGENTS.md`/`global-rules.md` todo turno. Alternativa (hook que conta recusas) foi
  rejeitada por ser estado extra para ganho marginal.
- "Modo Sticky" fica como convenção **latente** — não há caso de uso no `agent-sync` hoje.

## Reversibilidade

- Remover a §5 de `global-rules.md` e apagar este ADR. Nenhum código muda.

## Não escopo

- Implementar contador de recusas em hook (rejeitado — estado extra, ganho marginal).
- Portar `policies.json`/`meta_policies.json` com enforcement declarativo.
- META-03 do repo externo (esconder veto do usuário) — rejeitado em
  `docs/ADR-reuso-bash-sudo-agents.md §3`.

## Refs

- `docs/ADR-reuso-bash-sudo-agents.md` §2 — decisão de origem
- Repo externo: `nickarab/bash-sudo-agents` — `security/policies.json` (RULE-10/13/14)
- `rules/global-rules.md §3` (anti-overengineering) e §7 (convenções de processo)
