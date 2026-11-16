---
name: llm-integration
description: "Use when integrando um provedor de LLM a um produto que já existe: adapter de provedor, streaming, contrato de saída validado por schema, orçamento de tokens/custo por requisição, SLO de latência (TTFT/TPOT, goodput), cache de segmento e decisão retrieval-vs-fine-tune por tipo de falha. Assume que o modelo é de terceiro e que o produto é seu. Não escreve o prompt (prompt-engineering) nem mede qualidade (llm-eval)."
---

# LLM Integration

O modelo é de terceiro; o sistema que o chama é seu. O trabalho aqui é a **fronteira**: como a chamada entra no produto, o que volta, quanto custa, o que acontece quando falha e o que o usuário vê.

Regra de ordem que evita 80% do retrabalho: comece em `query → modelo → resposta` e só adicione componente quando a dor aparecer — contexto, guardrail, router/gateway, cache, lógica agêntica (p.470). Pipeline complexo antes de existir observabilidade é dívida imediata.

Não escreve nem revisa o texto do prompt (`prompt-engineering`), não mede qualidade (`llm-eval`), não treina nem faz fine-tune de modelo — o fine-tune entra aqui apenas como **decisão** econômica (`reference/retrieval-decision.md`).

## Use this skill when

- Adicionar uma chamada de LLM a um produto existente (web, app, CLI, backend).
- Escolher entre manter prompt, adicionar retrieval ou partir para fine-tune.
- Definir contrato de saída que o código vai consumir sem quebrar.
- Estimar custo por requisição e decidir o que cachear.
- Definir SLO de latência e comparar provedores com número, não com sensação.
- Tratar entrada não confiável (documento, e-mail, tool output) que vai para o contexto.
- Definir o que acontece quando o provedor falha, atrasa ou devolve formato inválido.

## Do not use this skill when

- O assunto é redigir/revisar o prompt em si → `prompt-engineering`.
- O assunto é medir qualidade de resposta ou gate de regressão → `llm-eval`.
- O assunto é reduzir tokens da **sua** leitura de arquivo/log no agent-sync → `token-saving-toolkit`.
- O assunto é janela cheia, compactação ou `STATE.md` → `context-guard`.
- Retry/timeout/circuit breaker genéricos, sem nada de LLM → `error-handling-patterns`.
- Desenho de API pública, versionamento e contrato HTTP → `api-design-principles`.

## Instructions

1. **Isole o provedor atrás de um adapter seu** antes da primeira linha de feature. Nenhum tipo, header ou nome de modelo do fornecedor vaza para a regra de negócio; troca de provedor vira uma implementação nova do mesmo contrato (`reference/integration-boundary.md#provider-adapter`).
2. **Escolha o tier de modelo por tarefa, não por prestígio.** Classificação e extração raramente precisam do modelo maior; raciocínio multi-passo pode precisar. Registre a escolha com o número que a justificou (p.337).
3. **Feche o contrato de saída antes de integrar.** Schema explícito, validação no código, retry limitado em falha de formato e nenhuma escrita em banco com saída não validada (`reference/output-contract.md#contract`).
4. **Classifique a origem do contexto.** Conteúdo recuperado, e-mail, página e tool output são **input não confiável** e não obedecem a instrução: aplique hierarquia e nunca concatene como se fosse system prompt (p.262-263).
5. **Defenda no sistema, não só no prompt.** Comando com efeito (DELETE/DROP/UPDATE, enviar e-mail, transferir) exige aprovação humana; código gerado roda isolado (p.270). Guardrail que recusa tudo é tão quebrado quanto o que aceita tudo (p.268).
6. **Faça a conta por requisição antes de escalar**: tokens de entrada (system + contexto recuperado) + tokens de saída × preço, com cache de segmento invariante. System prompt de 1.000 tokens em 1M de chamadas/dia é ~1 bilhão de tokens repetidos (p.463-464).
7. **Meça latência como distribuição.** Reporte TTFT e TPOT separados (`total ≈ TTFT + TPOT × tokens de saída`) em p50/p90/p99, e conte **goodput** (só o que cumpre o SLO), não throughput bruto com batching (p.433-435).
8. **Escreva o comportamento de falha antes do caminho feliz**: timeout, rate limit, provedor fora, formato inválido, recusa, truncamento — com o que o usuário vê em cada caso (`reference/integration-boundary.md#failure-modes`).
9. **Decida retrieval por tipo de falha, não por moda.** Falha de informação → retrieval; falha de comportamento (formato, estilo, irrelevância) → fine-tune, e só depois de esgotar prompt/retrieval (p.336).
10. **Instrumente com trace desde a primeira versão**: endpoint, modelo, sampling, template, tokens, saída e decisão do guardrail. Sem isso, todo incidente vira arqueologia (p.487, p.490).

## Gate antes de expor ao usuário

- [ ] Existe adapter e a regra de negócio não importa o SDK do provedor.
- [ ] A saída é validada por schema; falha de formato tem retry **limitado** e caminho de erro visível.
- [ ] Entrada não confiável está marcada e hierarquizada; nenhuma instrução vem dela.
- [ ] Toda ação com efeito colateral exige aprovação humana (p.270).
- [ ] Custo por requisição calculado com número, e o segmento invariante está em cache (p.463-464).
- [ ] SLO declarado em percentil (p90/p99) com TTFT/TPOT separados, e goodput medido (p.433-435).
- [ ] Comportamento de falha de cada dependência está escrito e testado.
- [ ] Trace registra modelo, versão do template, tokens, custo e saída (p.490).
- [ ] A decisão retrieval-vs-fine-tune tem o **tipo de falha** escrito ao lado (p.336).

## Referências

- [integration-boundary](reference/integration-boundary.md) — adapter de provedor, router/gateway, streaming, retry/idempotência em chamada não determinística, modos de falha, telemetria, rollout.
- [output-contract](reference/output-contract.md) — schema, validação, retry limitado, recusa/truncamento, input não confiável, guardrail no sistema.
- [retrieval-decision](reference/retrieval-decision.md) — prompt vs retrieval vs fine-tune por tipo de falha, hybrid search, rerank, chunk/overlap, métricas do retriever.

## Fontes

- Huyen, *AI Engineering* — cap. 5 (contexto, prompt injection indireta, guardrails, p.262-275); cap. 6 (RAG: retrieval híbrido, chunking, query rewriting, p.284-290); cap. 7 (fine-tuning vs RAG, curva de dados, p.333-394); cap. 8 (inferência: TTFT/TPOT, goodput, batching, p.433-435); cap. 9 (prompt cache, gateway/router, ordem de componentes, métricas e trace, p.463-490).
- Números da comparação RAG vs fine-tune (p.337) vêm do digest `~/Documentos/Books/analysis/ai-engineering.md`, não de leitura direta da página.
