# token-economics

Custo medido das tres ferramentas de reducao (`repo-map`, `ast-outline`, `trace-strip`) e dos
subcomandos equivalentes do `ctx-window`, com o ponto onde otimizar deixa de pagar. Todos os numeros
abaixo foram medidos no proprio repo agent-sync (526 arquivos, 1778 simbolos, `repo-map --summary`) e o
comando esta junto de cada linha. Nao ha estimativa de fatura de provedor: o que se mede aqui e tamanho de
saida (que vira token no prompt) e wall clock.

## 1. Unidade de cobranca

- O que chega ao modelo e token, nao byte. Referencia do livro: ~4 chars/token em ingles e ~2 em
  codigo/chaves, e a contagem **nao e aditiva** entre snippets (Prompt Engineering, p.148; p.42).
- Consequencia para as tabelas abaixo: dividir bytes por 4 subestima o custo de codigo. Um recorte de
  2.242 bytes de `repo-map --focus` sobre um `.go` custa mais perto de 1.100 tokens do que de 560.
- Nenhuma ferramenta do repo conta token real: `EstimatedChars` soma chars (`store.go:90-96`) e o
  `--max-tokens` do `repo-map --summary` conta palavras (`repomap.go:1148-1156`). Marque como divergencia
  sempre que comparar com budget do provedor.
- Ordem de grandeza de um turn: um tool call vira ~16000 chars no maximo (`summarize.go:336-346`), o que
  ja e ~4k tokens por leitura grande de arquivo — mais caro que todas as saidas otimizadas desta pagina somadas.

## 2. Custo medido por operacao

Rodado com `--cache-dir /tmp/p4bench/cache` para nao escrever cache no repo.

| Operacao | Tempo | Saida | Entrada equivalente | Ganho |
| --- | --- | --- | --- | --- |
| `repo-map --update` (cache frio) | 413 ms | — | 526 arquivos varridos | — |
| `repo-map --update` (sem mudanca) | 64 ms | — | delta vazio | 6,5x mais rapido que frio |
| `repo-map --summary` (default 1000) | 33 ms | 4.288 B | — | -93% vs sem limite |
| `repo-map --summary --max-tokens 500` | 43 ms | 2.180 B | — | -96% vs sem limite |
| `repo-map --summary --max-tokens 2000` | 34 ms | 8.429 B | — | -85% vs sem limite |
| `repo-map --summary --max-tokens 0` | 33 ms | 57.442 B (~14k tokens) | — | **nenhum** |
| `repo-map --focus <arquivo>` | 31 ms | 2.242 B | 17.620 B (`store.go`) | -87% |
| `repo-map --brief <arquivo>` | 29 ms | 3.685 B | 17.620 B | -79% |
| `ast-outline <arquivo>` | 2 ms | 1.276 B / 39 linhas | 17.620 B / 592 linhas | -93% |
| `ctx-window skeleton <arquivo>` | 3 ms | 2.001 chars, `reduction_pct 88` | 17.620 B | -88% |
| `trace-strip` (stack trace sintetico) | 4 ms | 6 linhas mantidas | 8 linhas | 2 linhas ocultadas |

Fatos de custo que a tabela esconde:

- `--summary --max-tokens 0` e o unico caminho que **piora** o contexto: 57.442 B (~14k tokens) em 1236 hubs
  neste repo. Nao e otimizacao, e despejo — por isso o default e 1000, com aviso de truncamento na ultima
  linha. Nunca passe `0` por curiosidade.
- O cache do `repo-map` pesa 2.887.723 B em disco. Custa disco, nao contexto — a economia esta no
  `--focus`/`--brief`, nao no cache.
- `--max-tokens` conta `len/4` tanto no `--summary` quanto no `--brief` (`repomap.go:789-791`). Antes o
  `--summary` contava palavras e entregava ~2,4x o valor pedido; corrigido em 2026-11-16.
- `ast-outline` so usa AST real em Go (`astoutline.go:25-27`); `.py/.js/.ts/.php` caem em regex
  (`astoutline.go:29-35`). O ganho de 93% nao garante esqueleto correto fora de Go.
- `skeleton` so economiza acima de 8 KiB (`skeletonMinBytes`, `skeleton.go:29-31`); abaixo disso devolve o
  arquivo inteiro com `reduction_pct 0`, ao custo de uma chamada de ferramenta.
