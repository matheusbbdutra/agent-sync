# Métricas e judge — exato, rubrica, viés e baseline

Como decidir **o que** medir antes de rodar o harness, e quando o número vem de código
(exato) e quando vem de um juiz. Páginas citadas do digest de origem; o comportamento do
`eval-run` corresponde ao código em `tools/internal/evalrun/score.go`.

## 1. Escolha a métrica pelo tipo de tarefa, não pelo que é fácil de calcular

- Conhecimento/classificação → tarefa close-ended com baseline aleatório declarado; geração
  aberta → verificação por judge/regra; código/SQL → correção funcional executável (pass@k,
  com `k` declarado). "MCQs test the ability to differentiate good responses from bad
  responses (classification), which is different from the ability to generate good responses"
  (p.184); "The final score, called pass@k, is the fraction of the solved problems out of all
  problems" (p.149).
- Escolha a métrica pela finalidade e pela assimetria de custo, e ancore o threshold no custo
  do erro: "The purpose of the model will determine which evaluation metrics are most
  important" (p.112); "metrics that are robust to noise, such as mean absolute error" (p.113);
  "RMSLE is less responsive to outliers than RMSE, and it tends to penalize underestimations
  slightly more than overestimations" (p.115).
- Threshold é parâmetro declarado, não default da ferramenta: "The best confidence threshold
  will depend on your business needs and goals, so there is no single correct answer here"
  (p.154).
- Classe rara destrói métrica agregada: "the PR curve tends to be a stronger metric in the
  case of unbalanced datasets" (p.147); "accuracy quickly becomes unreliable on its own"
  (p.145).

## 2. Exato (determinístico) — o que ele prova e o que não prova

- No harness, exato = `must_match` (substring case-insensitive), `must_not_match` e
  `json_schema` (parse JSON + `required` de nível 1). Não custa token, não varia entre runs e
  é o único sinal que dá para usar em gate de CI sem baseline estatístico.
