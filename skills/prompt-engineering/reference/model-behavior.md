# model-behavior

Comportamento mecânico do modelo que muda **decisão de uso** de prompt e contexto. Não é aula de
arquitetura: cada seção termina na consequência prática. Playbook de decodificação (greedy, beam,
temperature, top-k/top-p) fica em [generation-params](generation-params.md); orçamento e compactação de
janela ficam em `context-guard`.

Marcadores `p.N` vêm do digest da fonte, não da página impressa: no digest de *Prompt Engineering for LLMs*
o marcador está ~14-15 páginas acima do número impresso (cap. 6 impresso "123" = marcador p.133). Citação
sem prefixo de livro é de *Prompt Engineering for LLMs*; `NLPwT` = *Natural Language Processing with
Transformers*; `AI Eng.` = *AI Engineering*.

## 1. O modelo vê tokens, não texto

O texto entra no modelo como sequência de IDs de token, produzida por um pipeline de quatro estágios —
normalization, pretokenization, tokenizer model, postprocessing (NLPwT p.114). Trocar o tokenizer é trocar o
significado dos IDs: "switching the tokenizer is like shuffling the vocabulary" (NLPwT p.57), e "you can't
mix and match tokenizers and models" (p.42).

O que parece igual pode não ser:

- **Espaço antes do elemento muda o token.** A recomendação da fonte é "prefer prompt elements that start
  with a space rather than ending with one" (p.149) — o espaço inicial é parte da fronteira do token, não
  decoração. Colar duas strings também não concatena as listas de token: a tabela de p.148 mostra junções
  1+1➜1 e 1+1➜3, porque "pasting strings together doesn't mean the arrays of tokens just get concatenated".
- **Palavra comum pode sair partida.** "Christa" vira `Chr` + `##ista` (NLPwT p.117); tokenizer de outro
  domínio desconhece palavra comum do seu (T5 nunca viu "sex", CamemBERT não conhece "being") e parti-la
  "will increase the input sequence length" (NLPwT p.330).
- **Acento, emoji e símbolo não têm token próprio garantido.** A cobertura de todo Unicode é garantida pelo
  alfabeto de bytes, com 256 símbolos (NLPwT p.333); cobertura não é o mesmo que um token por caractere — o
  que aparece fora do vocabulário é fatiado, e o comprimento da sequência sobe (NLPwT p.330).
- **Notação e código custam mais por caractere.** A regra de bolso da fonte é ~4 caracteres por token em
  inglês e ~2 em código/chaves (p.42).

Custo e limite se medem em token, não em caractere: "so you need to count tokens to stop that from
happening" (p.42). Contar letra, inverter string ou alinhar caractere-a-caractere exige a representação que
o modelo não recebe (NLPwT p.117; p.148); é derivação da mecânica, não medição no digest.

**Consequência para o uso:** dois templates que parecem iguais podem custar e performar diferente — meça
com o tokenizer do modelo antes de comparar versões, comece cada elemento com espaço (p.149), e mande
contagem/inversão/alinhamento de caractere para código, não para o prompt.

## 2. O modelo completa documento, não obedece comando

O modelo foi treinado a continuar texto: "don't stray far from the path upon which the model was trained"
(p.81) e "make your prompts follow the patterns and motifs seen in training data" (p.257). Por isso a
unidade de decisão é o **tipo de documento** que o prompt imita, não a força do imperativo.

Reafirmar o pedido não basta: "the prompt must condition the model to generate a completion that is
actually helpful" (p.82). E tudo que está no documento vira instrução de fato — "LLMs have a compulsion to
continue patterns" (p.104) e "the model often feels compelled to use every bit of information it gets"
(p.116).

A forma do documento também governa o quanto o modelo respeita fronteira: limite declarado é respeitado com
mais consistência em relatório do que em diálogo (p.140).

**Consequência para o uso:** escolha o arquétipo do documento pela forma da resposta desejada antes de
enfeitar a instrução — reporte para escopo rígido, conversa para multi-turno, documento estruturado para
saída parseada (`prompt-craft` §1, p.137, p.140, p.143). Instrução mais forte não conserta arquétipo errado.

## 3. Auto-regressivo e sem revisão

