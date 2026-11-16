---
name: llm-eval
description: "Use when medindo qualidade com número: rodar eval-run, comparar pass_rate contra baseline e detectar regressao de uma skill ou prompt. Mede artefato existente; nao escreve prompt nem inventa rubrica."
---

# LLM Eval — medir o que já existe

Skill de **medição**. Escopo: validar suite, rodar, ler relatório e decidir se o número é
confiável. Escrever ou reescrever prompt, skill ou rubrica é outra tarefa — aqui você mede o
artefato no estado em que ele está.

## Use this skill when

- Alguém diz "melhorou"/"piorou" e você precisa de número com baseline.
- Antes e depois de editar `SKILL.md`, `AGENTS.md`, prompt ou template.
- Precisa de gate de regressão em CI (passou/não passou com critério declarado).
- Precisa decidir se um caso cabe em regra exata (`must_match`) ou exige judge.

## Do not use this skill when

- Ainda não existe suite: primeiro defina os casos (formato em
  `reference/eval-harness.md`).
- A pergunta é "como escrever melhor este prompt" — isso não é medição.
- O artefato é código e o critério é determinístico: use teste normal (`go test`, e2e).

## Fluxo

```bash
# 0. binário (make build gera bin/eval-run)
make build

# Suite de exemplo real, para ver o formato em uso:
#   tools/cmd/eval-run/testdata/smoke.jsonl

# 1. validar a suite antes de rodar
bin/eval-run suite check <sua-suite>.jsonl

# 2. baseline (antes da mudança)
bin/eval-run run --suite <sua-suite>.jsonl --target skills/repo-auditor/SKILL.md \
  --model baseline-manual

# 3. editar o artefato ...

# 4. mesma suite, mesmo alvo, contra o baseline
bin/eval-run run --suite <sua-suite>.jsonl --target skills/repo-auditor/SKILL.md \
  --baseline <run_id do passo 2> --tolerance 0.02

# 5. ler o que ficou gravado
bin/eval-run report                 # último run
bin/eval-run list --limit 10        # histórico
```

## Como ler o relatório

| Sinal | Significado |
| --- | --- |
| exit `0` | nenhuma falha e, se havia baseline, comparável |
| exit `1` | ao menos um caso em `fail` |
| exit `2` | suite inválida, alvo ausente, juiz quebrado, uso errado |
| exit `3` | `suite_hash` ou `target_hash` diverge do baseline: **não compare** |

- `scored` + `skipped` = `cases`. `skipped` é caso sem regra aplicável (ex.: bloco `judge`
  sem `--judge-cmd`); **nunca** leia como acerto.
- `pass_rate` de um slice é ponderado por `weight` e usa só casos `scored` no denominador.
- `delta_pass_rate` e `regressions` só existem quando `comparable=true`. Regressão = queda
  acima de `--tolerance` (default `0.02`), medida por slice.
- Leia `by_slice` antes do total: total estável com slice caindo é regressão.

## O que é número confiável

- **Hashes iguais.** Sem `suite_hash` e `target_hash` idênticos ao baseline não há delta; só
  há exit `3`.
- **Delta maior que a amostra.** Mudança pequena precisa de mais casos: cada 3× de redução na
  diferença pede ~10× de amostras.
- **Nenhum slice em queda** com o total igual.
- **Ganho não vem de formato.** Se o `pass_rate` subiu junto com o tamanho do artefato, com
  nome de arquivo ou com cabeçalho de seção, trate como suspeito até inspecionar uma amostra
  que passou.
- **Judge auditável.** Score de juiz só vale com rubric visível, `max_score` declarado e
  `--model` gravado. Juiz sem prompt visível não é métrica.

## Erros comuns (todos invalidam a medição)

- Editar a suite e comparar com baseline antigo (vira exit `3`, mas o número já mudou de régua).
- Rodar uma vez e chamar de resultado: uma execução não distingue sinal de ruído.
- Reportar só o total e ignorar o slice que caiu.
- Contar `skipped` como sucesso para "fechar verde".
- Confiar em judge cuja rubric foi ajustada depois de ver o resultado.

## Reference

- `reference/eval-harness.md` — schema real do JSONL, split congelado, slice, baseline, gate
  de regressão, campos do registro e flags do binário.
- `reference/metrics-and-judge.md` — exato vs judge, rubrica, viés do judge, baseline
  obrigatório.
