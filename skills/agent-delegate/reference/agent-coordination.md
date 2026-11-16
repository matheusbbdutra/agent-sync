# Coordenacao de agentes

Criterio de leitura para sistema com dois ou mais agentes, ou com plano de controle explicito:
quais componentes existem, quem controla o fluxo, quem planeja, quem executa e onde ha aprovacao.
Fora deste arquivo: escolha de CLI/alvo e modo `print`/`session` (ver `SKILL.md` desta skill) e o
ciclo Thought/Action/Observation de uma sessao unica (`agent-react`).

## 1. Declare o modo e o ponto de aprovacao

"Modo" aqui e a taxonomia de agente do livro, nao o modo `print`/`session` do `SKILL.md`.

- Escolha explicitamente um dos 4 modos e registre no design: LLM direto, proxy (o LLM reformula a
  requisicao), agente com aprovacao do usuario antes da tool call, ou autonomo (p.25).
- No modo com aprovacao, o checkpoint fica **antes** da chamada da tool: "before making a call, the
  LLM requires user approval" (p.25).
- Trate o modo autonomo como decisao de risco registrada, nao como default: e o modo que o livro
  associa a "most ethical and safety concerns" (p.25-26).
- Com mais de uma tarefa no fluxo, configure a cadencia de permissao — a cada tarefa ou a cada N
  tarefas (p.32-33). Sem cadencia declarada, o checkpoint existe no desenho e nao no fluxo.
- Nao rotule de autonomo o que so reformula prompt (proxy): o rotulo errado esconde que nao ha
  aprovacao em ponto nenhum (p.25).

## 2. Decomponha o agente em 5 componentes

- Descreva todo agente como profile/persona, actions/tools, memory/knowledge,
  reasoning/evaluation e planning/feedback (p.27-28).
- O perfil e o centro: "At the core of all agents is the profile and persona" (p.27-28).
- Componente inexistente vira linha "ausente" no spec. Agente reduzido a system prompt grande nao
  tem onde evoluir por partes (p.27-28).
- Mantenha reasoning/evaluation e planning/feedback como itens distintos do spec: o livro os
  desenha como componentes separados (p.31).

## 3. Plano de controle: arvore de comportamento, nao FSM

- Modele o controle em arvore com os tipos de no e retorno binario success/failure: selector
  (fallback), sequence, condition, action, decorator e parallel (p.153-154).
- Rejeite maquina de estados para fluxo que vai crescer: "FSMs aren't practical for agents because
  they don't scale well" (p.156).
- Use o segundo argumento do livro na mesma decisao: a arvore separa a logica de decisao
  ("Decoupling of decision logic", p.156).
- Ramificacao, retry e pre-condicao viram nos da arvore, nao `if` espalhado no executor: e a mesma
  tabela de nos que cobre essas tres funcoes (p.153-154).

## 4. Construa a topologia por back chaining, a partir do goal

- Siga os 5 passos nesta ordem: partir da behavior do goal, listar as acoes que levam a ela,
  derivar as condicoes de sucesso de cada acao, escolher o modo de comunicacao, e so entao montar
  os nos (p.178).
- O modo de comunicacao e passo proprio do back chaining ("Determine the mode of communication",
  p.178) — nao e detalhe de implementacao decidido depois.
- Comece com menos agentes do que a intuicao pede: "It's better to start evaluating with
  insufficient assistants than with too many" (p.179).
- Teste e verificacao ficam em agentes distintos: "testing and verification are best done with
  different assistants" (p.179).
- Nao monte arvore por tentativa: o metodo do livro deriva os nos das condicoes de sucesso, nao de
  hipoteses sobre quantos agentes o problema "merece" (p.178).

## 5. Escolha a topologia de conversa pelo que precisa convergir

- Varias personas que precisam ver o mesmo historico: group chat com mensagens compartilhadas e
  `max_round` explicito (p.106 usa `max_round=20`).
- Passos isolados: nested ou sequential — aceitando que o repasse serial pode alterar a tarefa ou o
  resultado desejado (p.105).
- Pese o custo antes de preferir group chat: esses fluxos sao "substantially more verbose and token
  expensive" (p.107).
