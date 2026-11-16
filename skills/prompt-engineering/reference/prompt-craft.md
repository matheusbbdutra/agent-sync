# prompt-craft

Regras de forma e de contrato para escrever e revisar prompt de aplicação. Pressupõe `SKILL.md` carregado.
Marcadores `p.N` vêm do digest da fonte (cap. 6 de *Prompt Engineering for LLMs* começa em p.133). Orçamento
de token, ordem de contexto por turno e compactação **não** estão aqui: ficam em `context-guard` e
`token-saving-toolkit`.

## 0. Fluxo de escrita

Seis passos, na ordem. Pular um deles é o que produz prompt que "funciona na minha máquina".

1. **Classifique a resposta** e escolha o arquétipo (§1) — p.137, p.140, p.143.
2. **Escreva a instrução como afirmação verificável** com a razão anexada (§2) — p.101-102.
3. **Feche o contrato de output** com start e terminador testável (§6) — p.161-162.
4. **Decida few-shot pelo gatilho**, nomeando a regra implícita (§3) — p.111.
5. **Declare posição, importância e dependência** de cada elemento (§5) — p.151.
6. **Passe o gate de 4 critérios** do `SKILL.md` — p.81-82.

## 1. Arquétipo de documento

Antes de escrever, decida **que tipo de documento a resposta é** e copie essa forma. O critério de escolha
é a forma da resposta desejada, não o gosto de quem escreve (p.137, p.140, p.143).

| Arquétipo | Seção | Quando serve | Falha típica |
| --- | --- | --- | --- |
| Advice conversation | p.137 | diálogo multi-turno, resposta é a próxima fala de um assistente | tarefa analítica com fronteira rígida; o modelo abre idas e voltas de esclarecimento |
| Analytic report | p.140 | análise com escopo declarado e seções nomeadas | escopo implícito; o modelo extrapola o que estava fora |
| Structured document | p.143 | saída que precisa ser parseada (XML/YAML/JSON/código) | terminador frágil: indentação não é testável por substring (p.161-162) |

- **Declare a fronteira como negação explícita no corpo**: "This report focuses solely on novels, excluding
  self-help books." (p.140).
- **Reporte respeita limite melhor que diálogo**: "LLMs tend to respect such clear boundaries more
  consistently in reports than in dialogues" (p.140) — se há conteúdo proibido, use reporte com seção
  `Scope`, não conversa.
- **Prompt longo em Markdown com índice**: seções `# Ideas` / `# Analysis` antes de `# Conclusion` guiam a
  conclusão (p.141).
- **Seção sentinela como fim**: `# Further Reading` no fim do documento dá ao modelo um ponto de parada
  previsível (p.141). O corte por `stop` em si é chamada de API, ver `token-saving-toolkit`.
- **Não invente sintaxe**: o formato do prompt deve parecer documento que existe em corpora de treino
  (p.81). `#`, cerca de código, lista numerada e heading já são motifs conhecidos.

## 2. Instrução verificável

- **Positivo em vez de negativo**: "Ask for positives instead of negatives and dos instead of don'ts."
  (p.102). Reescreva "não invente números" como "cite apenas números que aparecem no contexto".
- **Ancore o comando numa razão**: "Bolster your command with a reason." (p.102) — "responda em no máximo
  3 bullets porque a saída é lida em terminal".
- **Evite absoluto** (p.102): "nunca" / "sempre" em contexto ambíguo produz violação parcial silenciosa;
  troque por condição testável.
- **Peça clarificação antes de produzir**: "Clarification creates consistency." (p.101) — se o pedido
  admite mais de uma leitura, o prompt exige que o modelo pergunte antes da resposta.
- **O prompt precisa condicionar a uma resposta útil**, não só reafirmar o pedido: "the prompt must
  condition the model to generate a completion that is actually helpful" (p.82).
- **Toda informação relevante no prompt**: critério 2 do gate — o modelo não deve precisar adivinhar
  (p.81). Falta de dado não se compensa com instrução mais forte.

## 3. Few-shot com gatilho

Use exemplo só quando houver regra implícita a demonstrar. O gatilho é: **nomeie a regra antes de colar o
exemplo**.

- **Sem regra nomeável, sem few-shot**: "Use few-shot prompting if you have relevant examples that
  illustrate an aspect of what you want the model to do that is otherwise unobvious." (p.111).
- **Formato apenas → exemplo mínimo**: few-shot é "particularly suited to quickly and easily demonstrating
  just the expected output format and nothing more" (p.106).
