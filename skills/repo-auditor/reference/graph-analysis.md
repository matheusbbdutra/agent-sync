# Leitura de grafo de dependencias

Criterio de leitura para grafo de imports/chamadas de um repositorio: o que se pode afirmar, o que
precisa ser certificado e o que nao pode ser prometido. Este arquivo **nao** define como calcular:
calculo e do `repo-map`.

## 1. Divisao de trabalho: calculo no `repo-map`, criterio aqui

- O binario expoe hoje, conforme `repo-map --help`: `--update` (cache incremental), `--focus
  <caminho>` (subgrafo), `--brief <caminho>` (JSON delimitado), `--summary` (hubs de chamadas/imports)
  e `--audit-removal <alvo>`; com `-depth`, `-max-tokens` e `-root` como ajustes.
- Se o numero que voce precisa (componentes, corte, ciclo) nao sai desses modos, calcule fora e
  valide o resultado com os testes da secao 2 — nao apresente o numero sem passa-los.
- O que este reference governa: qual afirmacao o relatorio pode fazer com o grafo que o `repo-map`
  produziu (conectividade, componente, acoplamento, ciclo, particao, otimalidade).

## 2. Valide o grafo antes de concluir qualquer coisa

- **Normalize para grafo simples.** Colapse arestas duplicadas entre o mesmo par de vertices e
  descarte lacos (`v->v`, ex.: funcao recursiva) antes de contar grau, componente ou ciclo: o modelo
  do livro e `A ⊆ V(2)`, sem aresta repetida e sem ponta coincidente (p.8).
- **Nao conte linha de import como aresta.** Duas referencias entre os mesmos dois modulos sao uma
  aresta; contar cada linha infla grau, infla contagem de ciclos e cria "ciclo" de modulo consigo
  mesmo (p.8).
- **Rode o teste de integridade dos graus:** `Σ g(v) = 2·|A|` (p.14) e o numero de vertices de grau
  impar e par (p.15). Falha em qualquer um = export truncado, aresta duplicada ou ponta inexistente.