- `expand` reidrata o original do archive: o ganho do arquivamento e temporario e so se sustenta enquanto
  ninguem pede o conteudo de volta (`--help` de `ctx-window archive|expand`; limites em `archive.go:27-32`).
- Cada tool call custa ate 16000 chars na working memory, independente do tamanho do arquivo lido
  (`summarize.go:336-346`). O `AUTO_ARCHIVE` opt-in troca o resultado grande por preview de 500 chars e
  guarda o resto em disco (`archive.go:27-32,210-214`): e a unica alavanca que corta custo **por numero de
  tool calls**, nao por tamanho de leitura.

## 3. Quando cache compensa

- **Cache de artefato do `repo-map`: sim, a partir da segunda leitura.** Frio 413 ms, quente 64 ms. Se a
  sessao vai fazer uma unica busca, `--update` e puro custo.
- Divergencia com o `SKILL.md` desta skill: ele anuncia cache "<20ms"; medido aqui no repo agent-sync,
  `--summary` levou 33 ms e `--update` sem mudancas 64 ms. Use os numeros medidos, nao o do SKILL.
- **Cache de esqueleto: sim, por fingerprint.** `skeleton` grava por mtime+tamanho+hash com `from_cache` no
  JSON (`skeleton.go:31,44-54`): a segunda chamada no mesmo arquivo nao re-parseia.
- **Cache de prompt: nao existe no repo.** O livro trata prompt caching como o ganho grande — segmentos
  invariantes processados uma vez, com "up to 90% cost savings" e "up to 75% latency reduction"
  (AI Engineering, p.463-464; p.481: nao cachear query presa a usuario; p.482: cache semantico tem
  componentes frageis). O agent-sync cacheia **arquivo e esqueleto**, nao prefixo de prompt: o system
  prompt e as regras sao reenviados a cada turno sem cache explicito. Divergencia, e a de maior valor
  absoluto no custo por sessao.
- Criterio do livro para cachear: o segmento se repete em toda chamada (p.463). Regras + skill carregada +
  STATE.md satisfazem isso e hoje nao sao cacheados por nenhuma camada do repo.

## 4. Ponto de retorno decrescente

- Corte grande e de graca; corte fino e caro. Mesmo arquivo: 17.620 B -> 2.242 B (`focus`, -87%) ->
  1.276 B (`ast-outline`, -93%). Os 6 pontos percentuais finais (87% -> 93%) economizam 966 B (~240 tokens)
  e custam uma chamada adicional de ferramenta.
- No `--summary`, o primeiro corte remove 94% (81.308 -> 4.638 B). Subir de 500 para 2000 "tokens" multiplica
  a saida por 4,2 (4.638 -> 19.575 B) com latencia identica (43 ms vs 34 ms, ambas dominadas pelo cache).
  Headroom extra compra detalhe linearmente, nao economia.
- Referencia de razao aceitavel para resumo: o resumo deve ficar em ~1/10 do original, e o custo de
  sumarizar e dominado pelo texto de entrada (Prompt Engineering, p.128). Aplicado ao `--summary` livre:
  alvo ~8 KB. O corte de 2000 palavras (19,5 KB) ja esta acima disso; 500 palavras (4,6 KB) esta abaixo e
  e o ponto seguro.
- Custo fixo por chamada domina abaixo de ~40 ms: `ast-outline` (2 ms) e `trace-strip` (4 ms) sao tao
  baratos que o gargalo passa a ser o numero de chamadas, nao o tamanho de cada uma.

## 5. O que nao vale a pena otimizar

- **`trace-strip` em texto que nao e stack trace.** Ele so descarta linha que casa com 3 regexes
  (`node_modules|vendor/|site-packages|/usr/local/go|/lib/python`, `internal/process|runtime/asm_|runtime/proc.go`,
  `org.springframework|jakarta.|javax.`) e corta em `-max 25` (`tracestrip.go:12-16`). Em log comum o ganho e zero.
- **`ast-outline` em arquivo de 40 linhas.** O custo de decidir ja e maior que a leitura; o ganho cresce com o
  tamanho do arquivo.
