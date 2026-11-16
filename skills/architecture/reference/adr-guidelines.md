# ADR Guidelines (selecao, redacao e governanca)

> Carregue quando a pergunta for **se** a decisao precisa de ADR, **como** redigir para sobreviver a auditoria e **como** encadear supersessoes.
> Template e convencoes locais (`docs/ADR-*`, ciclo `D-N`/`A-N`) ficam em `adr.md`; aqui esta o criterio de conteudo, nao o formulario.
> Citacoes usam o marcador `p.N` do digest de origem.

## 1. Quais decisoes merecem ADR

Abra ADR quando a decisao toca um destes gatilhos. Nenhum gatilho, nenhuma ADR.

- Outra pessoa vai questionar a decisao depois; sem registro, a mesma discussao volta (p.287).
- A decisao e estruturante: contrato, fronteira, tecnologia persistente, padrao de comunicacao (p.286).
- A justificativa nao cabe no commit nem no PR sem virar ensaio (p.286).
- A decisao so existe hoje em e-mail ou thread de chat — o livro e direto: e-mail comunica bem e documenta mal (p.287).
- Nao abra ADR para detalhe interno de implementacao, renomeacao ou bug fix; nesses casos a decisao ja e o diff (p.287).

## 2. Estrutura: cinco secoes obrigatorias, mais duas

O livro fixa cinco secoes principais: Title, Status, Context, Decision e Consequences (p.289).

| Secao | Conteudo exigido | Erro tipico |
| --- | --- | --- |
| Title | nomeia a decisao tomada, nao o tema | titulo generico ("decisao sobre mensageria") |
| Status | estado atual, com link para o sucessor quando houver | status desatualizado, ADR superada ainda marcada como vigente |
| Context | situacao e forcas que exigem a decisao (p.289) | contexto que so repete a decisao |
| Decision | "we will ..." no imperativo (p.292) | "I think ... would be best" |
| Consequences | trade-off assumido, incluindo os riscos negativos (p.289) | lista so de beneficios |

Duas secoes adicionais que o livro trata como parte do artefato:

- **Compliance**: como a decisao sera medida e governada; e onde o teste que a verifica vive (p.294, p.295).
- **Notes**: autor original, datas e aprovador (p.295).

- Sem a secao Compliance, a decisao nao tem como ser verificada depois — fica no campo da intencao (p.294).
- Sem Notes, ninguem sabe a quem perguntar quando a decisao for questionada (p.295).
- As sete secoes juntas, nao cinco de sete: contexto sem consequencia e proposta, nao decisao (p.289).

## 3. Redacao: imperativo, justificativa dupla, consequencia negativa

- Escreva no imperativo: "we will use asynchronous messaging between services" (p.292).
- Nunca escreva em primeira pessoa do singular. "I think ... would be best" nao registra decisao, registra opiniao (p.286).
- Toda ADR precisa de justificativa tecnica **e** de negocio (p.286).
- As quatro justificativas de negocio mais comuns segundo o livro: custo, time to market, satisfacao do usuario e posicionamento estrategico (p.286).
- A consequencia negativa e obrigatoria. O caso do livro e exemplar: a troca de gRPC por mensageria aumentou a latencia, e a ADR nao registrava o porque — a decisao foi revertida sem que ninguem soubesse o motivo original (p.293).
- Consequencia escrita com numero quando houver: latencia esperada, custo mensal, throughput assumido (p.293).
- Se voce nao consegue escrever uma consequencia negativa, a decisao nao foi analisada como trade-off (p.289).

## 4. Compliance: a decisao precisa de um lugar para morrer

- A secao Compliance forca o arquiteto a pensar em como a decisao sera medida e governada (p.294).
- Declare o mecanismo: verificacao manual com dono, ou verificacao automatizada no build (p.294).
- Aponte o lugar exato da verificacao (arquivo, teste, gate), nao a intencao "vamos monitorar" (p.295).
- Compliance manual aceita so quando nao ha forma automatizada; nesse caso declare a cadencia (p.294).
- Decisao sem Compliance e a que erode primeiro: ninguem sabe quando ela deixou de valer (p.294).

## 5. Status, supersessao e RFC

- Ao substituir uma decisao, o vinculo e **bidirecional**: a antiga recebe "Superseded by N" e a nova registra que supersede N (p.295).
- Registre a data de supersessao em Notes; sem data, a linha do tempo das decisoes se perde (p.295).
- Decisao que precisa de revisao aberta usa status RFC com data-limite explicita — o exemplo do livro e literal: "Request For Comments, Deadline 09 JAN 2010" (p.291).
- RFC sem deadline vira decisao adiada: o prazo e o que transforma comentario em decisao (p.291).
- Status com prazo vencido e decisao inexistente: ou vira aceita, ou vira nova ADR com novo prazo (p.291).
- Mudanca de rumo gera ADR nova que referencia a anterior; a supersessao e o mecanismo de mudanca, nao a edicao do documento antigo (p.295).

## 6. Os tres anti-patterns de decisao, em sequencia

Verifique nesta ordem ao auditar um conjunto de ADRs:

