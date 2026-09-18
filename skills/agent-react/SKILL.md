---
name: agent-react
description: "Disciplina o loop ReAct em tarefas multi-etapa, forçando validação de hipóteses antes de concluir."
---

# Agent ReAct

Responda em PT-BR, objetivo (CLAUDE.md global). O harness já faz ReAct “cru”; esta skill **disciplina** o loop — não substitui `context-guard`, `debugging-strategies` nem `token-saving-toolkit`.

## Use this skill when

- Tarefa com 3+ passos ou várias tool calls até concluir.
- Hipótese precisa ser testada com evidência (código, log, doc, comando).
- Risco de loop: mesma Action repetida sem progresso.
- Usuário pede ReAct, scratchpad estruturado ou “pense → aja → observe”.

## Do not use this skill when

- Tarefa curta e direta (1–2 passos, resposta óbvia).
- Só falta carregar skill de domínio (`symfony`, `ddd`, etc.) — carregue essa, não este protocolo.
- Problema é saúde da janela/drift → `context-guard`.
- Bug com playbook de debug → carregue também `debugging-strategies`.

## Ciclo (obrigatório)

Em cada iteração, nesta ordem:

1. **Thought** — 1–3 linhas: objetivo do ciclo, hipótese atual (marcada como não comprovada), o que já foi refutado.
2. **Action** — **uma** tool call que **valida ou refuta** a hipótese (ou batch paralelo só se independentes).
3. **Observation** — registrar o fato retornado (não o que você esperava).
4. **Update** — promover a fato (com evidência), manter pendente, ou **descartar**. Hipótese refutada **não** volta sem evidência nova.

Não escreva monólogo longo no Thought. Não chame tool “só para ver” sem hipótese.

## Hipótese ≠ fato (contrato obrigatório)

Hipótese **pode** existir (“acredito que X”), mas **nunca** circula como conclusão até Observation positiva.

Toda hipótese ativa exige um **plano de validação** no mesmo turno:

| Quem valida | Quando | Como comunicar |
| --- | --- | --- |
| **Agente** | Tem tool/acesso (código, comando, doc, log) | Formule a hipótese → execute a Action → só então conclua |
| **Usuário** | Falta ambiente, credencial, UI, produção, dado só dele | “Hipótese: … Para validar, rode/confirme: …” — **pare** e espere |

Regras:

- Proibido empilhar “provavelmente A → logo B → logo C” sem validar A.
- Proibido seguir implementação/refator com base só em hipótese não validada.
- Na resposta ao usuário, rotule: **verificado** vs **hipótese (pendente de validação)**.
- Se não puder validar agora, diga o que falta — não preencha com certeza fingida.

## Anti-loop

- Mesma Action (mesmos args relevantes) + mesmo resultado → **proibido** repetir. Mude hipótese ou Action.
- Hipótese refutada → liste em “descartadas” e não reteste sem dado novo.
- Após **3 ciclos sem progresso** (sem fato novo útil) → pare, resuma o que sabe/não sabe e pergunte ou mude de estratégia (outra skill, delegação, handoff).

## Budget

| Sinal | Ação |
| --- | --- |
| ~8–12 tool calls na mesma subtarefa sem fechar | Checkpoint curto + reavaliar objetivo |
| ~15+ tools ou zona amarela do `context-guard` | Carregar `context-guard`, atualizar `STATE.md` |
| Sem progresso claro | Parar e comunicar bloqueio — não “mais uma tentativa” |

## Evidência

- Cite fonte verificada: `path:line`, saída de comando, trecho de doc.
- Proibido afirmar comportamento de código/API sem ter lido/rodado nesta sessão (regra anti-alucinação global).
- Prefira `ast-outline` / leituras parciais / `docs-fetch` a despejar arquivo inteiro (`token-saving-toolkit`).

## Scratchpad (opcional, curto)

Use só se ajudar a não perder o fio — não cole na resposta final ao usuário:

```text
Goal: …
Hypotheses: [ativas + como validar] / [descartadas + motivo]
Verified: …
Last: Thought → Action → Observation
Next validate: (eu: tool…) | (user: comando/ação…)
```

## Integração com outras skills

| Situação | Skill |
| --- | --- |
| Sessão longa / drift / compaction | `context-guard` + `STATE.md` |
| Bug / root cause | `debugging-strategies` |
| Reduzir tokens na inspeção | `token-saving-toolkit` |
| Delegar a outro CLI/modelo | `agent-delegate` |
| Arquitetura / DDD / padrões | `arch-context-check` primeiro |

## Fechamento

Antes de declarar pronto: evidência bate com a conclusão; nenhuma hipótese pendente foi vendida como fato; hipóteses descartadas não foram reintroduzidas; nada de Action órfã sem Observation usada.

## Lembrete automático (hook)

- Se aparecer mensagem começando com `[agent-sync]` cobrando validação de hipótese / `agent-react`, ela vem do hook `hooks/agent-react-nudge*` (ou plugin OpenCode), a cada N tool calls (padrão 15, `AGENT_SYNC_REACT_NUDGE_THRESHOLD`) — não é o usuário. Aja: valide a hipótese ativa ou peça o passo ao usuário; não precisa recitar o protocolo inteiro.

## Additional resources

- Exemplos de ciclo bom vs ruim: [reference.md](reference.md)
