# Eval harness — suite, split congelado, slice e gate de regressão

Contrato operacional do `bin/eval-run` (Go, `tools/cmd/eval-run`). Toda regra abaixo tem
página verificável no digest de origem; o schema e o comportamento do binário foram copiados
do código e da saída real do comando, não de memória.

## 1. O que o harness mede

- Mede **um artefato** (arquivo `--target`: SKILL.md, prompt, AGENTS.md) contra uma **suite
  congelada** de casos. O conteúdo do target é o output aplicado a cada caso.
- Sinal determinístico primeiro: `must_match`, `must_not_match`, `json_schema` não dependem
  de modelo, não custam token e são reprodutíveis byte a byte.
- Sinal caro depois: `judge` só roda quando `--judge-cmd` é passado, e só para casos que
  declaram bloco `judge`. Componha por custo: sinal barato em 100% dos casos, caro na
  amostra que você sabe interpretar (p.225 — "a cheap classifier that gives low-quality
  signals on 100% of your data, and an expensive AI judge… on 1% of the data"; p.139).
- Avalie componente e ponta: uma suite por artefato (routing, formato de saída, cobertura de
  seção) além da suite end-to-end. "You should evaluate the end-to-end output and each
  component's intermediate output independently" (p.221).

## 2. Schema real do caso (JSONL)

Uma linha = um caso, linhas vazias ignoradas. Campos exatamente como em
`tools/internal/evalsuite/suite.go`:

| Campo | Tipo | Regra de validação |
| --- | --- | --- |
| `id` | string | obrigatório, não vazio, único na suite |
| `slice` | string | obrigatório, não vazio |
| `weight` | number | `> 0`; ausente/0 vira `1` |
| `input.kind` | string | rótulo do estímulo (ex.: `prompt`) |
| `input.text` | string | estímulo legível por humano |
| `expect.must_match` | []string | substring, case-insensitive, todas precisam aparecer |
| `expect.must_not_match` | []string | substring, case-insensitive, nenhuma pode aparecer |
| `expect.json_schema` | object | JSON Schema compilável; scoring checa `required` de nível 1 |
| `judge.rubric` | string | obrigatório se `judge` existe |
| `judge.max_score` | number | `> 0` |
| `judge.threshold` | number | fração em `(0,1]` de `max_score` |

Exemplo mínimo válido (uma linha, sem quebras):

```json
{"id":"roteamento-pt-01","slice":"pt-br","weight":1,"input":{"kind":"prompt","text":"audite o repo"},"expect":{"must_match":["repo-auditor"]}}
```

Validação (`eval-run suite check`): id vazio/duplicado, slice vazio, `weight <= 0`, caso sem
nenhuma regra em `expect` ou `judge`, `json_schema` que não compila, `judge` sem `rubric`,
`max_score <= 0` ou `threshold` fora de `(0,1]` — cada problema vem com linha e campo.

## 3. Split congelado

- Gere a suite **uma vez**, versione no git e nunca re-split: "Splitting the dataset at the
  beginning of each training run is not a good idea… we should split once, and then continue
  using the same training and validation datasets for all our hyperparameter tuning
  experiments" (p.213).
- Consequência prática: o `suite_hash` (sha256 do arquivo) é a identidade da suite. Editar a
  suite invalida a comparação histórica — e o binário recusa comparar (§6).
- Separe o que **escolhe** do que **confirma**: use validação para decidir entre candidatos e
  teste para reportar o número final; escolher pelo mesmo conjunto em que se reporta produz
  viés otimista (p.264). No harness: suítes `.jsonl` distintas ou `--slice` distintos para os
  dois papéis.
- Reserve casos de erro conhecido e casos fora de escopo: "If you care about something, put a
  test set on it" (p.227).

## 4. Slice

- Todo caso pertence a um slice nomeado (idioma, tipo de tarefa, tamanho do input, origem).
  Slice é o que impede a média de esconder a cauda: "the model may perform much worse on some
  slices of the data than others, and these deficiencies can be lost in the ocean of
  calculating on the entire dataset" (p.314).
- Leia `by_slice` antes do total. Um total estável com slice em queda é regressão, não ruído
  (é o mesmo fenômeno descrito em p.314–315).
- `--slice <nome>` roda só um slice; caso o nome não exista na suite, o binário sai `2` em vez
  de reportar zero casos.
- Slice novo aparece no delta como `new_slice: true` e **não** entra no cálculo de delta global
  nem na contagem de regressões — comparar universos diferentes daria número sem significado.

- Slice novo aparece no delta como `new_slice: true` e **não** entra no cálculo de delta global
  nem na contagem de regressões — comparar universos diferentes daria número sem significado.

## 4.1 Failure mode vira slice

- Antes de escolher slices, liste os modos de falha que você quer pegar; a métrica deriva
deles, não do que a ferramenta reporta por default (p.487, "design your metrics around these
failures").
- Para artefatos com ação (routing, escolha de skill, chamada de tool), enumere os modos e
meça frequência por modo: tool inválida; tool válida com parâmetro inválido; tool válida com
valor errado; objetivo não atingido; reflexão que declara concluído sem estar (p.318–319).
Cada modo é um slice ou um caso `must_not_match`.
- Falha rara exige tratamento próprio: com classe positiva rara, métrica agregada não serve e
a decisão precisa do custo de FP vs FN explicitado (p.127, "equally accurate and useless";
p.145, "accuracy quickly becomes unreliable on its own").

## 5. Baseline

- Sem baseline não existe "melhorou": todo número é absoluto e não comparável. O registro
  append-only (`.agent-sync/eval-runs.jsonl`) é o baseline de fato.
- `--baseline` aceita caminho de arquivo de registros (usa a **última** linha) ou `run_id`
  (busca no registro corrente). `baseline`, `delta` e `comparable` ficam gravados no registro,
  então o veredito é auditável depois.
- Escolha a métrica e o critério **antes** de olhar o número: "satisfactory performance is
  something that should be dictated by the business needs and should be agreed upon before
  starting any ML engagement" (p.27).

## 5.1 Versionamento do alvo e da suite

- Trate suite como prompt: fora do código, versionada, com metadado de mudança — "It's good
practice to separate prompts from code" (p.253; catálogo versionado em p.255). O `suite_hash`
é a versão efetiva; se o arquivo muda sem que a versão mude, o histórico de `pass_rate` perde
sentido.
- Dois artefatos não compartilham baseline: alvo diferente = `target_hash` diferente = exit 3.
Compare como séries separadas, uma por artefato.

## 6. Gate de regressão

- `comparable=false` e exit `3` quando `suite_hash` **ou** `target_hash` diverge do baseline.
  Motivo: com suite ou alvo diferente, a diferença de `pass_rate` não é atribuível à mudança
  que você quer medir.
- Regressão = queda de `pass_rate` **acima** de `--tolerance` (default `0.02`), por slice.
  Comparação estrita com epsilon `1e-9` para que uma queda exatamente igual à tolerância não
  vire regressão por erro de ponto flutuante.
- `pass_rate` é ponderado por `weight` e calculado só sobre casos `scored`: `pass_rate =
  Σ weight_pass / Σ weight_scored`. Caso `skipped` fica **fora do denominador** — nunca conta
  como acerto.
- Precedência dos exit codes: `3` (incomparável) vence `1` (falhas), porque um delta medido
  contra outro alvo não deve ser interpretado nem quando há falhas.
- Detecte diferenças pequenas com amostra, não com tolerância generosa: "for every 3× decrease
  in score difference, the number of samples needed increases 10×" (p.228); reamostragem
  bootstrap instável (90% em uma, 70% em outra) significa suite pequena demais (p.227).

## 7. Como rodar (flags reais)

```
eval-run suite check <suite.jsonl> [--json]
eval-run run --suite <s> --target <t> [--baseline v] [--tolerance 0.02] \
             [--judge-cmd '<cmd>'] [--slice <nome>] [--model <nome>] \
             [--record <arquivo>] [--no-record] [--json]
eval-run report [--run <run_id>] [--record <arquivo>] [--json]
eval-run list [--record <arquivo>] [--limit N] [--json]
```

- Registro default: `.agent-sync/eval-runs.jsonl` (append-only, casado por `.gitignore`).
  `--no-record` roda sem gravar (útil em experimento descartável).
- `--judge-cmd` é processo externo: recebe `{"case":...,"output":...}` em stdin e devolve
  `{"score":N,"reason":"..."}` em stdout. Sem SDK de provider, sem HTTP. Juiz que falha ou
  devolve stdout inválido aborta a run com exit `2` — erro de juiz nunca vira veredito.
- Exit codes: `0` ok, `1` falhas, `2` uso/config, `3` incomparável (hash).

## 8. Campos do registro

`schema_version`, `run_id` (`eval-YYYYMMDD-HHMMSS-<4 hex>`), `ts` (RFC3339 UTC), `suite`,
`suite_hash`, `target`, `target_hash` (todos hash `sha256:<hex>`), `model`,
`totals{cases,scored,skipped,pass,fail}`, `by_slice[{slice,cases,scored,skipped,pass,fail,weight_pass,weight_scored,pass_rate}]`,
`baseline{run_id,ts,pass_rate}`, `delta{pass_rate,regressions}`, `comparable`.

O registro não guarda resultado por caso: se precisar do detalhe, use a saída da run
(`--json` traz `results` com regra, padrão e motivo de cada veredito).

## 9. Crescimento da suite em produção

- Amostre e salve uma fração das execuções reais, rotule pelo mesmo critério da suite e
acrescente como caso: "Randomly sample and save the data being sent for model predictions…
we might choose to save 1%" (p.316).
- Dispare revisão por gatilho explícito (tempo ou queda de performance), não por reclamação:
retreino/alerta por limiar e por janela comparada (p.316; p.310 para gatilho por percentual
de perda reportada).
- Distinga drift de input de drift de relação feature→rótulo ao decidir o que fazer com o
resultado: "data drift, a change in the underlying data distribution" vs "concept drift is
where the relationship between the features and the labels change over time" (p.310). Na
prática: slice novo ou distribuição de casos diferente = input drift; mesmo slice caindo =
relação mudou.

## 10. Checklist antes de confiar no número

- [ ] `suite_hash` e `target_hash` iguais aos do baseline (senão exit 3 e nada a interpretar).
- [ ] `skipped` foi olhado: se muitos casos não têm regra aplicável, a suite mede pouco.
- [ ] O total bate com `by_slice`; nenhum slice caiu com o total estável.
- [ ] O número não é de uma única execução quando o delta é pequeno (p.227, bootstrap;
p.228, 3× de diferença exige 10× de amostra).
- [ ] Se há `judge`, o prompt do juiz é visível e auditável (p.165).
- [ ] O ganho não vem de pista de formato: nome de arquivo, cabeçalho, ordem de seção
(p.211).

## 11. Atalhos que invalidam a medição

- Mudar a suite e comparar com baseline antigo: vira exit `3`, mas o hábito esconde que o
  número mudou de régua.
- Contar `skipped` como sucesso: caso sem regra aplicável nunca é evidência de acerto.
- Aceitar ganho sem olhar slice e sem olhar o que o artefato usa de fato — "we need to be
  mindful of what other things are within our images (and examine model explanations)"
  (p.211): nome de arquivo, cabeçalho, ordem de seção e contagem de itens são pistas fáceis
  de aprender no lugar do comportamento real.
- Reportar só a média global quando o caso de falha é raro: métrica dominada pela classe
  majoritária "acerta" e é inútil (p.302, "correct 998 times and wrong only twice… accuracy is
  not a useful evaluation metric").

## Fontes

- Huyen, *AI Engineering* (O'Reilly, 2025) — cap. 3 (Evaluation Methodology): p.139, p.147–149;
  cap. 4 (Evaluate AI Systems): p.221, p.225, p.227, p.228; cap. 10 (AI Engineering
  Architecture and User Feedback): p.487. Marcador `[[page N]]` do digest
  `analysis/ai-engineering.md`.
- Lakshmanan, Görner, Gillard, *Practical Machine Learning for Computer Vision* (O'Reilly,
  2021) — cap. 5 (Creating Vision Datasets): p.211; cap. 8 (Model Quality and Continuous
  Evaluation): p.264, p.302, p.314–315, p.316. Digest `analysis/practical-ml-cv.md` (U1–U6).
- Stripling & Abel, *Low-Code AI* — cap. 1: p.27; cap. 9 (Next Steps): p.310. Digest
  `analysis/low-code-ai.md` (U1, U8).
- Schema e comportamento do binário: `tools/internal/evalsuite/suite.go`,
  `tools/internal/evalrun/{score,compare,record}.go`, `tools/cmd/eval-run/main.go` e saída real
  de `eval-run --help`.
