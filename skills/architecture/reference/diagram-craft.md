# Diagram Craft (diagramas de arquitetura)

> Carregue quando for produzir ou revisar um diagrama de arquitetura — C4, contexto, container, sequencia ou fluxo.
> Escopo: o que entra, como rotular, como nao enganar o leitor. Nao cobre preenchimento de ADR nem rubrica de review.
> Citacoes usam o marcador `p.N` do digest de origem.

## 0. Qual artefato produzir

Escolha o artefato pela pergunta que o leitor vai fazer, nao pela ferramenta que voce tem aberta.

| Pergunta do leitor | Artefato | O que NAO entra | Fonte |
| --- | --- | --- | --- |
| quem conversa com o sistema e por qual canal | contexto (C4) | classe, tabela, fila interna | p.321 |
| o que executa e onde roda | container (C4) | fluxo de dados passo a passo | p.321 |
| de que o container e feito por dentro | componente (C4) | deploy, infraestrutura | p.321 |
| como um fluxo se comporta, inclusive em falha | sequencia/comportamento | topologia estatica | p.84 |
| o que esta implantado em qual ambiente | estrutura/deploy | sequencia de chamadas | p.84 |

- Uma pergunta, um artefato. Duas perguntas na mesma imagem e o caso de §3 (p.28, p.84).
- Se voce nao sabe qual pergunta o leitor faz, o problema e de audiencia, nao de diagrama (p.28).

## 1. Um nivel de abstracao por artefato

- Nao misture contexto, container, componente e codigo no mesmo artefato. Dois niveis no mesmo desenho produzem clutter e confusao do ponto de vista da audiencia (p.28).
- A regra de separacao vale para **todos** os tipos de diagrama, nao so para C4 (p.28).
- Se dois niveis sao necessarios, produza dois artefatos ligados entre si (§2) em vez de um artefato denso (p.28, p.30).
- Teste rapido: se o leitor precisa ignorar metade das caixas para entender a mensagem, ha dois niveis no mesmo desenho (p.28).
- Nivel de abstracao tambem se aplica a texto dentro do desenho: nome de classe em diagrama de contexto e ruido (p.28).

## 2. Ligacao explicita entre artefatos do mesmo sistema

- Torne a correspondencia visivel e nomeada: label, ID, hyperlink ou caixa tracejada rotulada (p.30).
- Nao deixe o leitor inferir que "o servico X do diagrama A" e "o componente X do diagrama B" sao o mesmo — inferencia errada e o modo de falha padrao (p.30).
- O livro pede consistencia representacional **explicita** em diagramas e documentacao, e justifica pelo custo cognitivo da audiencia (p.35).
- Regra pratica: mesmo elemento, mesmo nome e mesmo simbolo em todos os artefatos do conjunto (p.35).
- Consequencia para docs versionados: o conjunto de diagramas e um artefato unico com N visoes, nao N artefatos independentes (p.35).

## 3. Estrutura e comportamento sao artefatos diferentes

- Diagrama estrutural comunica **o que e onde**; diagrama comportamental comunica **como e para quem** (p.84).
- Se o desenho responde as duas perguntas, divida em dois. A mensagem se perde quando as duas convivem na mesma imagem (p.84).
- Na pratica: topologia/deploy em um artefato; fluxo, sequencia e falha em outro (p.84).
- Nao use a mesma notacao para os dois fins so porque a ferramenta permite (p.84).

## 4. Relacoes: unidirecionais e rotuladas com o que flui

- Uma seta por relacao, rotulada com o que passa naquela direcao (p.71).
- Seta de duas pontas so quando o mesmo processo ocorre nos dois sentidos, o que e raro; na duvida, use duas setas (p.72).
- Bidirecional ambiguo perde a informacao de quem chama quem — a informacao simplesmente desaparece do desenho (p.72).
- Rotule com o dado ou o comando que trafega, nao com "chamada" ou "comunicacao" (p.71).
- Para request/response, o request segue o sentido do texto e o response flui na direcao oposta (p.92).
- Marque o que e sincrono e o que e assincrono no proprio desenho ou na legenda (§5, §9).

## 5. Cor, legenda e legibilidade