- Exato é **necessário e insuficiente**. Compare com a lição de EM/F1: "EM = 1 if the
  characters in the predicted and ground truth answers match exactly" (p.216) — EM subestima
  ("about 6000 hours" vs "6000 hours" → EM 0, F1 0.8, p.216) e F1 superestima ("about 6000
  dollars" → EM 0, F1 0.4, p.217). "Relying on just the F1-score is thus misleading … tracking
  both metrics is a good strategy" (p.217).
- Traduzindo para o harness: `must_match` é frouxo (substring), `json_schema` é rígido
  (presença de campo), nenhum dos dois mede qualidade do conteúdo. Sempre que a suite só tiver
  exato, declare isso no relatório — o `pass_rate` mede forma, não acerto.
- Exato serve para travar o que não pode variar: formato de saída, presença de seção, nome de
  skill esperado, ausência de termo proibido. Não use judge para checar presença de campo.

## 3. Baseline trivial antes de qualquer métrica de texto gerado

- Inclua sempre uma heurística idiota e reporte lado a lado; se ela vence, o número do modelo
  não é evidência. Baseline do livro: "the first three sentences" (p.143) — e na tabela
  correspondente o baseline marca ROUGE-1 0.303571 contra 0.187500 do gpt2 (p.173).
- Prefira métrica com tokenização interna a implementação caseira: "it expects the text to
  already be tokenized … SacreBLEU … is the preferred metric for benchmarking" (p.170).
- Métrica automática de texto é proxy: "human judgment remains the best measure" (p.183). No
  harness, isso significa: judge/regex nunca substitui inspeção da amostra que falhou.

## 4. Rubrica — escreva antes de medir, e valide com humano

- Criar a guideline é o passo mais importante do pipeline: "Creating a clear evaluation
  guideline is the most important step of the evaluation pipeline" (p.223).
- Valide a rubrica com humanos antes de confiar nela como métrica: "Validate your rubric with
  humans… If humans find it hard to follow the rubric, you need to refine it" (p.224). Rubrica
  que humano não aplica de forma consistente não gera score interpretável.
- Declare o que o sistema **não** deve fazer, não só o que deve. Rubrica sem escopo negativo
  aprova "correto mas inútil" (p.223).
- No harness: `judge.rubric` é texto livre curto e acionável — critérios, escala e o que
  desqualifica. Se a rubrica precisa de mais de um parágrafo, ela ainda não está pronta.

## 5. Anatomia do prompt de judge

- Três partes obrigatórias: (1) a tarefa, (2) os critérios detalhados, (3) o sistema de
  pontuação com exemplos por nota — "1. The task… 2. The criteria… 3. The scoring system"
  (p.162).
- Prefira classificação a número contínuo: "AI judges work better with classification than
  with numerical scoring systems" (p.162). Se precisar de número, use escala discreta curta
  (1–5) com significado por nota, como no livro.
- Juiz opaco é juiz inválido: "Do not trust any AI judge if you can't see the model and the
  prompt used for the judge" (p.165). No harness isso é estrutural — o prompt do juiz é o
  `judge-cmd` que você versiona, e o `--model` fica gravado no registro.
- Normalização e threshold do harness: `normalized = score / max_score`, aprovado quando
  `normalized >= threshold`, com `threshold` em `(0,1]`. Isso impede comparar escalas
  diferentes como se fossem a mesma métrica: a Tabela 3-4 do cap. 3 lista 1-5 para MLflow,
  0/1 para Ragas e YES/NO para LlamaIndex no mesmo rótulo "faithfulness", e o sistema de
  pontuação declarado no prompt de judge é o que define a escala (p.162).

## 6. Viés do judge — assuma antes de interpretar delta

- Custo multiplica: gerar + avaliar por critério faz várias chamadas por item — "you'll do
  twice as many GPT-4 calls… increase your number of API calls four times" (p.166). Defina a
  taxa de amostragem do judge e registre-a.
- Position bias: "Many AI models have first-position bias" (p.167) — ao comparar duas
  respostas, reordene e repita; nunca compare em ordem fixa.
- Verbosity bias: juiz prefere resposta longa "regardless of their quality" (p.167). Um ganho
  de `pass_rate` acompanhado de aumento de comprimento é suspeito até prova contrária.
- Self-bias e spot-checking: revisar por amostra "might fail to catch some failures" (p.166).
  Conclusão operacional: judge serve para sinal agregado e monitoramento, não para aprovar
  caso individual sem inspeção.

## 7. Baseline obrigatório

- Sem baseline, `pass_rate` é número solto. Comparação só vale com suite e alvo congelados
  (hashes iguais); qualquer mudança em um dos dois torna o delta incomparável — e o binário
  sai com `3` justamente para impedir interpretação indevida.
- Escolha critério e alvo de sucesso **antes** de ver o número: "satisfactory performance is
  something that should be dictated by the business needs and should be agreed upon before
  starting any ML engagement" (p.27).
- Não escolha o vencedor pelo conjunto em que reporta: "the final model you choose may be
  biased toward the validation dataset since you specifically choose the model that performed
  the best on the validation dataset" (p.264).
- Ao adicionar judge a uma suite que antes era só exata, o baseline muda de régua: rode o
  baseline de novo na mesma configuração antes de comparar. Caso `judge` sem `--judge-cmd`
  fica `skipped` (nunca `pass`), e `--judge-cmd` que quebra aborta a run com `2`.

## 8. Onde o judge não entra

- Formato e schema: use `json_schema` (`required` de nível 1) — determinístico e barato.
- Presença/ausência de termo: `must_match`/`must_not_match`.
- Contagem de itens, tamanho, ordem: regra explícita ou revisão humana; judge de escala 1–5
  não mede grandeza.
- Segurança e política: se um caso proibido passar, o custo de FP e FN não é simétrico —
  trate como gate rígido (`must_not_match`), com o custo de erro declarado (p.147, p.154).

## 9. Exemplo trabalhado (suite real de routing)

Caso exato — o pedido é o estímulo, o nome da skill esperada é a regra:

```json
{"id":"roteamento-pt-01","slice":"pt-br","weight":1,"input":{"kind":"prompt","text":"audite o repo e liste os débitos técnicos"},"expect":{"must_match":["repo-auditor"]}}
```

Rodando contra `skills/repo-auditor/SKILL.md` o caso passa (o arquivo contém o próprio nome);
rodando contra outro SKILL.md, falha. Ou seja: `must_match` de nome comprova **que artefato
foi selecionado**, não que o routing decidiu bem — para isso o output precisa ser a decisão do
roteador, não o documento.

Caso negativo — a mesma entrada com termo proibido separa vizinhos confusos:

```json
{"id":"roteamento-neg-09","slice":"negativo","weight":2,"input":{"kind":"prompt","text":"audite as vulnerabilidades de segurança deste serviço"},"expect":{"must_match":["security-auditor"],"must_not_match":["repo-auditor"]}}
```

Caso de judge — quando a resposta é texto livre e nenhuma substring prova acerto:

```json
{"id":"roteamento-aberto-10","slice":"aberto","input":{"kind":"prompt","text":"resuma o que este agente fez nesta sessão"},"judge":{"rubric":"Cita um arquivo alterado e uma pendência concreta?","max_score":5,"threshold":0.6}}
```

Sem `--judge-cmd`, esse caso sai `skipped` (nunca `pass`). Com um juiz que devolve
`{"score":5}`, `5/5 = 1.0 >= 0.6` → `pass`; com `{"score":2.5}`, `0.5 < 0.6` → `fail`.

## 10. Checklist antes de confiar em judge

- [ ] A rubrica foi escrita antes da primeira run e não foi ajustada para o resultado.
- [ ] A escala é discreta e cada nota tem significado (p.162).
- [ ] O prompt/modelo do juiz é visível e versionado (p.165).
- [ ] A amostragem e o custo do judge estão declarados (p.166).
- [ ] Ordem e comprimento foram controlados na comparação (p.167).
- [ ] Existe baseline com suite e alvo idênticos; delta só é lido quando `comparable`.
- [ ] Há um baseline trivial ou exato ao lado para provar que o judge agrega sinal (p.143,
      p.173).

## Fontes

- Tunstall, von Werra, Wolf, *Natural Language Processing with Transformers* (O'Reilly, 2022)
  — cap. de sumarização/métricas: p.143, p.170, p.173, p.183; cap. de QA: p.216–217. Digest
  `analysis/nlp-transformers.md` (U12, U18).
- Stripling & Abel, *Low-Code AI* — cap. 1: p.27; cap. 4: p.112–115; cap. 5: p.127, p.145,
  p.147, p.154; cap. 8: p.264. Digest `analysis/low-code-ai.md` (U1–U8).
- Huyen, *AI Engineering* — cap. 3 (Exact Evaluation p.149, AI as a Judge p.162, Limitations
  p.165, Biases p.166–167), cap. 4 (Generation Capability p.184, Evaluation Criteria p.223–224,
  Design Your Evaluation Pipeline p.221). Digest `analysis/ai-engineering.md` (U3, U5, U8, U9).
- Comportamento do binário: `tools/internal/evalrun/score.go` (`ScoreCase`, `ExecJudge`,
  normalização `score/max_score`) e `tools/cmd/eval-run/main.go` (exit codes).
