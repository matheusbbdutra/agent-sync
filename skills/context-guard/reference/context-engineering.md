# context-engineering

Registro do que o `ctx-window` **mede e decide hoje** no agent-sync, com o ponto exato onde o
comportamento diverge da recomendacao de livro. Nao e proposta de design: todo numero abaixo sai de
`--help`, de `file:line` do repo ou de comando rodado (comando citado junto).

Complementa `context-window-strategy.md`, que descreve o protocolo (K, summarizer, formato do resumo).
Aqui o assunto e o medidor, os limiares e as lacunas.

## 1. A unidade medida e caractere, nao token

- `Session.EstimatedChars()` soma `len(Content) + len(Role) + 32` por turn (`tools/cmd/ctx-window/store.go:90-96`).
  Nao ha tokenizer no caminho: nenhum limiar do binario conhece o vocabulario do modelo.
- O limiar de compactacao e **fixo em 200 chars** (`compactAtThreshold`, `store.go:172-181`). Existia o knob
  `AGENT_SYNC_CTX_COMPACT_AT`, removido em 2026-11-16 junto com o subsistema `on-tool-call`: ele nao alcancava
  o fluxo vivo e so servia de seam de teste.
- Efeito pratico: 200 chars ~ 50 tokens, ordem de grandeza de um unico tool call. Medido nas sessions locais:
  `~/.cache/agent-sync/ctx-window` tem 321 sessoes, 273 com `turns.jsonl`, e **264 delas (96,7%) passam de 200
  bytes** (`find . -name turns.jsonl -size +200c`). O default nao separa sessao saudavel de sessao cheia.
- Consequencia no score: `ctx-window quality 007c7baa-...` devolveu `0 (w=25%) 5924/200 chars (over threshold)`
  e `grade: F`. `context_fill` zera o sinal quando `fill >= 2*threshold` (`quality.go:114-132`).
- O unico ponto do repo que le token de verdade e o nudge do Claude, que le
  `message.usage.input_tokens|cache_creation_input_tokens|cache_read_input_tokens` do transcript
  (`claude_usage.go:13-24`), com tail de 2 MiB (`transcriptTailBytes = 2 << 20`).

## 2. Limiares reais e variaveis de ambiente

| Decisao | Variavel / constante | Default | Onde |
| --- | --- | --- | --- |
| Tamanho da working memory | `AGENT_SYNC_CTX_K` | 5 turns | `store.go:423-431` |
| Budget do resumo | `AGENT_SYNC_CTX_BUDGET` | 1000 tokens | `store.go:434-442` |
| Nudge Claude (uma vez por sessao) | `AGENT_SYNC_CTX_NUDGE_TOKENS` | 150000 tokens | `claude_usage.go:14` |
| Nudge Antigravity (preinvocation) | `AGENT_SYNC_CTX_NUDGE_INVOCATIONS` | 30 invocacoes | `hook.go:315-321` |
| Nudge pos-tool (`ctx-window-nudge.sh`) | `..._MIN_TOOL_CALLS` / `..._MAX_AGE_HOURS` | 80 / 4h | `hooks/ctx-window-nudge.sh:22-23` |
| Cadencia do mirror para `memory-mcp` | `..._MIRROR_THRESHOLD` | 25 | `hooks/ctx-window-nudge.sh:75` |
| Nudge do guard (injeta `additionalContext`) | `AGENT_SYNC_NUDGE_THRESHOLD` | 120 (modulo) | `hooks/context-guard-nudge.pretooluse.sh:13,40` |
| Auto-summarize no Stop | `..._SUMMARIZE_MIN_TOOL_CALLS` / `..._MAX_AGE_HOURS` | 100 / 4h | `hooks/ctx-window-summarize-at-stop.sh:23-24` |
| Arquivamento de tool result | `AGENT_SYNC_CTX_ARCHIVE_AT` | 4096 bytes | `archive.go:31,36-45` |
| Arquivamento automatico (opt-in) | `AGENT_SYNC_CTX_AUTO_ARCHIVE` | desligado (exige `1`) | `archive.go:210-214` |
| Deteccao de loop | `AGENT_SYNC_CTX_LOOP_WINDOW` / `..._LOOP_MIN` | 10 / 3 | `store.go:107-170` |
| Desligar o nudge de ctx | `AGENT_SYNC_CTX_WINDOW_NUDGE_DISABLE` | — | `hooks/ctx-window-nudge.sh:17-20` |

Leituras que o `--help` induz a erro:

- `AGENT_SYNC_CTX_BUDGET` **nao trunca nada**. `s.Budget` so e interpolado no texto do prompt
  (`summarize.go:170`, `buildSummarizePrompt`); nao existe corte por budget na saida do LLM.
- O nudge de `ctx-window-nudge.sh` nunca injeta contexto: emite `{}` nos dois caminhos (nao-trigger e trigger),
  por limitacao de schema do Antigravity (`hooks/ctx-window-nudge.sh:96-105`). O unico ponto que injeta e o
  `context-guard-nudge.pretooluse.sh`, via `additionalContext` (e via stderr no Antigravity).

