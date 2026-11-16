# Evidência sobre hooks: o que foi medido, não opinado

Duas fontes públicas com número. Use para escolher **que emoção** o hook puxa e para justificar
teste A/B — não para prometer CTR.

## 1. Upworthy Research Archive — 32.487 testes A/B de manchete

| Fato | Valor |
|---|---|
| Experimentos (package tests) | **32.487** |
| Braços/pacotes de experimento | **150.817** |
| Período | **24/jan/2013 – 30/abr/2015** |
| Impressões | **538.272.878** |
| Cliques | **8.182.674** |

Fonte: *The Upworthy Research Archive, a time series of 32,487 experiments in U.S. media*,
Scientific Data (2021), `https://www.nature.com/articles/s41597-021-00934-7`.

O que isso permite afirmar: existe um corpus grande de manchetes **randomizadas com CTR medido**.
O que **não** permite: prever seu CTR. É manchete de mídia americana, de 2013-2015, para leitor
geral — a distribuição de atenção não é a de um feed profissional em 2026.

Uso correto: a lição é o **loop** — duas variantes, mesma audiência, vencedor fica. Diferença entre
manchetes do mesmo texto no archive é grande justamente porque **palavra escolhida** move mais que
template.

## 2. Berger & Milkman — emoção e viralidade

Estudo: *What Makes Online Content Viral?* (Journal of Marketing Research, 2012; versão MSI Report
10-114, `https://www.msi.org/wp-content/uploads/2020/06/MSI_Report_10-114.pdf`).

Método, para você pesar a evidência:

- **6.956 artigos** do New York Times, publicados entre **30/ago/2008 e 30/nov/2008**, capturados por
  crawler a cada 15 minutos.
- Variável dependente: entrar na lista **"most emailed"** do NYT — **20%** dos artigos entraram.
- Emoções específicas codificadas por **3 avaliadores independentes e cegos** para as hipóteses,
  escala Likert de 5 pontos, em subamostra aleatória de **2.566 artigos** (α > .70).
- Valência/emocionalidade medida por análise textual com lista de **7.630 palavras**.
- Regressão logística com efeito fixo de dia, erros-padrão agrupados por dia.

Achados:

| Emoção / traço | Efeito na chance de viralizar |
|---|---|
| Conteúdo emocional (positivo **ou** negativo) | **+** |
| Valência positiva | **+** (positivo viraliza mais que negativo) |
| Awe | **+** |
| Surpresa | **+** |
| Raiva | **+** — preditor mais forte: **+1 DP → 1,5× mais chance** de entrar na lista |
| Ansiedade | **+** |
| Tristeza | **−** |
| Nojo | sem relação significativa |

## Regras que saem daí

1. **Escolha a emoção antes de escrever a frase.** Hook não é "impactante" em abstrato: é awe,
   surpresa, raiva ou ansiedade. Diga qual você está mirando e escreva para ela.
2. **Tristeza derruba compartilhamento.** Post técnico que abre em tom de lamento (“depois de meses
   perdidos…”) pede pena, não alcance. Se a história é de fracasso, o hook é a **lição**, não a dor.
3. **Raiva viaja, mas tem preço.** Para audiência técnica, o equivalente defensável é a *contrarian
   claim com evidência* — raiva de uma prática, nunca de uma pessoa nomeada.
4. **Surpresa é o mais barato de produzir com honestidade**: número que contradiz a intuição, ou a
   parte que ninguém conta. É o hook que menos depende de marca pessoal.
5. **Teste em par.** Duas variantes, mesma audiência e mesmo horário. Sem par, você não está
   medindo hook — está publicando.
6. **Não transfira número entre plataformas.** “1,5×” é chance relativa de entrar numa lista de
   e-mails em 2008, não CTR no LinkedIn. Efeito relativo orienta escolha; valor absoluto é ficção.

## O que ainda não está medido aqui

- Efeito de **comprimento do hook** e do ponto de corte por plataforma: os limites estão em
  `social-check/references/platform-limits.md`, mas o efeito de cada corte não foi medido.
- Diferença de hook entre LinkedIn, Reddit e X com dado público: não encontrei dataset equivalente
  ao archive para essas plataformas. Se você rodar seus próprios pares, você vira a fonte.