- **Pattern é compulsão**: "LLMs have a compulsion to continue patterns" (p.104) — tudo que aparece no
  exemplo vira regra, inclusive o que você não queria fixar.
- **Respeite a distribuição real**: "If you have an idea what the distribution is, don't stray too far away
  from it." (p.108).
- **Cubra as classes e as exceções não triviais**: um exemplo por classe relevante, incluindo o caso de
  borda que o prompt precisa acertar (p.108).
- **Embaralhe a ordem**: "Your prompt examples are not randomly ordered unless you consciously shuffle
  them." (p.109) — happy path primeiro e erro depois cria padrão espúrio.
- **Mantenha o nível de contexto**: exemplo gigante emparelhado com pergunta curta (ou o contrário) muda o
  que o modelo imita; encolher demais remove o raciocínio que se queria demonstrar (p.109).
- **Formato do exemplo segue o documento de origem** (p.81, p.149): código em cerca de código, tabela em
  Markdown — o modelo já viu esses motifs em treino.

| Situação | Few-shot? | Base |
| --- | --- | --- |
| formato de saída não óbvio | sim, exemplos mínimos | p.106 |
| persona ou estilo específico | sim, exemplos reais e curtos | p.111 |
| fronteira sutil entre classes | sim, um por classe | p.108 |
| regra já escrita em texto e obedecida | não | p.111 |
| orçamento curto e formato simples | não; descreva o formato em palavras | p.106 |

## 4. Inertness e formato de snippet

Inertness é manter o conteúdo colado **inerte**: o snippet deve ser lido como dado do escopo dele, não
como continuação da instrução.

- **O modelo usa tudo que recebe**: "the model often feels compelled to use every bit of information it
  gets" (p.116) — snippet que não serve à resposta desloca a resposta.
- **Colagem não é soma de tokens**: "Pasting strings together doesn't mean the arrays of tokens just get
  concatenated" (p.148). A borda entre dois snippets pode fundir tokens (1+1 → 1) ou quebrar em três
  (1+1 → 3), então o custo do prompt não é a soma dos custos dos pedaços.
- **Separe elementos por whitespace** e não confie em concatenação direta de strings (p.148).
- **Comece o elemento com espaço**: "prefer prompt elements that start with a space rather than ending with
  one" (p.149) — elemento que termina com espaço/newline contamina o vizinho.
- **Formato de snippet é contrato**: o bloco deve parecer o documento de origem (heading, cerca, lista), e
  não um formato proprietário que o modelo nunca viu (p.81).
- **Inertness não substitui delimitação** de início/fim da resposta: ver §6.

## 5. Posição, importância e dependência

Modele o prompt como conjunto de elementos declarados, não como string montada na ordem em que o código
foi escrito.

- **Três relações por elemento**: "has to account for three ways in which the elements relate to each
  other: position and ordering, importance, and dependency" (p.151). Para cada elemento declare posição
  fixa, tier de importância e de quais outros ele depende.
- **Recência domina**: "The closer a piece of information is to the end of the prompt, the more impact it
  has on the model" (p.135); o meio é onde a instrução é ignorada ("Valley of Meh", p.135).
- **Sandwich**: instrução no início, contexto no meio, refocus no fim — o refocus nomeia o que fazer agora
  (p.135-136).
- **Dependência é acoplamento**: snippet e a instrução que explica como usá-lo viajam juntos; cortar um
  sem o outro deixa o par quebrado (p.151). Requisitos e incompatibilidades fazem parte da declaração do
  elemento.
- **O teto é a janela menos a resposta**: o orçamento de elementos é "your context window size minus the
  tokens needed for the model's response" (p.153). O algoritmo de montagem e a política de corte ficam em
  `context-guard`; aqui a regra é só declarar posição e dependência antes de cortar.

## 6. Contrato de output parseável

- **Defina o começo da resposta**: start reconhecível antes de escrever o contrato (p.161). O preâmbulo
  pode ser longo — "having a long preamble is a virtue, not a vice" (p.158) — desde que exista marcador que
  separe preâmbulo de resposta.
- **Escolha o terminador pelo teste de fim**: a tabela de p.161 classifica os formatos pela coluna "Test
  for end is test for substring" (Yes/No). Escolha o formato que responde Yes para o seu caso; indentação
  não é terminador testável por substring (p.161-162).
- **Fim testável por substring**: "you can determine the end by checking for specific substrings" (p.162)
  — heading de seção, cerca de código e item numerado são terminadores válidos; "qualquer cabeçalho" não é
  (o parser precisa do literal exato).
