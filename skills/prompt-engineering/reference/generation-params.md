# generation-params

Parâmetros de decodificação: como a distribuição de probabilidade do modelo vira texto. Escolha por tipo
de tarefa e por custo, não por default herdado de tutorial. Base: cap. 5 (Text Generation) de *Natural
Language Processing with Transformers*, seções `Greedy/Beam Search` (p.148-153) e `Top-k and Nucleus
Sampling` (p.156).

Contexto: qualquer política de exibição baseada em confiança é `prompt-craft` §8 (logprobs); custo de
janela e de contexto é `context-guard`.

## 1. Ordem de decisão

Decida nesta ordem, parando no primeiro item que resolver a tarefa:

1. **A saída é verificável e precisa ser determinística?** (aritmética, extração, resposta factual curta)
   → greedy, ou greedy combinado com beam (p.159-160).
2. **A saída é texto aberto e repetitiva com greedy?** → beam com penalidade de n-gram repetido, ou
   sampling (p.150, p.153-154).
3. **A saída aberta precisa de diversidade controlada?** → truncar a cauda (`top_k` / `top_p`) e só então
   ajustar temperature (p.155-159).
4. **Sempre**: fixe `max_new_tokens` coerente com o maior caso real; não há decoder que "termine sozinho"
   de graça (p.160).

Não existe escolha universal: "there is no universally 'best' decoding method" (p.159-160).

## 2. Greedy

- **O que faz**: a cada passo toma o token de maior probabilidade; um único caminho, sem exploração.
- **Custo mínimo**: um forward pass por token gerado (p.160).
- **Defeito estrutural**: "one of the main drawbacks with greedy search decoding: it tends to produce
  repetitive output sequences" (p.150) — em texto livre o modelo entra em loop de frase.
- **Onde serve**: tarefa determinística, resposta curta, saída que você vai parsear (p.159-160).
- **Não resolve com temperature**: temperature reescala a distribuição antes do softmax (p.155); em greedy
  a escolha é o token de maior probabilidade, então reescalar não troca o token escolhido. Para mudar o
  comportamento em texto livre é preciso trocar o decoder (p.150, p.153).

## 3. Beam search

- **O que faz**: mantém `num_beams` caminhos simultâneos e escolhe a sequência de maior probabilidade
  conjunta; não há sorteio entre candidatos, então a mesma entrada tende ao mesmo resultado (p.153).
- **Custo**: "Generating text requires at least one forward pass per generated token, and even more if we
  use beam search" (p.160) — beam multiplica o compute por caminho mantido.
- **Não elimina repetição**: "beam search also suffers from repetitive text" (p.153). Beam melhora a
  escolha da sequência, não a diversidade.
- **Onde serve**: tradução, sumarização e tarefa precisa em que você quer a sequência mais provável, não
  a mais criativa (p.159-160).
- **Cuidado com beam grande**: paga-se N× compute por ganho decrescente; justifique `num_beams` por medição,
  não por default (p.160).

## 4. Repetição: penalidade de n-gram

- **Solução para repetição de decoder determinístico**: "One way to address this is to impose an n-gram
  penalty" (p.153).
- **Efeito medido**: com `no_repeat_ngram_size=2` o log-prob cai de −55.23 para −93.12 e a repetição cessa
  (p.154) — o texto fica menos provável e menos repetitivo; é exatamente o trade-off aceito.
- **Custo da penalidade**: bloquear n-gram repetido reduz o espaço de continuação. n=2 é o valor medido no
  livro (p.154); n maior proíbe mais sequências e passa a vetar repetição legítima de termos técnicos
  (p.153-154).
- **Alternativa real**: trocar para sampling com truncamento quando a tarefa aceita variação (p.155-159).

## 5. Temperature

- **O que faz**: reescala os logits antes do softmax — "When T ≪ 1, the distribution becomes peaked ...
  when T ≫ 1, the distribution flattens out" (p.155).
- **Efeito por faixa**: T baixa concentra massa nos tokens prováveis; T alta espalha massa e passa a
  amostrar tokens improváveis — com T=2 a saída vira gibberish (p.155).
- **O trade-off**: "a trade-off between coherence (low temperature) and diversity (high temperature)"
  (p.156). Não existe T "correto"; existe T coerente com a tarefa.
- **Erro comum**: subir temperature buscando criatividade sem truncar a cauda — o ganho vem do tokens de
  cauda entrarem na amostragem, e é isso que produz palavra inventada (p.155-156).