A geração é um token por vez: "Generating text requires at least one forward pass per generated token"
(NLPwT p.160), e o capítulo da fonte chama isso de "One Token at a Time" (p.44). Cada token escolhido vira
condição do próximo, então o começo da resposta ancora o resto — é por isso que escrever o início muda o
comportamento: "change your perspective from problem poser to problem solver and begin writing the answer
for the model" (p.136).

Não há rascunho interno: "language models have no internal monologue" (p.192) — raciocínio que não está no
texto não acontece. E a probabilidade dos primeiros tokens prediz a qualidade do todo: "averaging the
probabilities ... of early tokens in the completion is predictive of overall quality" (p.164).

**Consequência para o uso:** quando o modelo reformula o pedido em vez de resolver, escreva o início da
resposta no prompt (p.136); trate o começo como âncora, não como preâmbulo descartável; leia logprob dos
primeiros tokens como sinal de risco, não como verdade (`prompt-craft` §8, p.163-164).

## 4. Determinismo do tokenizador vs amostragem do modelo

O tokenizador aplica uma regra fixa: a mesma string, no mesmo tokenizer, produz os mesmos IDs, e o round-trip
`decode(encode(x)) == x` devolve o texto "without ambiguity" quando o whitespace é preservado
(NLPwT p.114-115). A saída do modelo não tem essa garantia:

- **Greedy** toma sempre o token de maior probabilidade, um caminho único (NLPwT p.150).
- **Sampling** sorteia da distribuição, e temperature reescala os logits antes do softmax: "When T ≪ 1, the
  distribution becomes peaked ... when T ≫ 1, the distribution flattens out" (NLPwT p.155). Com T=2 a saída
  vira gibberish (NLPwT p.155).
- **Truncar a cauda** (`top_k`/`top_p`) reduz o espaço de sorteio, não o elimina (NLPwT p.158-159); o
  trade-off declarado é coerência versus diversidade (NLPwT p.156).

Logo, mesma entrada **não** garante mesma saída: com qualquer temperature acima de zero, tokens de cauda
entram na amostragem (NLPwT p.155-156). Uma execução não é evidência — "You'll have no automated way of
knowing whether any change is an improvement or a regression" (p.232); julgar por inspeção visual "won't be
sufficient for application iteration" (AI Eng. p.139); e score que oscila entre 90% e 70% em reamostragem
mostra pipeline instável, não modelo instável (AI Eng. p.227).

**Consequência para o uso:** avalie com suíte versionada (5-20 exemplos cobrindo o range real já dão sinal,
p.232), fixe o sampling e versione o prompt fora do código com metadado — modelo, data, schema, sampling — em
catálogo de versões (AI Eng. p.253, p.255), e nunca conclua "melhorou" a partir de uma execução.

## 5. Alucinação é o padrão, não a exceção

A saída é a continuação mais provável do documento, não a mais verdadeira. O custo disso aparece mesmo com a
fonte na mão: na sumarização, o GPT-2 "hallucinates" fatos (NLPwT p.167). Quando a pergunta não tem resposta
no material, o comportamento natural é preencher: o modelo de reescrita "should acknowledge that this query
isn't solvable instead of hallucinating a name" (AI Eng. p.290).

São dois modos distintos, e ambos precisam de teste próprio: consistência factual **local** (contra o
contexto fornecido) e **global** (contra conhecimento aberto) (AI Eng. p.186); os gatilhos reais são "queries
that involve niche knowledge" e "queries asking for things that don't exist" (AI Eng. p.187).

**Consequência para o uso:** exija citação do trecho que sustenta cada afirmação e escreva a ausência como
saída válida no contrato — "se não houver evidência no contexto, responda NAO ENCONTRADO". Trate consistência
local como checagem contra o contexto colado (AI Eng. p.186) e inclua no eval set pergunta cuja resposta não
existe (AI Eng. p.187).

## 6. Janela de contexto é finita e cara

O custo da janela cresce quadrático: self-attention "naively scales like O(n²), where n is the length of the
sequence" (NLPwT p.369), e o limite máximo de contexto vem "due to the quadratic memory requirements of the
attention mechanism" — 512 tokens na maioria dos BERT, 4.096 no BigBird, 8× maior (NLPwT p.84, p.370). O
orçamento real é "your context window size minus the tokens needed for the model's response" (p.153).