- **`skeleton` em arquivo < 8 KiB**: devolve tudo (`skeleton.go:29-31`).
- **`repo-map --update` numa sessao de leitura unica**: 413 ms e 2,9 MB para economizar uma busca.
- **Quantizacao / INT8 / destilacao.** O livro recomenda dynamic INT8 como primeiro degrau (~4x menos
  memoria) e escolher a menor precisao toleravel, reavaliando metrica no mesmo test set (NLP with
  Transformers, p.255-256). Nao se aplica aqui: o repo nao roda inferencia de transformer local — o custo e
  token de API. Otimizar compute local nao reduz a fatura. Divergencia deliberada, nao divida tecnica.
- **Otimizar `quality`, `gain`, `activity`**: sao metadados de alguns KB. O ganho e ruido.

## 6. Geracao e latencia: o que o repo controla

- **Stop sequences: nao controladas.** O livro manda passar `stop` com terminador logico (`\n#`, `\nclass`,
  `\ndef`) e cancelar o stream (Prompt Engineering, p.162-163). `knownCLIs` so define binario, args estaticos
  e flag de modelo — sem `stop` (`summarize.go:71-80`). Toda geracao do `summarize` roda ate o modelo parar.
  Divergencia marcada.
- **Latencia: medida pontual, nao distribuicao.** O livro exige TTFT e TPOT separados em p50/p90/p99 e
  goodput (`total = TTFT + TPOT x tokens de saida`; media engana) (AI Engineering, p.433-435). Os tempos
  desta pagina sao **uma amostra** de wall clock; nao ha percentil, nao ha separacao prefill/decode. Nao cite
  estes numeros como SLO.
- **Telemetria existente:** so o MCP do `repo-map` tem canal de metrica (`--telemetry-file` ou
  `AGENT_SYNC_GRAPH_TELEMETRY=1`, conforme `repo-map --help`). Nao ha telemetria de token por sessao.
- **Tracking de economia:** `ctx-window gain <session>` le `gain.jsonl` com `before_chars/after_chars/
  saved_chars` e `saved_tokens = chars/4` (`gain.go:15-24`). Como a compactacao nao altera `s.Turns`
  (`store.go:301-315`), o `saved_chars` de uma compactacao e 0 — nao use `gain` para provar economia de
  compactacao.

## 7. Regras acionaveis

- Sempre passe `--max-tokens` no `repo-map --summary`; 500 e o ponto medido de melhor razao ganho/headroom.
- Prefira `--focus` a `--brief` quando quiser texto curto para o modelo: 2.242 B contra 3.685 B no mesmo arquivo.
- Use `ast-outline` como etapa de localizacao e depois leia a faixa exata; nao trate o esqueleto como
  substituto do arquivo fora de Go.
- Nao rode `repo-map --update` para uma leitura; rode quando houver 3+ buscas estruturais na sessao.
- Meca antes de otimizar: `reduction_pct` esta no JSON de `skeleton` — decida pelo numero, nao pela sensacao.
- Nao apresente wall clock de uma amostra como garantia de performance; para isso faltam percentis.

## 8. Verificacao rapida

```bash
repo-map --summary | wc -c                                  # 4,3k (default 1000 tokens)
repo-map --summary --max-tokens 500 | wc -c                 # 2,2k
repo-map --focus tools/cmd/ctx-window/store.go | wc -c      # 2,2k
ast-outline tools/cmd/ctx-window/store.go | wc -lc          # 1,2k / 39 linhas
ctx-window skeleton --json <arquivo> | grep reduction_pct   # 0 abaixo de 8 KiB
ctx-window gain <session>                                   # o que a compactacao contabilizou
```

## Fontes

- Prompt Engineering for LLMs (Berryman & Ziegler) — Cap. 2 "Counting Tokens" p.42; Cap. 5 "Summarization"
  p.128 (razao do resumo e custo dominado pela entrada); Cap. 6 "More on Inertness" p.148 (chars por token,
  contagem nao aditiva); Cap. 7 "Recognizable Start and End" p.162-163 (stop sequences).
- AI Engineering (Chip Huyen) — Cap. 9 "Inference Performance Metrics" p.433-435 (TTFT/TPOT, percentis,
  goodput) e "Inference Service Optimization" p.460, 463-464 (prompt caching); Cap. 10 "Step 4 Caches"
  p.480-482 (o que nao cachear).
- NLP with Transformers (Tunstall et al.) — Cap. 8 "Making Models Faster with Quantization" p.255-256
  (INT8 ~4x, menor precisao toleravel, reavaliar no mesmo test set).
