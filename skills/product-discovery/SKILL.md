---
name: product-discovery
description: "Use when descobrir e validar um problema de produto antes de construir: hipótese de produto, queda de retenção/churn, roteiro de entrevista com cliente, priorização com RICE/ICE e corte de roadmap, e quem decide o quê (DACI). Processo de discovery em 6 fases com portão de saída verificável e condição de matar a iniciativa. Não gera ideias em aberto (product-brainstorming) nem audita produto já pronto (gtp)."
---

# Product Discovery

Discovery responde **"o problema existe e vale resolver agora?"** — não "como implementar". O erro caro não é escolher a solução errada: é construir com confiança uma solução para um problema que ninguém confirmou. Cada fase abaixo termina em um artefato e um portão; sem o artefato, a fase não fechou.

Discovery não é ideação (`product-brainstorming`), não é teste de usabilidade de solução pronta e não substitui entrega — roda em paralelo, tipicamente 1-2 ciclos por trimestre ([discovery-process](reference/discovery-loop.md#quando-usar)).

## Use this skill when

- Existe uma hipótese de produto ("usuários abandonam por causa de X") e ainda não há evidência.
- Retenção, ativação ou churn caíram e a causa é desconhecida.
- É preciso montar roteiro e amostra de entrevista com cliente, ou revisar um roteiro que induz resposta.
- Vai-se decidir o que entra no roadmap e é preciso score defensável + dono da decisão.
- Uma iniciativa precisa de condição explícita de cancelamento antes de ganhar budget.
- Alguém pediu uma feature e é preciso separar pedido de problema.

## Do not use this skill when

- O objetivo é gerar ou expandir alternativas de solução → `product-brainstorming`.
- O produto já existe e o pedido é interrogá-lo do ponto de vista do usuário → `gtp`.
- A decisão é técnica (trade-off de arquitetura, escolha de biblioteca) → `architecture`, `backend`.
- A decisão é sobre medir qualidade de saída de LLM → `llm-eval`.
- O problema é gestão de execução (sprint, tarefa, prazo) — não é discovery.

## Instructions

Curto-circuito a fase só quando a saída dela já existe e está escrita.

1. **Enquadre o problema em uma frase falsificável** antes de qualquer entrevista: "Acreditamos que [persona] sofre com [problema] porque [causa raiz], levando a [consequência]." Sem persona, causa e consequência, não é hipótese — é assunto ([Phase 1](reference/discovery-loop.md#fase-1--enquadrar-o-problema)). Saída: 1 hipótese, 3-5 perguntas de pesquisa, critério de validação/invalidação.
2. **Defina amostra e roteiro antes de agendar**: quem (segmento e papel), quantos, e o que cada pergunta tenta falsificar. Roteiro com pergunta no futuro, hipotética ou de opinião sobre a ideia está errado ([biases](reference/discovery-loop.md#roteiro-e-vieses)).
3. **Conduza até saturação, não até o calendário**: saturação é a mesma dor aparecendo em 3+ entrevistas sem insight novo; na prática fecha em 5-7 entrevistas. Não saturou → mais 3-5, e registre esse custo ([Decision Point 2](reference/discovery-loop.md#fase-3--conduzir)).
4. **Separe evidência de opinião na síntese**: evidência é o que a pessoa fez ou pagou; "eu usaria" é opinião. Registre a dor com a contagem de entrevistas que a sustentam e o que ficou de fora ([Fase 4](reference/discovery-loop.md#fase-4--sintetizar)).
5. **Mapeie as 5 categorias de risco antes de escolher solução**: desejável, usável, viável, factível, ético — e diga qual evidência falta em cada uma. Suposição sem evidência e de alta importância é onde o experimento vai ([assumption map](reference/discovery-loop.md#fase-5--soluções-e-experimento)).
6. **Gere soluções como árvore, não como lista**: 1 outcome → 3 oportunidades → 3 soluções por oportunidade → 1 experimento por solução escolhida ([OST](reference/discovery-loop.md#fase-5--soluções-e-experimento)).
7. **Escolha o framework de priorização pelo estágio, não pelo gosto**: RICE exige dados de reach/impact; pré-product-market-fit com RICE é erro de encaixe ([prioritization](reference/prioritization.md#escolher-o-framework-pelo-estágio)). Registre o framework usado e quando ele será reavaliado.
8. **Nomeie a condição de cancelamento junto com a de sucesso**: "matamos se [métrica] não atingir [limiar] em [prazo]". Sem kill condition, o experimento só confirma ([Pitfall 5](reference/discovery-loop.md#condição-de-cancelamento)).
9. **Feche com GO / PIVOT / KILL explícito** contra 4 critérios: problema validado, solução validada, encaixe estratégico, factível. Decisão que não é nenhum dos três não foi tomada ([Phase 6](reference/discovery-loop.md#fase-6--decidir-e-documentar)).
10. **Atribua um único dono por decisão** e escreva no decision log: o que foi decidido, quem decidiu, quem foi consultado, data e o que reabriria ([DACI](reference/prioritization.md#direito-de-decisão-daci)).

## Portão de honestidade

Reprove a fase que falhe em qualquer item:

- [ ] A hipótese nomeia persona, causa e consequência — e pode ser falsificada.
- [ ] Toda dor citada tem contagem de entrevistas e pelo menos uma citação literal registrada.
- [ ] Nenhuma conclusão depende de uma única voz (diga de quem você **não** ouviu).
- [ ] Cada suposição crítica tem evidência ou experimento designado, com limiar numérico.
- [ ] Existe kill condition escrita antes do resultado.
- [ ] A decisão final é GO, PIVOT ou KILL, com dono e data no log.

## Referências

- [discovery-loop](reference/discovery-loop.md) — 6 fases, hipótese, amostra, roteiro e vieses, saturação, síntese, opportunity solution tree, assumption map, kill condition, decisão.
- [prioritization](reference/prioritization.md) — escolha de framework por estágio, RICE/ICE, template de score, DACI/RACI, decision log.

## Fontes

- `deanpeters/Product-Manager-Skills` — `skills/discovery-process` (6 fases, saturação, GO/PIVOT/KILL), `skills/opportunity-solution-tree` (4 níveis), `skills/assumption-mapping` (5 riscos, evidência vs opinião), `skills/discovery-interview-prep` (vieses e pares pergunta/resposta), `skills/prioritization-advisor` (paisagem de frameworks, fórmula RICE), `skills/decision-rights-matrix` (RACI vs DACI, um decisor).
- `dend/awesome-product-management` (CC0-1.0) — índice de leitura; base canônica: Torres, *Continuous Discovery Habits*; Cagan, *Inspired*.
- Conteúdo reescrito; nenhum trecho literal das fontes. A fonte primária é CC BY-NC-SA 4.0 — não copie texto dela para este repo.