## 3. O que cada subcomando decide

- `on-tool-call-llm` / `hook <cli>`: grava o turn com input truncado em 16000 chars apos `redactSecrets`
  (`summarize.go:336-346`) e **nunca compacta** — o comentario de `hook.go:60-63` diz isso explicitamente.
- `compact`: forca a heuristica local sem LLM (`main.go:148-176`). `AppendVersionedSummary` escreve
  `summary.md` + `summary_v{N}.md` e incrementa `Version`, **sem tocar em `s.Turns`** (`store.go:301-315`).
  Logo `beforeChars == afterChars` e a entry de gain sai com `saved_chars = 0` (`main.go:162-176`).
- `quality`: 7 sinais com pesos 25/15/15/15/10/10/10 (`quality.go:36-42`), notas `S>=90, A>=80, B>=70, C>=55,
  D>=40, senao F` (`quality.go:80-93`). `compaction_depth` cai 20 pontos por versao (`quality.go:134-148`):
  uma sessao com 5+ compactacoes nao recupera S mesmo que o conteudo esteja limpo.
- `activity`: classifica o modo dominante pelas ultimas 10 tool calls (`activity.go:24`), pesos por substring do
  nome da tool (`activity.go:26-40`).
- `skeleton`: esqueleto de `.py/.ts/.js/.go` cacheado por fingerprint (mtime+tamanho+hash), mas so a partir de
  8 KiB — abaixo disso devolve o arquivo inteiro com `reduction_pct = 0` (`skeleton.go:29-31,44-54`).
- `archive`/`expand`: arquiva acima de 4096 bytes guardando apenas 500 chars de preview (`archive.go:27-32`).

## 4. Camadas de estado e o que cada uma preserva

O binario mantem quatro camadas independentes. Nenhuma reconcilia as outras: nao existe fonte unica de
verdade entre elas.

- **Working memory (turns).** O input de cada tool e redigido por `redactSecrets` **antes** do corte em 16000
  chars, para nao deixar um segredo partido pela metade (`summarize.go:336-346`). A deteccao de loop reduz
  cada turn a 512 chars com whitespace colapsado (`store.go:140-150`): diferenca de newline nao esconde loop
  real, mas conteudo alem de 512 chars nao participa da comparacao.
- **Resumo heuristico.** 6 categorias por regex bilingue sobre os turns disponiveis (`heuristic.go:24-33`,
  `HeuristicExtract`). Nunca falha e nunca chama LLM; em troca, so preserva frase que casa com marcador.
- **Resumo por LLM (`summarize`).** Monta o prompt com o historico redigido (`formatHistoryForPrompt`,
  `summarize.go:365-372`) e interpola `{BUDGET_TOKENS}` sem truncar a resposta. No modo `auto`, usa heuristico
  enquanto `turns <= 20` **e** `EstimatedChars <= 24000`; acima disso vai para o agente
  (`summarize.go:194-218`; ~6k tokens pelo proprio comentario do codigo).
- **Arquivo.** Resultado acima de 4096 bytes vira `archive/<id>.txt` com 500 chars de preview no turn
  (`archive.go:27-32`), recuperavel por `expand --list|--search <q>|<id>`.
- **`memory-mcp`.** Espelho do resumo so com `AGENT_SYNC_CTX_REMEMBER=1` — default off porque a tabela e
  compartilhada entre maquinas e sincronizada (`summarize.go:249-257`).

Regra derivada: ao reancorar, diga de qual camada esta lendo. `ctx-window show` (turns + summary) e `expand`
(original arquivado) podem discordar, e o resumo por regex nao tem como recuperar o que a working memory ja
perdeu no corte de 16000 chars.

## 5. Roteamento real dos hooks (3 lacunas verificadas)

1. **Auto-compactacao por threshold nao existe mais (resolvido).** O hook Go e o plugin do OpenCode chamam
   `on-tool-call-llm` (`hook.go:132`, `hooks/ctx-compact.v2.ts:136`), que nunca compactou. O subsistema
   `on-tool-call` — unico que comparava com o threshold — e o knob `AGENT_SYNC_CTX_COMPACT_AT` foram removidos
   em 2026-11-16. Compactacao agora so acontece por `compact` ou `summarize`, explicitos.
2. **`ctx-window-summarize-at-stop.sh` nunca summarize.** A linha 86 chama
   `"$BIN" summarize -root "$root"`, mas `summarize` so aceita `--cli`, `--model`, `--timeout`
   (`summarize.go:108-110`). Rodado: `ctx-window summarize -root /tmp/x` imprime
   `flag provided but not defined: -root` e sai com `exit=1`. Como a chamada esta dentro de um `if`, o resumo
   do Stop falha em silencio.
3. **A compactacao e invisivel na metrica.** `quality` reporta `7 compactions (deep)` e `gain` da mesma sessao
   responde `(no compaction events recorded yet)`. Nao da para auditar o que a compactacao descartou.

## 6. Onde o comportamento diverge da recomendacao de livro