- Isole em thread nova o no que sofre com ruido: o verificador roda "on a different thread,
  essentially in closed room" (p.171-172).
- Use thread compartilhada so onde a iteracao e o objetivo; o isolamento deixa a interacao de cada
  assistente em uma conversa concisa e mais facil de depurar (p.173).
- Assistente silado consome a thread nova sozinho ("are the only consumers", p.176): nao espere
  feedback mutuo de quem roda isolado.

| Situacao | Topologia | Evidencia |
| --- | --- | --- |
| Personas precisam convergir no mesmo artefato | group chat com manager e `max_round` | p.106 |
| Passo unico, sem dependencia de contexto | nested/sequential | p.105 |
| Etapa de verificacao que nao pode ser contaminada | thread nova (silo) | p.171-172, p.176 |
| Iteracao e correcao mutua sao o objetivo | thread compartilhada | p.173 |
| Custo por run ja alto e time pequeno | sequential/isolado, nao group chat | p.107 |

## 6. Troque revisao humana serial por critic com teto de turnos

- Use um segundo agente como proxy/critic quando a revisao precisa ser automatica: o proxy
  "works to replace the required human feedback and evaluation" (p.93).
- Escreva o criterio do critic na persona, nao no genio do revisor: "Your task is to scrutinize
  code content for any harmful or substandard elements" (p.103).
- Limite o nested chat do critic com `max_turns` curto (p.104 usa `"max_turns": 1`); sem teto, o
  critic vira segundo loop de producao.

## 7. Contrato do planner

- Injete apenas tres coisas: a lista de funcoes disponiveis com descricao/args, o goal e o feedback
  corretivo de planos anteriores (p.303).
- Exija no prompt: JSON, "Each subtask must be from within the [AVAILABLE FUNCTIONS] list. Do not
  use any functions that are not in the list" e "The plan should be as short as possible" (p.303).
- Nao envie historico de conversa ao planner: o plano e construido em isolamento para focar no goal,
  porque prompts de planejamento consomem muitos tokens (p.304).

## 8. Execute o plano fora do LLM

- Interprete o JSON do plano em executor local e mantenha um contexto de resultados por funcao
  (p.304-305); devolva esse contexto ao LLM apenas para sumarizar/responder.
- Nao dependa de o modelo orquestrar chamadas sequenciais: "OpenAI, Groq, and Azure OpenAI all
  support parallel actions but not sequential or planned actions" (p.299).
- Ganho declarado do executor externo: reduz tokens e tool use, e o modelo nao precisa suportar
  tools para usar um planner (p.308).
- Nao reenvie o estado inteiro a cada passo: o executor popula o contexto e o LLM recebe o
  resultado consolidado (p.304-305).

## 9. Coordenacao: canal e contrato do pedido

- Sync para construir rapport e gerar ideias; async para reportar progresso, coletar feedback e
  disseminar informacao (p.264-265).
- O que define async e a expectativa de resposta, nao a ferramenta: "Whether communication is
  asynchronous does not depend on the tool used but rather on the expectations of the way the tool
  is used" (p.272-279).
- Se uma troca assincrona passa de 4 idas-e-voltas sem convergir, troque para sincrono (p.279).
- Todo pedido a outro agente explicita Who (quem responde e quem so fica ciente), What (entrega e
  formato), When (prazo absoluto), Wah-wah (o que acontece se nao houver resposta) e Why (p.280-281,
  p.293).
- Prazo absoluto com fuso, nunca relativo (p.293-295): "Using statements such as end of day tomorrow
  or asap is meaningless"; use a forma "@Gino, can you please let me know by 4 p.m. (CET) this
  Friday" (p.293-295).
- Delta em relacao ao brief do `SKILL.md`: la estao objetivo/autoridade/validacao; aqui o que falta
  no brief tipico e o prazo absoluto e o Wah-wah (p.280-281).

## 10. Escolha o modelo por criterios declarados

- Registre a decisao sobre os 6 criterios do livro: tipo/use case, dado de treino, tamanho,
  performance em benchmark, custo e velocidade de deploy (p.34-35).
- Reavalie a escolha quando custo ou latencia passarem a dominar o caso de uso (p.34-35).
- Nao espere limiar do livro: ele lista os criterios e nao define priorizacao nem numeros de corte
  (p.34-35, unidade de confianca media no digest).