- Nenhum significado pode existir **so** em cor. Para cada significado codificado em cor, adicione um segundo canal: padrao, simbolo ou texto (p.51, p.53).
- Inclua legenda sempre que houver convencao nao obvia; se decidir nao incluir, registre o motivo ao lado do artefato (p.58).
- Seja explicito, nao implicito: a legenda declara o que cada canal significa (p.58).
- Fonte minima de 12 pt em qualquer diagrama entregue ou commitado (p.61).
- Daltonismo, impressao em grayscale e projetor sao o cenario de teste da regra, nao a excecao (p.51, p.53).

Verificacao de cor e legibilidade, na ordem:

1. converta o desenho para grayscale: se a informacao desaparece, a cor estava sozinha (p.51);
2. desligue a legenda: se o leitor erra o significado, a legenda era necessaria e nao estava la (p.58);
3. meca a fonte no artefato exportado, nao no editor (p.61);
4. mostre a alguem de fora do projeto e pergunte o que cada cor significa (p.53).

## 6. Fluxo e layout seguem a expectativa de leitura

- Inicio no topo-esquerda (ou meio-esquerda); fluxo topo para baixo e esquerda para direita (p.67).
- Layout contra a expectativa de leitura obriga o leitor a reconstruir a ordem — custo pago em toda leitura (p.67, p.69).
- Canvas landscape em proporcao 16:9 ou 16:10 quando o consumo for tela (p.92).
- Request e response na mesma seta ou na mesma direcao e erro de leitura, nao de estetica (p.92).
- Antes de entregar, mostre o desenho por 10 segundos para alguem de fora e pergunte onde comeca (p.69).

- Mantenha o mesmo sentido de leitura em todos os artefatos do conjunto; inverter o eixo entre dois diagramas cobra do leitor a remontagem (p.67, p.69).
- Barra de progresso, numero de passo e ordem implicita no nome dos nos sao alternativas quando o fluxo e ramificado (p.69).

## 7. Notacao formal e custo de acesso

- Antes de escolher UML, ArchiMate ou BPMN, verifique se autor **e** audiencia dominam a notacao (p.78).
- Se nao dominam, use notacao simples (C4, caixas e linhas) — o custo de acesso nao paga o rigor (p.79, p.82).
- Nao e obrigatorio usar UML exatamente como especificado quando a especificacao nao atende ao objetivo (p.82).
- Notacao formal limita quem consegue criar ou atualizar o diagrama; o resultado tipico e gargalo de uma pessoa e diagrama desatualizado (p.198).
- Icone nunca e a unica informacao: a mensagem deve permanecer compreensivel se os icones forem removidos (p.77).
- Regra de selecao: notacao e ferramenta de comunicacao, nao certificado de rigor (p.79, p.198).

## 8. Nao distorca escala, baseline ou tamanho de elemento

- Baseline de grafico sempre em zero; se o zero nao for possivel, use tabela (p.99).
- Em visuais lado a lado, use a mesma escala ou torne a diferenca de escala explicita e obvia para a audiencia (p.106).
- Mantenha elementos do mesmo tipo no mesmo tamanho em diagramas logicos — tamanho nao deve sugerir peso que nao existe (p.106).
- O sintoma a evitar: a mesma serie de numeros sustentando duas conclusoes opostas (p.99, p.106).
- Vale para dashboard de metrica e para relatorio de comparacao de performance, nao so para diagrama (p.99).

- Em diagrama de deploy, elemento de mesmo tipo em tamanho diferente sugere capacidade ou papel que nao existe (p.106).
- Composicao enganosa e uma classe de defeito propria: o desenho induz conclusao que os dados nao sustentam (p.103).

## 9. C4: o que usar e onde ele para

- Comece com artefato barato e iteravel; so invista em diagrama polido quando o design estabilizou (p.319).
- O motivo declarado: rascunho efemero impede o apego excessivo ao artefato — "irrational artifact attachment" congela uma decisao ruim (p.319).
- Para comunicar, use os quatro C's: Context, Container, Component (e code) (p.321).
- Convencao de linha: solida indica comunicacao sincrona, pontilhada indica assincrona (p.322).
- Limite declarado pelo livro: C4 e menos adequado a arquiteturas distribuídas (p.322).
- Regra derivada: em sistema distribuido, complemente C4 com um artefato de fluxo/sequencia proprio (§3) em vez de forcar tudo no C4 (p.322).
- Diagrama sem convencao declarada que induz interpretacao errada e pior que nenhum diagrama (p.322).
- C4 descreve estrutura; comportamento de um fluxo especifico exige artefato proprio (§3) (p.322, p.84).
- Rascunho de baixa fidelidade primeiro: o objetivo inicial e decidir, nao apresentar (p.319).

