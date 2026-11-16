# Escrita tecnica

Como redigir o artefato de pesquisa — resumo, reference, handoff, PR description. Trata de
estrutura, sintaxe e vocabulario do texto.

Fora de escopo: como buscar, escolher fonte oficial, versionar a consulta e separar verificado de
inferencia (isso esta no `SKILL.md` desta skill) e operadores de busca
([search-specialist](search-specialist.md)).

## 1. Piramide: conclusao primeiro, argumento depois, dado por ultimo

- **Primeira frase = ideia principal + o que voce precisa do leitor** (p.119, p.296).
- **Depois os argumentos que sustentam, e so entao os dados** (p.119-121).
- O exemplo do livro coloca o pedido antes do contexto: "We need to reschedule the project kick-off
  meeting. Can you let me know ASAP" (p.296).
- **Anti-padrao que isso previne:** texto em ordem cronologica de descoberta, com o pedido no ultimo
  paragrafo (p.119-121).
- **Aplicacao ao artefato de pesquisa:** abra com o achado e a decisao que ele pede; metodo de
  busca, tentativas e descartes vao depois.
- Vale para email, PR description, reference e slide deck — o livro aplica a mesma estrutura aos
  quatro (p.119).

## 2. Sintaxe: verbo de acao, uma razao por frase, 3 a 5 frases por paragrafo

- **Troque verbo fraco por verbo de acao:** `be`, `happen`, `there is`/`there are` nao dizem o que
  acontece (p.123).
- **Uma razao por frase:** "Your sentences should have only one reason to exist" (p.124).
- **Uma razao por paragrafo**, com 3 a 5 frases: "The sweet spot is at around three to five
  sentences" (p.124).
- **Teste pratico:** se a frase precisa de dois "e"/"mas" para ligar ideias independentes, sao duas
  frases.
- **Por que importa em instrucao de agente:** instrucao ambigua e interpretada de formas diferentes
  pelo executor (p.123-124).
- **Aplicacao:** instrucoes de skill, prompt de subagente, mensagem de erro e doc de repo seguem a
  mesma sintaxe (p.123).

## 3. Um termo por conceito, acronimo definido na primeira ocorrencia

- **Escolha um termo e use sempre ele:** "Pick one and stick to it when you mean the same thing"
  (p.125).
- **Se usar dois termos parecidos, declare a diferenca** (p.125). Sem isso, o leitor — ou o agente —
  trata como duas coisas.
- **Defina todo acronimo na primeira ocorrencia** ou em glossario/legenda; nao presuma que a
  audiencia conhece (p.116, p.118, p.125).
- **Nao alterne entre forma longa e curta** do mesmo nome (p.116-118).
- **Aplicacao direta:** nome de ferramenta, hook, skill e comando; mensagens de log; titulo de
  secao de reference. Dois nomes para o mesmo conceito fazem o leitor (ou o agente) tratar como duas
  coisas (p.125).

## 4. Checklist minimo de documento tecnico

Todo doc tecnico declara, no inicio: (p.127)

- [ ] **pontos-chave** — o que o leitor leva embora se parar aqui.
- [ ] **scope** — o que o documento cobre.
- [ ] **nonscope** — "State the nonscope" (p.127).
- [ ] **audiencia pretendida** — "State the intended audience" (p.127).
- [ ] **pre-requisitos / leitura obrigatoria** (p.127).

- **Anti-padrao que isso previne:** o leitor consome o documento inteiro para descobrir que nao era
  para ele (p.127).
- **Nonscope declarado** evita exatamente esse consumo: o leitor descarta o documento cedo, em vez de
  descobrir no fim que estava fora do escopo (p.127).

| Item | Pergunta de teste | Evidencia |
| --- | --- | --- |
| pontos-chave | se o leitor parar na primeira secao, ele sabe o resultado? | p.127 |
| scope | o que este documento cobre esta escrito em uma frase? | p.127 |
| nonscope | o que ele **nao** cobre esta escrito? | p.127 |
| audiencia | esta dito para quem o documento foi escrito? | p.127 |
| pre-requisitos | esta dito o que ler antes? | p.127 |