## 11. Antes de acrescentar agente ou camada hierarquica

- Ligue tracing (tokens, custo, latencia, tool calls) e acompanhe repeticao de pensamento: padroes
  repetitivos indicam agente pouco decisivo (p.117).
- Se o custo dobra sem ganho de qualidade, mude padrao de processamento, tasks e goals — nao
  adicione manager: "it will cost over double what it would have without the agent manager. The
  output will also likely not be significantly better" (p.117).
- Coloque avaliacao e feedback no fim do fluxo, como etapa separada, para evitar feedback looping
  (p.318).

## Componente -> pergunta de revisao

| Componente | Pergunta que o revisor faz | Evidencia |
| --- | --- | --- |
| profile/persona | qual o papel e o criterio de sucesso deste agente? | p.27-28 |
| actions/tools | quais acoes existem e qual o schema de cada uma? | p.27-28 |
| memory/knowledge | o que este agente lembra entre execucoes? | p.27-28 |
| reasoning/evaluation | como ele decide que o passo atual esta bom? | p.31 |
| planning/feedback | como ele replaneja depois de falhar? | p.31 |

## Sinais de alerta no design

| Sinal | Leitura | Correcao |
| --- | --- | --- |
| Modo do agente nao declarado | nao se sabe onde ha aprovacao (p.25) | declarar modo + cadencia (p.32-33) |
| Arvore de controle com dezenas de estados `if` | FSM disfarcada, nao escala (p.156) | reescrever em nos de arvore (p.153-154) |
| Planner recebendo o historico da conversa | plano inflado e caro (p.304) | cortar historico e feedback corretivo (p.303-304) |
| Modelo orquestrando chamadas sequenciais | suporte inexistente nos provedores citados (p.299) | executor local (p.304-305) |
| Pedido com "asap" | prazo sem referencia (p.293-295) | data/hora com fuso (p.293-295) |
| Novo agente para resolver custo alto | custo dobra sem ganho (p.117) | medir repeticao e revisar tasks/goals (p.117) |

## Checklist de revisao do design

- [ ] Modo declarado e ponto de aprovacao antes da tool call, com cadencia (p.25, p.32-33).
- [ ] 5 componentes nomeados, com "ausente" explicito onde faltar (p.27-28, p.31).
- [ ] Plano de controle em arvore com retorno binario; nenhuma FSM (p.153-154, p.156).
- [ ] Topologia derivada por back chaining, incluindo o modo de comunicacao (p.178).
- [ ] Teste e verificacao em agentes distintos e no minimo suficiente de agentes (p.179).
- [ ] Planner sem historico, preso a lista de funcoes e ao plano mais curto (p.303-304).
- [ ] Executor local do plano, LLM so sumariza (p.304-305, p.308).
- [ ] Pedido com prazo absoluto e Wah-wah; escalada para sincrono no 4o round-trip (p.279,
  p.293-295).
- [ ] Custo e repeticao medidos antes de escalar a topologia (p.117).

## Fontes

- AI Agents in Action (Micheal Lanham) — cap. 1 (p.25-33: modos, 5 componentes, cadencia de
  permissao); cap. 2 (p.34-35: 6 criterios de modelo); cap. 4 (p.93-107: proxy/critic, nested vs
  group chat, custo do manager; p.117: repeticao e custo antes de escalar); cap. 6 (p.153-156:
  nos da behavior tree e limite da FSM; p.171-176: thread silada vs compartilhada; p.178-179: back
  chaining); cap. 11 (p.299-308: planner e executor externo; p.318: feedback isolado no fim).
- Communication Patterns (Jacqui Read) — cap. 14 (p.264-279: sync vs async, definicao por
  expectativa de resposta, escalada no 4o round-trip); cap. 15 (p.280-295: 4 Ws + Why, prazo
  absoluto).
- Nota de escopo: o brief listou U1, U2, U10, U12, U21, U22 e U25; os topicos pedidos (topologia e
  coordenacao multi-agente, aprovacao antes da tool call) exigiram tambem U3, U7, U8, U9, U11 e
  U24, que o proprio digest mapeia para o reference de `agent-delegate` (secao 3 do digest).
