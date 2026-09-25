# Convenção de Sprints

**Status**: Proposta (esboço 2026-09-24, ses_f2d156b9cffek5gYKsdKGlMW96).
**Origem**: decisão do usuário em 2026-09-24 sobre taxonomia/workflow.

## O que é uma Sprint aqui

Uma Sprint é um **recorte de trabalho com começo, fim e entregável verificável**. Cada
Sprint mora em `docs/sprints/sprint-<id>.{json,md}` e tem um **mapa** (JSON, ≤100 linhas)
e um **detalhado** (MD, prosa livre).

A unidade de trabalho da Sprint é o **item referenciado** (`ref: A-N | ADR-XXX | I-N`),
nunca conteúdo duplicado. Toda decisão histórica fica onde já está (ADR, D-N, git log) —
a Sprint só **aponta**.

## ID

Sequencial: `S-0.1`, `S-0.2`, `S-0.3`. Sem SemVer, sem data, sem semântica de versão —
a ordem cronológica é o que vale. Sprint fechada vai para `docs/sprints/archive/`.

## Estrutura do JSON (mapa)

```json
{
  "id": "S-0.X",
  "title": "≤60 chars, frase nominal",
  "status": "in_progress | done | cancelled",
  "starts": "YYYY-MM-DD",
  "ends": "YYYY-MM-DD | null",
  "scope": "1 frase: o que esta sprint promete",
  "items": [
    {"ref": "A-N | ADR-XXX | I-N", "phase": "Proposta|Analise|Investigacao|Complexibilidade|Re-analise|Refinamento|Final", "verdict": "Aceita|Re-Proposta|Descartada|null", "note": "opcional"}
  ],
  "non_goals": ["o que NÃO entra nesta sprint"],
  "links": {
    "details": "docs/sprints/sprint-<id>.md",
    "branch": "feat/..."
  }
}
```

## Estrutura do MD (detalhado)

Prose livre, mas com seções convencionais (não obrigatórias):

- **Contexto** — por que esta Sprint existe
- **Items em ordem cronológica** — links pra D-N, A-N, ADR-XXX
- **Decisões adiadas** — o que ficou de fora + motivo
- **Riscos / gaps conhecidos** — links pra I-N
- **Métricas de saída** — como saber que a Sprint acabou (ex.: "0 duplicações no JSON")

## Regras (convenção, NÃO hook)

| Regra | Como aplicar |
|---|---|
| JSON ≤150 linhas | Na revisão de fim de Sprint, se estourar, subdividir em `S-X.Y` e `S-X.Y+1`. Limite original era 100 (S-0.1); bumpeado para 150 em S-0.2 (D-74, 2026-09-24) porque a Sprint consolidou reavaliação gaps (A-23/24/25/28) + catalogação ai-memory (A-51/52/53 + A-54..A-61 = 11 itens) em escopo único. |
| MD ≤500 linhas | Idem |
| Itens referenciados | Sempre `ref: ID-já-existente`, nunca duplicar descrição |
| `status` da Sprint | `in_progress` enquanto houver item sem `Final`; `done` quando todos `Final`; `cancelled` com motivo no MD |
| Mudar item de fase | Editar o JSON. Diff no PR mostra a mudança. |
| Mover fase de item | **Manual ou delegado** (não automatizar). Perguntar antes de mover. |

## Fases (workflow de 7 etapas por ADR)

Aplicar **a cada ADR nova** que entre na Sprint. Não é tool automática — é rito.

1. **Proposta** — nasce em `docs/adr-staging/` com template curto (~30 linhas)
2. **Análise** — investigar se pergunta já foi respondida antes (grep + `search_memory`)
3. **Investigação** — probe empírica com `path:line`
4. **Complexibilidade** — análise de esforço vs retorno (3 linhas máx)
5. **Re-análise** — releitura crítica sob luz da evidência
6. **Refinamento** — reescrever no formato longo, mover pra `docs/ADR-XXX-<slug>.md`
7. **Final** — Aceita / Re-Proposta (com data) / Descartada (com motivo)

## Gatilhos anti-monstro (convenção revisada no fim de cada Sprint)

1. **ADR > 250 linhas** → candidata a quebra em ADR-mãe + ADR-filhas
2. **Sprint JSON > 100 linhas** → subdividir
3. **> 5 ADRs Propostas/Re-Propostas em aberto simultaneamente** → trava: revisar antigas antes de criar nova

A trava #3 é a única candidata futura a hook automático se o usuário mudar de ideia.
Hoje (2026-09-24) **fica como convenção**.

## Quem lê o quê

- **Agente** (no início da sessão): lê **só o JSON** da Sprint ativa. Decide o que atacar.
- **Usuário** (em revisão): lê o **MD** detalhado. Aceita mudança de fase editando 1 linha no JSON.