## 5. Aplicacao ao artefato de `docs-research`

| Secao do artefato | Regra | Evidencia |
| --- | --- | --- |
| Primeira linha | achado principal + decisao/pergunta para o leitor | p.119, p.296 |
| Paragrafo de abertura | argumento que sustenta o achado | p.119-121 |
| Escopo | o que foi investigado e o que ficou fora | p.127 |
| Audiencia | quem deve ler e o que precisa saber antes | p.127 |
| Corpo | um termo por conceito, acronimo definido | p.116-118, p.125 |
| Paragrafos | 3 a 5 frases, uma razao cada | p.124 |
| Dados | depois da conclusao e dos argumentos | p.119 |

- O documento nao pode abrir pelo metodo de busca: metodo e dado de apoio, nao conclusao (p.119).
- Nome de biblioteca, flag e comando seguem a regra de vocabulario unico (p.125); a verificacao do
  nome na fonte oficial ja esta no `SKILL.md`.

## 6. Antes e depois

| Antes | Depois | Regra aplicada |
| --- | --- | --- |
| "Pesquisei varias fontes e em algumas o comportamento aparece como deprecated, mas a doc oficial ainda descreve a assinatura antiga, e ha uma issue aberta, entao talvez seja melhor confirmar." | "A assinatura antiga ainda aparece na doc oficial; confirme a versao antes de usar. A issue aberta nao foi aceita." | conclusao primeiro (p.119) |
| "O framework possui um sistema de plugins que e configurado via arquivo e que tambem permite extensao por codigo, o que da flexibilidade." | "O framework configura plugins por arquivo. Extensao por codigo existe, mas nao substitui a configuracao." | uma razao por frase (p.124) |
| "Ver o PRM (o gerenciador) e o prm-handler (o handler) do PRM." | "Ver `prm-handler`, o handler do PRM (gerenciador de pacotes)." | um termo por conceito + acronimo definido (p.125) |
| "O endpoint mudou e agora retorna 404 e nao 200, mas antes funcionava, entao a migracao pode ter quebrado." | "O endpoint retorna 404 na versao nova; a migracao e a causa provavel." | uma razao por frase (p.124) |
| "Investiguei o tema; este documento cobre a API de streaming." | "A API de streaming aceita `--follow`. Este documento cobre apenas streaming; batch esta fora." | conclusao primeiro + nonscope (p.119, p.127) |

## 7. Erros comuns

| Erro | Correcao | Evidencia |
| --- | --- | --- |
| Pedido no ultimo paragrafo | mover para a primeira frase | p.119-121 |
| Frase com duas ideias ligadas por "e" | dividir em duas frases | p.124 |
| Paragrafo de 10+ frases | cortar para 3-5 ou dividir | p.124 |
| Dois nomes para o mesmo componente | escolher um e declarar a diferenca do outro | p.125 |
| Sigla sem definicao | definir na primeira ocorrencia | p.116, p.125 |
| Documento sem nonscope | declarar o que nao cobre | p.127 |
| Metodo de busca abrindo o texto | mover metodo para depois do achado | p.119-121 |

## 8. Revisao em 60 segundos

1. A primeira frase diz o achado e o que voce quer do leitor? (p.119, p.296)
2. Cada frase tem uma razao? Cada paragrafo tem 3-5 frases? (p.124)
3. Existe um unico nome por conceito? Todo acronimo esta definido? (p.116-118, p.125)
4. O inicio declara pontos-chave, scope, nonscope, audiencia e pre-requisitos? (p.127)

## Fontes

- Communication Patterns — A Guide for Developers and Architects (Jacqui Read) — cap. 7, Escrita
  tecnica (p.113-127): Acronym Hell (p.116, p.118); Structured Writing / Minto (p.119-121, com o
  exemplo de email em p.296); Syntax of Technical Writing (p.123-124); Audience Empathy (p.125);
  checklist de documento (p.127).
- Nota de escopo: as unidades de audiencia e niveis de abstracao do mesmo livro estao em p.20-28
  (cap. 1) e nao foram usadas aqui por ficarem fora do capitulo pedido; "big picture first" esta em
  p.63-65 (cap. 4).