- **Ordem correta**: truncar primeiro (`top_k`/`top_p`), ajustar temperature depois (p.158-159). Temperature
  sozinha não remove a cauda improvável que você queria excluir.

## 6. top-k e top-p (nucleus)

| Parâmetro | Critério de corte | Força | Limite |
| --- | --- | --- | --- |
| `top_k` | mantém os k tokens mais prováveis | simples, previsível | corte fixo ignora a forma da distribuição: "that fixed cutoff might not be very satisfactory" (p.158) |
| `top_p` | mantém o menor conjunto que soma massa p | adapta o número de candidatos por passo | depende de `p` calibrado; em distribuição achatada mantém muitos tokens |

- **`top_k` evita a cauda**: "top-k sampling is to avoid the low-probability choices by only sampling from
  the k tokens with the highest probability" (p.158).
- **Combinar os dois**: "`top_k=50` and `top_p=0.9` corresponds to ... probability mass of 90%, from a pool
  of at most 50 tokens" (p.159) — o teto de candidatos vem do k, a massa vem do p.
- **Sem truncamento não há controle**: com qualquer temperature acima de 0, tokens de cauda improvável
  entram na amostragem (p.155-156).
- **Calibre p e k juntos**: mexer em um muda o efeito do outro; registre o par usado por tarefa (p.159).

## 7. Escolha por tarefa

| Tarefa | Decoder | Parâmetro que importa | Evidência |
| --- | --- | --- | --- |
| aritmética, extração, resposta factual curta | greedy ou greedy + beam | determinismo; sem sampling | p.159-160 |
| saída parseável (JSON, rótulo) | greedy/beam | nenhuma variação desejada | p.159-160 |
| tradução, sumarização | beam | `num_beams` justificado por medição | p.153, p.160 |
| texto aberto curto com repetição | beam + `no_repeat_ngram_size` | penalidade de n-gram | p.153-154 |
| texto aberto criativo | sampling com `top_k`/`top_p` + temperature | massa de cauda truncada | p.155-159 |
| chat/diálogo aberto | sampling truncado | coerência versus diversidade | p.156 |

Regra de bolso: quanto mais fácil é verificar a saída automaticamente, mais perto de determinístico o
decoder deve ficar (p.159-160).

## 8. Custo por token gerado

- **Piso de latência**: "Generating text requires at least one forward pass per generated token, and even
  more if we use beam search. This makes text generation computationally demanding" (p.160).
- **Orçamento = tokens de entrada + tokens de saída × passes por token**: o termo que escala por
  requisição é tokens de saída × passes por token, isto é, `max_new_tokens` × `num_beams` (p.160).
- **Consequência prática**: cortar `max_new_tokens` e evitar beam em tarefa que não precisa reduz custo
  direto; o custo de saída é o que escala por requisição (p.160).
- **Meça antes de fixar**: escolha de decoder é decisão por tarefa; a comparação de qualidade entre dois
  decoders exige baseline e eval set, fora do escopo desta skill (p.159-160).

## 9. Diagnóstico por sintoma

| Sintoma na saída | Causa provável | Ação | Evidência |
| --- | --- | --- | --- |
| loop de frase, texto se repete | greedy (ou beam) em texto aberto | trocar decoder; penalidade de n-gram | p.150, p.153-154 |
| repetição mesmo com beam | beam também repete | `no_repeat_ngram_size` ou sampling | p.153-154 |
| palavra inventada, texto incoerente | T alta sem truncamento de cauda | reduzir T e truncar antes com `top_k`/`top_p` | p.155-156 |
| texto sempre igual, sem variação | T baixa / greedy | subir T com cauda truncada, se a tarefa aceita diversidade | p.156 |
| candidatos ruins mesmo com k alto | corte fixo não acompanha a distribuição | usar `top_p` (ou combinar com `top_k`) | p.158-159 |
| resposta boa ocasional perdida | 1 caminho só | beam com k justificado | p.153, p.160 |
| custo/latência acima do previsto | `num_beams` ou `max_new_tokens` altos | reduzir beam e teto de tokens | p.160 |

## Fontes

- Tunstall, von Werra & Wolf, *Natural Language Processing with Transformers* (Revised Edition) — cap. 5
  Text Generation: `Greedy/Beam Search` (p.148-153; greedy em p.150, beam e penalidade de n-gram em
  p.153-154), temperature (p.155-156), `Top-k and Nucleus Sampling` (p.156-159), custo de geração (p.160).