## 10. Anti-padroes

- **Diagrama que explica tudo**: quatro niveis na mesma imagem, nenhum legivel (p.28).
- **Setas bidirecionais por omissao**: perde-se quem chama quem (p.72).
- **Significado so em cor**: parte da audiencia perde a informacao inteira (p.51).
- **Sem legenda**: convencao implicita lida de forma diferente por cada leitor (p.58).
- **Icone como unica informacao**: desenho vira teste de conhecimento de icones de cloud (p.77).
- **UML por rigor**: so quem escreveu consegue manter; vira gargalo (p.198).
- **Grafico com baseline cortado**: mesma serie, duas conclusoes (p.99).
- **Apego ao artefato caro**: diagrama polido cedo demais defende decisao em vez de comunica-la (p.319).
- **Nivel errado para a pergunta**: diagrama de componente respondendo pergunta de contexto (p.321, p.84).
- **Escala invertida entre artefatos**: dois desenhos do mesmo fluxo em sentidos opostos (p.67).

## 11. Checklist antes de entregar

- [ ] Um unico nivel de abstracao no artefato (p.28).
- [ ] Correspondencia com os outros artefatos do conjunto esta rotulada ou linkada (p.30, p.35).
- [ ] Estrutura e comportamento estao em artefatos separados (p.84).
- [ ] Toda seta e unidirecional e rotulada com o que flui (p.71, p.72).
- [ ] Nenhum significado depende so de cor; legenda presente ou motivo registrado (p.51, p.58).
- [ ] Fonte >= 12 pt; icones redundantes, nao portadores unicos de informacao (p.61, p.77).
- [ ] Leitura comeca no topo-esquerda e segue a expectativa (p.67).
- [ ] Sincrono/assincrono esta marcado (linha solida x pontilhada) (p.322).
- [ ] Nenhuma escala ou tamanho induz conclusao falsa (p.99, p.106).
- [ ] Notacao escolhida e dominada por quem vai manter o diagrama (p.78, p.198).

## 12. Como conduzir a revisao do diagrama

- Revise contra a pergunta declarada do leitor (§0), nao contra gosto pessoal (p.84).
- Quem revisa e alguem da audiencia, nao o autor: o autor ja sabe o que quis dizer (p.69).
- Reprova em nivel de abstracao? Divida em dois artefatos ligados (§2) em vez de apagar conteudo (p.28, p.30).
- Reprova em notacao? Troque a notacao e mantenha a mensagem; simplificar a formal e permitido (p.79, p.82).
- Reprova em cor? Acrescente um segundo canal (padrao, simbolo, texto) em vez de trocar a paleta (p.51, p.53).
- Sem legenda por decisao consciente? Registre o motivo junto do artefato (p.58).
- Revise cedo com rascunho efemero e investir no polimento so depois do design estabilizado (p.319).
- Um diagrama reprovado duas vezes na mesma revisao e sinal de que faltam dois artefatos, nao um (p.28).

## Fontes

- **Communication Patterns — A Guide for Developers and Architects** (Jacqui Read), cap. 1 "Essentials" (p.25, p.28, p.30, p.35), cap. 2 "Reduzir carga cognitiva" (p.51, p.53, p.58, p.61), cap. 4 "Narrativa em diagramas" (p.63, p.67, p.69, p.71, p.72), cap. 5 "Notacao" (p.77, p.78, p.79, p.82, p.84), cap. 6 "Composicao" (p.92, p.99, p.103, p.106), cap. 11 "Just-in-Time Architecture" (p.198).
- **Fundamentals of Software Architecture — An Engineering Approach** (Mark Richards, Neal Ford), cap. 21 "Diagramming and Presenting" (p.319, p.321, p.322) — C4, convencao solida/pontilhada e limite em arquitetura distribuida, usados no §9 e §11.