Dentro da janela, posição importa: "The closer a piece of information is to the end of the prompt, the more
impact it has on the model" (p.135), e o meio é onde a instrução se perde ("Valley of Meh", p.135); daí o
padrão instrução no início, contexto no meio, refocus no fim (p.135-136).

Contexto irrelevante não é neutro: o modelo "feels compelled to use every bit of information it gets"
(p.116) — "if you retrieve snippets, retrieve the right ones" (p.117). E truncar não é solução neutra: cortar
texto pode remover "crucial information" que estava no fim (NLPwT p.51).

**Consequência para o uso:** ordene o contexto por importância em vez de por ordem de chegada, mantenha
instrução no início e refocus no fim, e omita snippet que não serve à resposta — o modelo vai usá-lo de
qualquer jeito (p.116). Contagem, ordem e compactação são orçamento: `context-guard`.

## 7. O que isso NÃO explica

- **Capacidade de raciocínio não se lê na arquitetura.** A derivação de self-attention e feed-forward
  (NLPwT p.61-73) fica fora deste arquivo; dela só se aproveita o custo O(n²) (NLPwT p.369), que é sobre
  preço de contexto, não sobre qualidade de resposta.
- **Benchmark público não prediz a sua tarefa.** "Public benchmarks will help you filter out bad models, but
  they won't help you find the best models for your application" (AI Eng. p.221), e contaminação é comum — o
  GPT-3 tinha "13 benchmarks with at least 40% in the training data" (AI Eng. p.220), que é o que a fonte
  define como "data contamination happens when a model was trained on the same data it's evaluated on"
  (AI Eng. p.218). Score em MCQ também não é score em geração: "MCQs test the ability to differentiate good
  responses from bad responses (classification), which is different from the ability to generate good
  responses" (AI Eng. p.184).
- **Nada aqui autoriza escolher modelo por tamanho.** A escolha se confirma no seu eval set e sem cravá-la no
  código: "don't bake your choice into your code too firmly" (p.169).

**Consequência para o uso:** use este arquivo para prever *comportamento* (tokenização, completação,
ancoragem, variabilidade, alucinação, custo de janela) e use a sua suíte de avaliação para decidir *qual
modelo e qual prompt servem* (`llm-eval`).

## Fontes

- Berryman & Ziegler, *Prompt Engineering for LLMs* — cap. 2 Understanding LLMs: Counting Tokens p.42; One
  Token at a Time p.44; cap. 4 Designing LLM Applications: p.81-82; cap. 5 Prompt Content: p.104, p.116-117;
  cap. 6 Assembling the Prompt: p.135-137, p.140, p.143, p.148-149, p.153; cap. 7 Taming the Model:
  p.163-165, p.169; cap. 8 Conversational Agency: p.192; cap. 10 Evaluating LLM Applications: p.232; cap. 11
  Looking Ahead: p.257.
- Tunstall, von Werra & Wolf, *Natural Language Processing with Transformers* — cap. 3 Transformer Anatomy:
  p.84 (limite de contexto), p.61-73 (derivação de atenção, descartada); cap. 4 Multilingual NER: p.114-117;
  cap. 5 Text Generation: p.150, p.155-160; cap. 6 Summarization: p.51, p.167; cap. 10 Training from Scratch:
  p.330, p.333; cap. 11 Future Directions: p.369-370.
- Huyen, *AI Engineering* — cap. 2 Understanding Foundation Models: Sampling p.105, The Probabilistic Nature
  of AI p.127, Hallucination p.129; cap. 3 Evaluation Methodology: p.139, p.227; cap. 4 Evaluate AI Systems:
  p.184, p.186-187, p.218, p.220-221; cap. 5 Prompt Engineering: p.253, p.255; cap. 6 RAG and Agents: p.290.
- Todos os números são marcadores do digest, não páginas impressas.

## O que não usar deste arquivo

Não infira capacidade por tamanho de modelo, por score de benchmark ou por "contexto maior": nada aqui mede o
seu domínio. Não use o mecanismo para justificar prompt mais esperto — arquitetura explica custo e
variabilidade, não garante raciocínio. Para decidir entre dois candidatos, a evidência é o seu eval set
(`llm-eval`), não os capítulos citados.