1. **Covering Your Assets** — a decisao foi adiada por medo de errar (p.286). Correcao: decidir no ultimo momento responsavel, nao no primeiro.
2. **Groundhog Day** — a decisao volta porque a justificativa nao ficou registrada (p.287). Correcao: Context + Consequences preenchidos, nao so Decision.
3. **Email-Driven Architecture** — a decisao existe apenas em e-mail/chat e por isso se perde (p.287). Correcao: mover o que importa para o repositorio canonico.

- Os tres produzem o mesmo sintoma final: "as pessoas perdem, esquecem ou nem sabem que uma decisao foi tomada" (p.287).
- Deteccao por sintoma: ADR com prazo sempre adiado e Covering Your Assets (p.286); ADR refeita com o mesmo tema e Groundhog Day (p.287); decisao que so aparece em busca de e-mail e Email-Driven Architecture (p.287).
- Os tres se aplicam a conjunto de ADRs, nao a uma ADR isolada: um unico documento pode estar correto e o processo continuar doente (p.287).
- Auditar um repo com ADRs: se as tres secoes de Context/Consequences/Compliance estao vazias, os tres anti-patterns estao ativos por definicao (p.286, p.294).

## 7. ADR guideline por tipo de decisao recorrente

O livro de API anexa uma "ADR Guideline" a cada decisao que se repete, em vez de deixar o criterio no texto corrido:

| Decisao recorrente | Guideline no livro | Pagina |
| --- | --- | --- |
| Escolher o padrao de API (REST, gRPC, GraphQL) | ADR Guideline: Choosing an API Standard | p.46 |
| Adotar contract testing e qual metodologia | ADR Guideline: Contract Testing | p.73 |
| Selecionar API gateway | ADR Guideline: Selecting an API Gateway | p.116 |
| Escolher grants OAuth2 | ADR Guideline: Which OAuth2 Grants | p.213 |

- Regra derivada: decisoes que reaparecem em todo projeto ganham guideline propria, com os mesmos campos de uma ADR (p.33, p.46).
- O formato base que o livro usa e o minimo: status, context, decision, consequences (p.33).
- Ao escrever uma ADR nova de tipo ja coberto por guideline, siga a guideline e registre apenas o que e especifico do projeto (p.46).

## 8. Classifique antes de decidir: Type 1 versus Type 2

- Classifique a decisao como Type 1 (dificil ou caro de reverter) ou Type 2 (reversivel) antes de escolher o processo (p.261).
- Decisao irreversivel nao e problema por si: o problema e irreversibilidade sem analise (p.228).
- Escolher gateway, service mesh ou tecnologia de borda e, no livro, explicitamente uma decisao Type 1 — exige ADR antes de executar (p.261).
- Type 2 aceita decisao rapida com registro curto; usar processo pesado em decisao reversivel custa velocidade sem ganho (p.261).
- Type 1 com processo leve produz lock-in e custo de saida nao contabilizado (p.228, p.261).
- Teste de classificacao: nomeie o custo concreto de reverter (migracao, contrato quebrado, dados a mover). Se o custo e baixo ou desconhecido, trate como Type 2 e siga (p.261).
- Para Type 1, a ADR e escrita **antes** de executar; para Type 2, o registro pode vir depois da execucao (p.261).

## 9. Checklist antes de abrir ou fechar a ADR

- [ ] A decisao passa em um dos gatilhos de §1 (p.286, p.287).
- [ ] Classificada como Type 1 ou Type 2, e o processo corresponde (p.261).
- [ ] Title nomeia a decisao, nao o tema (p.289).
- [ ] Context descreve as forcas, nao repete a decisao (p.289).
- [ ] Decision esta no imperativo "we will ..." (p.292).
- [ ] Ha justificativa tecnica e de negocio (p.286).
- [ ] Consequences inclui o negativo, com numero quando existir (p.289, p.293).
- [ ] Compliance aponta o mecanismo e o lugar da verificacao (p.294, p.295).
- [ ] Notes traz autor, datas e aprovador (p.295).
- [ ] Se substitui outra ADR, os dois links existem e a data esta registrada (p.295).
- [ ] Nenhum dos tres anti-patterns de §6 esta ativo (p.286, p.287).
- [ ] As cinco secoes do livro estao presentes (Title, Status, Context, Decision, Consequences) (p.289).
- [ ] Se o status e RFC, existe deadline explicito (p.291).

## Fontes

- **Fundamentals of Software Architecture — An Engineering Approach** (Mark Richards, Neal Ford), cap. 19 "Architecture Decisions" (p.286, p.287, p.289, p.291, p.292, p.293, p.294, p.295).
- **Mastering API Architecture** (James Gough, Daniel Bryant, Matthew Auburn), Introduction (p.33), cap. 1 "ADR Guideline: Choosing an API Standard" (p.46), cap. 2 "ADR Guideline: Contract Testing" (p.73), cap. 3 "ADR Guideline: Selecting an API Gateway" (p.116), cap. 7 "ADR Guideline: Which OAuth2 Grants" (p.213), cap. 8 "Using Fitness Functions" (p.228), cap. 10 "Understanding Decision Types" (p.261).