- **Feche o bloco de entrada com marcador improvável** para o modelo saber onde a saída começa
  (AI Engineering, p.243).
- **Declare o formato exato**: chaves do JSON, inteiro versus fracionário, ausência de preâmbulo
  (AI Engineering, p.243).
- **Prefira o formato de exemplo mais econômico** quando a performance for equivalente: 38 versus 27
  tokens no caso citado (AI Engineering, p.242).
- **Um contrato por resposta**: se a saída tem duas partes (rótulo + justificativa), o contrato precisa
  ordenar as duas; ver §7.

## 7. Classificação com opções disjuntas

- **Primeiro token único por opção**: "you need to make sure each option starts with a unique token"
  (p.165). `North America` versus `Northeast Asia` compartilham prefixo de token e as probabilidades das
  duas opções se somam, fazendo o modelo escolher a alternativa que ele considera pior.
- **Proíba múltiplas alternativas**: "avoid waffling answers that include several alternatives" (p.165).
- **Enumerado com rótulo antes da justificativa**: `1. [negative | positive | neutral], 2. [explicação]`
  (p.165) — o primeiro token carrega a decisão e a justificativa vem depois.
- **Rótulo único é correção de prompt, não de modelo**: se as classes não puderem começar com tokens
  distintos, reescreva os rótulos antes de culpar o modelo (p.165).
- **Rode o gate de 4 critérios** depois de mexer nos rótulos; opção nova pode quebrar o critério 3 (p.81-82).

## 8. Logprobs como gate de confiança

- **logprob não é probabilidade**: "To convert a logprob to a standard probability, you use the exp
  function" (p.163).
- **Agregue os primeiros tokens**: "averaging the probabilities ... of early tokens in the completion is
  predictive of overall quality" (p.164). A média simples dos primeiros tokens já é sinal preditivo.
- **Aja por limiar, não por intuição**: "Include warnings when the model struggles more than usual"
  (p.164) — as opções são não mostrar, avisar, recuperar mais contexto ou escalar para modelo melhor
  (p.163-164).
- **Gate ≠ verdade**: logprob baixo indica hesitação do modelo, e logprob alto não valida o conteúdo. Nunca
  use esse número para justificar a resposta; use para decidir se ela é exibida com aviso (p.163-164).
- **O limiar é política de produto**: decidir entre exibir, avisar ou escalar é escolha de quem opera a
  feature; comparar duas políticas por score é papel de `llm-eval` (p.163-164).

## 9. Diagnóstico de revisão

Sintomas que aparecem em revisão de prompt e a regra que os resolve. Se o sintoma não estiver aqui, meça
antes de mexer: comparação de versões por score é `llm-eval`.

| Sintoma | Causa provável | Regra | Evidência |
| --- | --- | --- | --- |
| resposta reafirma o problema e não resolve | prompt posiciona o modelo como "problem poser" | escrever o início da resposta | p.136 |
| análise extrapola o que foi pedido | escopo implícito | arquétipo de reporte + negação explícita | p.140 |
| texto divaga e não termina | falta ponto de parada | seção sentinela + terminador testável | p.141, p.161-162 |
| modelo continua o documento de entrada em vez de responder | falta marcador de fim do bloco de entrada | fechar a entrada com marcador improvável | AI Eng. p.243 |
| saída com duas alternativas na mesma resposta | opções com prefixo de token comum | primeiro token único por opção | p.165 |
| parser captura conteúdo a mais | delimitador não testável por substring | heading/cerca literal em vez de indentação | p.161-162 |
| few-shot copiado literalmente na saída | exemplo carrega pattern não intencional | gatilho do §3 e exemplos mínimos | p.104, p.111 |
| resposta muda ao reordenar snippets | snippet não inerte ou dependência quebrada | separação por whitespace e par snippet+instrução | p.148, p.151 |
| resposta não usa o dado que você colou | dado longe do fim ou fora do refocus | recência + sandwich | p.135-136 |
| correção errada exibida com confiança | nenhuma política sobre logprob | gate por limiar | p.163-164 |

## Fontes

- Berryman & Ziegler, *Prompt Engineering for LLMs* — cap. 4 Designing LLM Applications (p.81-82); cap. 5
  Prompt Content (p.101-102, p.104, p.106, p.108-109, p.111, p.116); cap. 6 Assembling the Prompt
  (p.135-137, p.140-141, p.143, p.147-149, p.151, p.153); cap. 7 Taming the Model (p.158, p.161-165).
- Huyen, *AI Engineering* — cap. 5 Prompt Engineering: Prompt Engineering Best Practices (p.240-243).