| Recomendacao (livro) | Comportamento atual | Divergencia |
| --- | --- | --- |
| Conte tokens pelo tokenizer do modelo; ~4 chars/token em ingles e ~2 em codigo (Prompt Engineering, p.42, p.148) | Tudo em chars; `--max-tokens` do `repo-map --summary` conta **palavras** (`len(strings.Fields(line))`, `tools/internal/repomap/repomap.go:1148-1156`) e o `--brief` usa `len(data)/4` (`repomap.go:789-791`) | O mesmo flag tem duas unidades diferentes no mesmo binario; nenhuma e token |
| Resumo deve ficar ~1/10 do original e o custo e dominado pelo texto de entrada (Prompt Engineering, p.128) | Heuristica local por regex extrai 6 categorias (`heuristic.go:24-33`), sem razao-alvo e sem medicao do tamanho relativo; `BUDGET` existe mas nao corta | Nao ha controle de compressao: o resumo pode ficar do tamanho do original |
| Nada critico no meio; instrucao e formato nas pontas (Prompt Engineering, p.135-136; AI Engineering, p.238-239) | Nenhum mecanismo posiciona conteudo: o hook de ctx emite `{}` e o summary e gravado em disco, fora do prompt | Sem controle de posicao — a recencia depende do CLI, nao do binario |
| Janela deslizante com overlap (`doc_stride`) para nao perder a resposta no fim (NLP with Transformers, p.199) | `s.Turns` cresce e nunca e truncado; `K` so afeta a exibicao (`store.go:90-96`, `store.go:301-315`) | Nao ha janela deslizante de armazenamento nem overlap: o "contexto" acumula e so o resumo representa o passado |
| Curadoria por turno: quais tools, quais artefatos, quao longe voltar (Prompt Engineering, p.196-199) | Decisao fixa: K turns, archive opt-in por tamanho | Nao existe decisao por turno; so dois defaults estaticos |
| Custo de atencao ~O(n^2); estruturar a entrada em vez de empilhar (NLP with Transformers, p.369) | O limiar de entrada e chars de turn registrado, sem modelo de janela do provedor | Nenhum limiar expressa a janela real do modelo; nao ha sinal para "quanto do budget do provedor ja gastei" |
| Threshold de compactacao deve ser calibrado por medicao, nao por palpite (AI Engineering, p.238: meca com NIAH no dado privado) | Default 200 chars, sem calibragem; 96,7% das sessions locais passam dele | Default nao calibrado produz `grade: F` como estado normal |

Onde o binario **esta alinhado** com o livro e vale registrar para nao "corrigir" o que funciona:

- Truncagem de `--brief` e explicita: corta calls/importers/symbols em blocos de 8/8/10 e marca o resto com
  `"... (N mais)"` (`repomap.go:789-813`) — respeita o alerta de que truncar sem aviso perde informacao
  (NLP with Transformers, p.51).

## 7. Regras acionaveis

- Trate `EstimatedChars` como proxy grosseiro: para decidir "cabe?", meca tokens no CLI antes de comparar com a
  janela do modelo.
- Nao existe auto-compactacao por threshold: o limiar e fixo (200 chars) e so `quality` o usa para pontuar.
  Compacte com `ctx-window compact <session>` ou `summarize` quando decidir.
- Nao conte com o auto-summarize do Stop como rede de seguranca: a chamada passa flag inexistente e falha.
- Nao leia `quality` como saude do contexto: `compaction_depth` e monotonicamente punitivo e `context_fill`
  saturado pelo default de 200 chars.
- Grave o essencial antes de compactar: o resumo heuristico nao preserva nada que nao case com os 6 regex de
  `heuristic.go`.
- Para `repo-map --max-tokens`, assuma palavras no `--summary` e chars/4 no `--brief`; se precisar de token,
  meca depois de gerar.

## 8. Verificacao rapida

```bash
ctx-window doctor                          # summarizer, K, budget, ollama
ctx-window quality <session>               # 7 sinais + grade
ctx-window gain <session>                  # o que a compactacao alegou ter economizado
ctx-window activity --json <session>       # modo dominante nas ultimas 10 tool calls
ctx-window summarize -root .               # deve falhar: summarize nao aceita -root
```

## Fontes

- Prompt Engineering for LLMs (Berryman & Ziegler) — Cap. 2 "Counting Tokens" p.42; Cap. 5 "Prompt Content"
  p.116-121, "Summarization" p.127-130; Cap. 6 "Assembling the Prompt" p.135-136 (posicao), p.148-151
  (inertness/contagem); Cap. 8 "Context for Task-Based Interactions" p.196-199.
- AI Engineering (Chip Huyen) — Cap. 5 "Context Length and Context Efficiency" p.238-239 (ordem do contexto,
  NIAH no dado privado).
- NLP with Transformers (Tunstall et al.) — Cap. 2 "From Text to Tokens" p.51; Cap. 7 "Question Answering"
  p.199 (sliding window com overlap); Cap. 11 "Sparse Attention" p.369 (custo quadratico).
