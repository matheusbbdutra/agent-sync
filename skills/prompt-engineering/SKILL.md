---
name: prompt-engineering
description: "Use when escrevendo ou revisando o prompt de uma aplicacao LLM: system message, template, persona de agente, few-shot e contrato de output. Construcao e revisao de prompt — arquetipo de documento, instrucao verificavel, formato parseavel, opcoes de classificacao e gate de qualidade. Nao mede qualidade de resposta; medicao e llm-eval."
---

# Prompt Engineering

Prompt é documento, não comando. O trabalho é decidir **que tipo de documento o modelo deve continuar** e fechar o critério de aceite antes de publicar. Medir qualidade e comparar versões por score fica em `llm-eval`; orçamento de janela e compactação ficam em `context-guard` e `token-saving-toolkit`.

## Use this skill when

- Escrever ou revisar system message, template de prompt, persona de agente ou mensagem de instrução.
- Decidir se um conjunto few-shot é necessário e como montá-lo.
- Definir o formato de saída que código vai parsear.
- Revisar prompt que não tem critério de aceite explícito — aplique o gate de 4 critérios abaixo.
- Tirar prompt de dentro de string no código e versioná-lo.
- Escolher parâmetros de geração (`temperature`, `top_p`, `top_k`, beam) coerentes com a tarefa.

## Do not use this skill when

- A pergunta é "esta saída está boa?" ou "a versão A é melhor que B?" → `llm-eval`.
- O problema é janela cheia, compaction, drift de sessão ou STATE.md → `context-guard`.
- O problema é reduzir tokens de leitura de arquivo/log ou cortar geração com stop sequence → `token-saving-toolkit`.
- O objeto é a tool spec, o workflow multi-etapa ou o loop reason/act → `mcp-advisor`, `agent-delegate`, `agent-react`.
- O tema é prompt injection, ação irreversível ou conteúdo não confiável → `security-auditor`.

## Instructions

1. **Escolha o arquétipo do documento** pela forma da resposta desejada e não invente formato: reuse motivos que existem em corpora de treino (`#`, cerca de código, listas). "Don't stray far from the path upon which the model was trained" — p.81. Detalhe em [prompt-craft](reference/prompt-craft.md).
2. **Escreva a instrução como afirmação verificável**: peça o que fazer, não o que evitar, e ancore com a razão — "Ask for positives instead of negatives and dos instead of don'ts." / "Bolster your command with a reason." — p.102.
3. **Exija clarificação antes da resposta** quando o pedido admite mais de uma leitura: "Clarification creates consistency." — p.101.
4. **Só adicione few-shot se houver aspecto não óbvio** (formato, persona, distribuição de rótulos) que os exemplos carreguem — p.111.
5. **Escreva o início da resposta no prompt** quando o modelo insiste em reformular o problema: "change your perspective from problem poser to problem solver" — p.136.
6. **Feche início e fim reconhecíveis** no output parseável e escolha delimitador cujo término seja testável por substring — p.161-162.
7. **Em classificação, garanta primeiro token único por opção** e proíba resposta com várias alternativas — p.165.
8. **Trate logprob como gate de confiança**, não como verdade: derive um limiar para avisar, recuperar mais contexto ou escalar o modelo — p.163-164.
9. **Versione o prompt fora do código**, com metadados (modelo, data, aplicação, schema de entrada/saída, sampling) e um catálogo que permita versões diferentes por aplicação — p.253, p.255.
10. **Fixe os parâmetros de geração** por tipo de tarefa; ver [generation-params](reference/generation-params.md).

## Gate de revisão: os 4 critérios (p.81-82)

Reprove o prompt que falhe em qualquer um:

- [ ] **Parece documento do training set** — formato reconhecível, sem sintaxe proprietária (p.81).
- [ ] **Contém toda a informação relevante** — nada que o modelo precise adivinhar (p.81).
- [ ] **Conduz a uma completion que resolve o problema** — não apenas reafirma o pedido: "the prompt must condition the model to generate a completion that is actually helpful" (p.82).
- [ ] **Tem ponto de parada natural** — o fim da resposta é testável por substring (p.161-162).

Aplique o gate antes de publicar e depois de cada mudança de template. Ordenar contexto por importância, contar tokens e decidir o que entra na janela são etapas de `context-guard`; este gate avalia forma e contrato.

## Referências

- [prompt-craft](reference/prompt-craft.md) — arquétipos de documento, few-shot com gatilho, inertness, posição/dependência de snippet, contrato de output parseável, classificação com opções disjuntas, logprobs.
- [generation-params](reference/generation-params.md) — greedy vs beam, temperature, top-k/top-p, repetição, custo por token gerado.

## Fontes

- Berryman & Ziegler, *Prompt Engineering for LLMs* — cap. 4 Designing LLM Applications (p.81-82); cap. 5 Prompt Content (p.101-102, p.111); cap. 6 Assembling the Prompt (p.136, p.161-162); cap. 7 Taming the Model (p.163-165).
- Huyen, *AI Engineering* — cap. 5 Prompt Engineering: Prompt Engineering Best Practices (p.240-243) e Organize and Version Prompts (p.253-255).
