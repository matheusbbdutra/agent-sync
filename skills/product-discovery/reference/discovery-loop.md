# Discovery loop — 6 fases com portão

Fonte primária: `deanpeters/Product-Manager-Skills`, `skills/discovery-process/SKILL.md` (CC BY-NC-SA 4.0).
Reescrito; nenhuma frase copiada. Citação por seção, não por página — o material é markdown.

## Quando usar

O ciclo roda em 2-4 semanas e serve para explorar área nova, investigar queda de retenção/churn,
validar iniciativa antes de comprometer roadmap, e manter toque semanal com cliente
(`discovery-process#Key Concepts`).

Não use quando:

- o problema já está bem entendido → vá para execução;
- o stakeholder já se comprometeu com a solução → resolva o alinhamento primeiro;
- é bug tático ou dívida técnica → não há o que descobrir.

Discovery **não é** pesquisa em cascata (waterfall) feita uma vez antes do dev, **não é** teste de
usabilidade — teste valida solução, discovery valida problema — e **não substitui** entrega
(`discovery-process#Anti-Patterns`).

## Fase 1 — Enquadrar o problema

Saída obrigatória:

```text
Hipótese: Acreditamos que [persona] sofre com [problema] porque [causa raiz], levando a [consequência].
Perguntas de pesquisa: 3-5, cada uma falsificável por evidência de cliente.
Critério: o que valida / o que invalida o problema.
```

Regras:

- Sem persona, causa e consequência a frase vira assunto. Causa raiz explícita é o que permite
  escolher o experimento depois (`discovery-process#Outputs from Phase 1`).
- Perguntas de pesquisa precisam ser respondíveis por comportamento passado, não por intenção.
- Se o segmento está nebuloso, personas são hipótese a testar, não retrato de mercado.
- Se a motivação do cliente está nebulosa, escreva o job (o progresso que ele tenta obter), não a
  feature que ele pediu (`discovery-process#Phase 1: Frame the Problem`).

Portão: a hipótese pode ser falsificada com uma entrevista? Se não, reescreva antes de agendar.

## Roteiro e vieses

O roteiro existe para **não induzir**. Pares verificados no material — à esquerda o erro, à direita
a pergunta utilizável (`discovery-interview-prep#Biases to Avoid`):

| Viés | Não pergunte | Pergunte |
|---|---|---|
| Confirmação | "Você não acha que X é um problema?" | "Me conte como você vive X hoje." |
| Pergunta indutiva | "Você usaria isto?" | "O que já tentou? Por que funcionou ou falhou?" |
| Hipotético | "Se construíssemos Y, você pagaria?" | "Pelo que você paga hoje? Por quê?" |
| Pitch disfarçado de pesquisa | "Estamos construindo Z para resolver X" | "Estou estudando X. Me conte sua experiência." |
| Sim/não | "Faturar é difícil?" | "Me mostre seu processo de faturamento." |

Estrutura de sessão: abertura curta (5 min), núcleo (30-40 min), fechamento (5 min)
(`discovery-interview-prep#Interview Framework`).

Amostra: defina segmento **e papel** (usuário, comprador, quem opera, quem aprova) — cada papel vê
um risco diferente. Registre quem faltou: conclusão apoiada em uma voz é hipótese, não achado
(`assumption-mapping#Generating Them`).

## Fase 3 — Conduzir

Saturação é o critério de parada, não o número redondo de entrevistas: **a mesma dor apareceu em
3+ entrevistas e nada novo surgiu**. Na prática fecha em **5-7 entrevistas**; se ainda aprende algo
novo, agende **3-5** e conte o custo de +1 semana (`discovery-process#Decision Point 2`).

Registro por entrevista, mínimo:

- citação literal da dor (palavras da pessoa, não parafrase);
- o que ela **fez** a respeito (workaround, gasto, abandono);
- papel e contexto (empresa, porte, ferramenta atual);
- o que a entrevista contradisse do que você achava.

## Fase 4 — Sintetizar

