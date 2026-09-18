---
name: arch-context-check
description: "Checklist obrigatório antes de propor arquitetura ou refatoração, prevenindo over-engineering."
---

# Checklist antes de sugerir arquitetura/design

Responda em PT-BR, objetivo (CLAUDE.md global). O objetivo desta skill não é substituir os skills especializados — é garantir que eles e a memória de decisões sejam **sempre consultados primeiro**, em vez de responder com conhecimento genérico quando o repo já tem algo curado e específico pra esse contexto.

## Ordem de consulta (nessa sequência, antes de responder)

1. **Skill especializado pertinente ao problema** — carregue o que for relevante antes de opinar:
   - `ddd` — modelagem de domínio, bounded contexts, agregados, linguagem ubíqua.
   - `design-patterns` — GoF e padrões de domínio (Repository, Unit of Work, Specification).
   - `object-calisthenics` — as 9 regras (indentação, sem else, coleções de primeira classe, etc.).
   - `architecture-patterns` / `architect-review` / `architecture-decision-records` — Clean/Hexagonal Architecture, fronteiras, ADRs.
   - Não responda de memória genérica do modelo quando um desses já cobre o caso — a curadoria existe justamente pra evitar isso.

2. **`memory-mcp`** (`search_memory` pelo módulo/domínio/decisão em questão) — verifique se já existe uma decisão registrada relacionada. Uma sugestão nova não pode contradizer uma decisão já tomada sem que isso seja discutido explicitamente com o usuário (não silenciosamente ignorado).

3. **O código/regra já existente** — leia o trecho real (`path:line`) antes de propor mudança. Isso já é regra global anti-alucinação (nunca supor comportamento sem verificar); aqui reforça especificamente pra decisões de arquitetura, onde o custo de estar errado é mais alto.

## Como estruturar a resposta

Ao dar a sugestão, deixe explícito de onde vem cada parte:
- O que vem do skill especializado (cite qual).
- O que vem de uma decisão anterior registrada (cite `search_memory` e o nome da memória).
- O que é análise nova sobre o código atual (cite `path:line`).

Isso evita passar uma opinião genérica como se fosse fundamentada, e deixa claro pro usuário o que já foi decidido versus o que está sendo proposto agora.

## Fechando o ciclo

Se a sugestão for aceita e virar uma decisão de arquitetura real (não só uma sugestão pontual), grave de volta no `memory-mcp` via `store_memory` (`type: project`, `scratch: false`, proveniência correta). Isso é o que permite a próxima consulta (seção 2) encontrar essa decisão depois — sem esse passo, o ciclo não fecha e a memória não cresce.

## Quando não vale o checklist completo

- Dúvida pontual e de baixo risco (ex.: "esse nome de variável está bom?") não precisa do fluxo inteiro — bom senso sobre o tamanho/risco da decisão, mesmo critério do `spec-planner` pra saber quando planejar antes de agir.
- Se não existir skill especializado nem memória relevante para o caso, prossiga com a análise direta do código — não invente uma fonte que não existe.