- **Aborte em vez de reportar.** Conclusao tirada de grafo corrompido ("sem ciclos", "sem
  dependencias") e pior que ausencia de conclusao (p.14-15).
- **Comece pelo corte vazio e total:** `∇(∅) = ∇(V(G)) = ∅` (p.14). Se o seu calculo de fronteira
  devolve aresta para esses dois casos, o export esta errado.

## 3. Componentes e conectividade

- **Certifique conectividade por alcance exaustivo**, nunca por amostragem: travessia a partir de um
  vertice semente e contagem igual a `n`. O criterio equivalente e `∇(X) ≠ ∅` para todo subconjunto
  proprio e nao-vazio `X` (p.18).
- **Nao use 2-3 caminhos de exemplo como prova.** Entrypoint -> modulo A -> B nao cobre ilhas
  (scripts, testes, codigo morto) que ficam fora do alcance (p.18).
- **Decomponha em componentes antes de ordenar, agendar ou medir impacto.** Componente e o subgrafo
  conexo maximal; cada vertice pertence a exatamente um; o grafo e conexo se e somente se tem um
  unico componente (p.18).
- **Ordene e paralelize por componente.** Nao existe ordem que atravesse componentes desconexos:
  emitir uma prioridade global unica mistura ilhas independentes que poderiam correr em paralelo
  (p.18).
- **Trate "3 componentes" como achado de arquitetura**, nao como nota de rodape (p.18).

## 4. Acoplamento pelo corte, nao pela contagem total

- **Calcule `|∇(X)|`** = arestas com exatamente uma ponta em `X` e outra fora; arestas internas a `X`
  nao entram (p.14).
- **Reporte esse numero como acoplamento da fronteira.** Dois cortes de particoes diferentes ficam
  comparaveis pela cardinalidade (p.14).
- **Nao eleja "modulo mais acoplado" por fan-in/fan-out absoluto:** modulo grande sempre ganha, e
  import interno contado como externo faz qualquer particao parecer igualmente ruim (p.14).

## 5. Ciclos e arestas redundantes

- **Teste de remocao:** uma aresta `a` participa de ciclo se, e somente se, `G − a` continua conexo no
  componente em que ela vive (p.18).
- **Equivalencia util para "sem ciclo":** o grafo e floresta se, e somente se, cada aresta e um corte
  (p.17); conexo e floresta se, e somente se, `G − a` deixa de ser conexo para toda aresta (p.18).
- **Orcamento barato de ciclo:** `m ≥ n − c`, com `c` = numero de componentes (p.18). `m − n + c`
  crescente ao adicionar aresta que nao muda conectividade e sinal de ciclo.
- **Nao venda `m − n + c` como numero de ciclos simples:** e limitante/dimensao, nao enumeracao
  (p.18; o proprio digest marca essa leitura como confianca media).
- **Antes de manter uma dependencia, pergunte se ela e um corte.** Aresta que sai sem desconectar e
  excedente para a conectividade: trate como candidata a remocao, nao como dependencia necessaria
  (p.17-18).

## 6. Particao em 2 camadas: exija certificado

- **Para afirmar que da:** exiba a bicoloracao completa. Bicolorivel se e somente se nao tem circuito
  impar (p.33).
- **Para afirmar que nao da:** exiba um unico circuito impar concreto. "O unico motivo para a
  inexistencia de uma bicoloracao e a presenca de um circuito impar" (p.34), e o circuito impar e
  "um certificado simples e convincente" (p.34).
- **Bicoloracao parcial nao serve** como prova de layering; ela nao cobre todos os vertices (p.33-34).
- **Em componente conexo a bicoloracao e unica** (p.35): se voce encontrou duas distintas, ha mais de
  um componente — volte para a secao 3.

## 7. O que o relatorio pode prometer

| Pergunta do relatorio | Pode afirmar | Base |
| --- | --- | --- |
| "esse modulo e o mais acoplado" | so com `|∇(X)|` da fronteira proposta | p.14 |
| "o repo e um grafo conectado" | so com alcance exaustivo = `n` | p.18 |
| "esse import e redundante" | so com teste de remocao | p.18 |
| "da para separar em 2 camadas" | so com bicoloracao completa | p.33-35 |
| "da para separar em 2 camadas" (negativo) | so com circuito impar exibido | p.34 |
| "esse e o conjunto minimo de arquivos para revisar" | nao, salvo prova: conjunto estavel maximo e `2^n` | p.28 |

- **Rotule heuristica como maximal.** Todo conjunto estavel maximal tem pelo menos `n/(Δ+1)` vertices
  (p.21); isso e garantia proporcional, nao otimalidade. No caso de emparelhamento, achar um maximal
  e facil e procurar o maximo e "bem mais dificil" (p.37).
- **Classifique a tratabilidade antes de prometer otimo:** conjunto estavel maximo e coloracao minima
  custam `2^n` (p.28, p.36); isomorfismo exato custa `n!` (p.13); decidir se o indice cromatico vale
  `Δ` ou `Δ+1` nao tem algoritmo rapido conhecido (p.53).
- **O que e polinomial no livro:** emparelhamento maximo (p.47) e, em grafo bipartido, emparelhamento
  maximo igual a cobertura minima (König, p.41).
- **Sem certificado barato, diga isso:** fora do caso bicolorivel nao se conhece certificado
  substancialmente melhor (p.36).

## 8. Limite do modelo: grafo nao-dirigido

- O livro trata exclusivamente de grafos simples **nao-dirigidos** e finitos (p.8). Ele responde "quem
  esta ligado a quem", nao "quem depende de quem".
- **Ordenacao topologica, DAG e caminho critico nao estao nesta fonte** (verificado no digest: zero
  ocorrencias de `topológ`, `acícl`, `DAG`, `direcionad`, `digrafo`, `orientad`). Nao use p.N deste
  livro para sustentar ordem de build ou caminho critico.
- Consequencia pratica: direcao de dependencia, fan-in/fan-out direcionado e sequenciamento ficam
  fora do que este reference pode afirmar.

## 9. Checklist antes de escrever o achado

- [ ] Grafo normalizado (sem aresta duplicada, sem laco) (p.8).
- [ ] `Σ g(v) = 2·|A|` e paridade dos graus conferidas (p.14-15).
- [ ] Componentes calculados e reportados como unidade (p.18).
- [ ] Conectividade certificada por alcance, nao por exemplo (p.18).
- [ ] Acoplamento medido por `|∇(X)|` (p.14).
- [ ] Ciclo comprovado por remocao, nao por suspeita (p.18).
- [ ] Layering com certificado (bicoloracao ou circuito impar) (p.33-34).
- [ ] Heuristica rotulada como maximal, com limitante (p.21, p.37).
- [ ] Nenhuma afirmacao de ordem/caminho critico atribuida a esta fonte (p.8).

## Fontes

- Uma Introducao Sucinta a Teoria dos Grafos (P. Feofiloff, Y. Kohayakawa, Y. Wakabayashi) — cap. 1
  (p.8: grafo simples nao-dirigido, sem aresta repetida e sem laco; p.13: isomorfismo `n!`; p.14:
  corte `∇(X)`, identidade `Σ g(v) = 2·|A|`; p.15: paridade dos graus; p.17: floresta e aresta de
  corte; p.18: conectividade, componentes, teste de remocao, `m ≥ n − c`); cap. 2 (p.21: `α(G) ≥
  n/(Δ+1)`; p.28: conjunto estavel maximo em `2^n`); cap. 3 (p.33-35: bicoloracao, circuito impar como
  certificado, unicidade em componente conexo; p.36: ausencia de certificado barato); cap. 4 (p.37:
  maximal vs maximo; p.41: König; p.47: emparelhamento maximo polinomial); cap. 5 (p.53: decisao `Δ`
  vs `Δ+1` sem algoritmo rapido).
- Ferramenta: `repo-map --help` (modos e flags citados na secao 1), executado no ambiente local; o
  binario nao documenta calculo de componentes, corte ou ciclo, por isso a secao 1 separa calculo de
  criterio.