Agrupe por dor, não por pessoa. Para cada dor: contagem de entrevistas que a sustentam, citação
representativa, workaround observado e o que ainda não se sabe.

Evidência é o que aconteceu: ação, gasto, abandono, tempo perdido. Opinião — inclusive elogio — não
é evidência (`assumption-mapping#Evidence Is What Happened, Not What Was Said`).

Síntese que não muda nenhuma decisão é relatório, não síntese.

## Fase 5 — Soluções e experimento

Estrutura obrigatória (opportunity solution tree, `opportunity-solution-tree#What is an Opportunity Solution Tree (OST)?`):

```text
Outcome desejado (1 métrica de negócio)
├── Oportunidade 1 ── solução A / B / C
├── Oportunidade 2 ── solução A / B / C
└── Oportunidade 3 ── solução A / B / C
```

Regras:

- Oportunidade é **problema do cliente**, não feature disfarçada — "precisa de dashboard" é solução
  (`opportunity-solution-tree#Pitfall 1: Opportunities Disguised as Solutions`).
- Diverja antes de convergir: 3 soluções por oportunidade antes de escolher a POC
  (`opportunity-solution-tree#Pitfall 2: Skipping Divergence`).
- Outcome vago ("melhorar experiência") não permite medir; ancore em uma métrica
  (`opportunity-solution-tree#Pitfall 3: Outcome is Too Vague`).
- Toda solução escolhida leva um experimento: o que mede, limiar, custo, prazo.

### Assumption map

Ordene cada suposição por **importância** (quanto dói se estiver errada) × **evidência** (quanto já
se sabe). O quadrante de alta importância e baixa evidência é a fila do experimento
(`assumption-mapping#Importance by Evidence`).

Cinco categorias de risco — classifique cada suposição em uma
(`assumption-mapping#Five Kinds of Risk`):

| Tipo | Pergunta |
|---|---|
| Desejável | O cliente quer, e resolve dor que ele sente? |
| Usável | Ele consegue usar sem ajuda? |
| Viável | Funciona para o negócio: receita, custo, canal, legal? |
| Factível | Dá para construir e operar? |
| Ético | Pode causar dano ou quebrar confiança? |

Gere suposições perguntando "o que teria de ser verdade?" ao longo da história inteira — primeiro
contato, uso, pagamento, renovação —, por papel. Inclua as entediantes: "o time terá tempo",
"jurídico aprovará" (`assumption-mapping#Generating Them`).

## Condição de cancelamento

Escreva antes do resultado, não depois:

```text
Matamos se [métrica] não atingir [limiar] até [data].
Seguimos se [métrica] atingir [limiar] em [amostra mínima].
```

Suposição sem kill condition vira projeto por inércia — é pitfall explícito do material
(`assumption-mapping#Pitfall 5: No Kill Condition`).

## Fase 6 — Decidir e documentar

Decisão é sempre uma das três (`discovery-process#Phase 6: Decide & Document`):

- **GO** — vai para roadmap, com épico escrito como hipótese e critério de sucesso.
- **PIVOT** — problema validado, solução não; volte para a árvore.
- **KILL** — problema real, mas não agora; registre por quê para não redescobrir.

Quatro critérios simultâneos para GO: problema validado (fases 3-4), solução validada (fase 5),
encaixe estratégico, factível (`discovery-process#Phase 6: Decide & Document`).

Readout de 30 min: o que foi validado sobre o problema, o que foi validado sobre a solução,
recomendação e o que muda se ela for aceita.

## Pitfalls que invalidam o ciclo

Extraídos do material (`discovery-process#Common Pitfalls`):

1. **Pular entrevistas** e "validar" com dados de analytics — analytics diz onde, não por quê.
2. **Pergunta indutiva** — a resposta confirma o que você queria ouvir.
3. **Não saturar** — decide com 2 conversas e uma planilha.
4. **Paralisia de análise** — discovery infinito para não ter de decidir.
5. **Discovery como evento único** — depois da primeira entrega, ninguém volta a falar com cliente.
