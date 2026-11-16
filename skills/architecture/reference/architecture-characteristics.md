# Architecture Characteristics (-ilities)

> Carregue quando a tarefa for decidir **quais** requisitos nao-funcionais valem, como medi-los e como impedir que erodem.
> Nao repete a rubrica de revisao, a dependency rule nem o template de ADR; para fronteira e granularidade de servico a rota e `microservices-patterns`.
> Citacoes usam o marcador de pagina `p.N` do digest de origem (nao o rodape impresso).
> Ordem de leitura: §2 e o pipeline; §12 e o que reprova; §13 e o gate de entrega.

## 1. Triagem: isto e caracteristica arquitetural?

Eleve um requisito a caracteristica arquitetural somente se os tres criterios forem verdadeiros (p.71):

| # | Criterio | Pergunta de teste | Falha se |
| --- | --- | --- | --- |
| a | Consideracao de design **fora do dominio** | o requisito fala de como o sistema se comporta, nao do que o negocio faz? | o item e regra de negocio disfarcada de -ility |
| b | **Influencia** algum aspecto estrutural | mudar isso muda fronteira, deploy, modelo de dados ou topologia? | o design padrao ja resolve |
| c | **Critico** para o sucesso da aplicacao | falhar nisso faz o sistema falhar para o usuario? | e preferencia, nao requisito |

- Os tres valem juntos ou o item nao e arquitetural (p.71).
- Requisito que falha em (b) e higiene de codigo. Higiene se aceita sem discussao de trade-off; arquitetura nao (p.71).
- O exemplo do livro separa integracao de dominio: "third-party payment processor" e consideracao fora do dominio; "in-application payment processing" e dominio (p.72).
- Consequencia: adjetivo sem efeito estrutural nao entra na lista. Lista inflada produz design generico (§4, §10).
- Seguranca, performance, privacidade e auditabilidade passam pela mesma triagem — nenhuma entra por default (p.71).
- Se voce nao consegue responder (b) com uma fronteira, um deploy ou um modelo de dados, a resposta e "nao" (p.71, p.72).

## 2. Pipeline completo (nesta ordem)

Nao pule etapa. Cada uma produz o artefato que a seguinte consome.

| # | Etapa | Artefato de saida | Criterio de saida |
| --- | --- | --- | --- |
| 1 | Identificar explicitas e implicitas | lista bruta com origem por item | todo item rastreia a requisito, decisao de negocio ou conhecimento de dominio (p.80, p.88) |
| 2 | Reduzir | lista curta (alvo: 3) | stakeholder escolheu as 3 principais (p.81) |
| 3 | Definir objetivamente | metrica + alvo + condicao | duas equipes concordam no que passa e no que falha (p.92) |
| 4 | Medir | valor atual ou baseline declarado | existe numero medido, nao so alvo (p.92) |
| 5 | Governar | fitness function no build | build falha quando a caracteristica erode (p.97, p.99) |
| 6 | Escopar | caracteristica por quantum | quantum nomeado com seus componentes e dados (p.105, p.111) |

- Pular da etapa 1 para a 5 produz verificacao de caracteristica que ninguem pediu (p.80, p.97).
- Pular a 3 produz metrica inventada por quem implementa, nao acordada com quem paga (p.92).
- Pular a 6 produz SLA global que nenhum componente sustenta sozinho (p.111).

- A ordem importa: identificar antes de reduzir evita cortar a caracteristica que ninguem lembrou de citar (p.80).
- A saida do pipeline e uma lista curta com numeros, nao um documento de requisitos (p.81, p.92).
- Cada etapa tem dono: a 1 e 2 sao do arquiteto com o stakeholder; a 3 e 5 sao do arquiteto com o time (p.81, p.100).

Saida do pipeline, forma minima (um bloco por caracteristica aceita):

```
caracteristica: <nome>
metrica:        <o que se mede>            # etapa 3 (p.92)
alvo:           <numero>                   # etapa 3 (p.92)
condicao:       <janela, carga, ambiente>  # etapa 3 (p.92)
baseline:       <valor medido hoje>        # etapa 4 (p.92)
fitness:        <verificacao que roda no build>  # etapa 5 (p.97, p.101)
quantum:        <componentes + dados>      # etapa 6 (p.105, p.111)
```

Bloco com `fitness` vazio e caracteristica aceita mas nao governada (p.97). Bloco com `quantum` vazio vira SLA global por omissao (p.111).

## 3. Etapa 1 — extrair explicitas E implicitas

- **Explicitas** estao escritas no requisito: numero de usuarios, SLA, volume, latencia citada. Varra o documento inteiro, nao so a secao de requisitos nao-funcionais (p.80).
- **Implicitas** nao estao escritas e saem do conhecimento de dominio. O livro e explicito: muitas caracteristicas arquiteturais nao estao especificadas em documentos de requisitos (p.88).
- Decodifique linguagem de negocio em equivalente de engenharia em vez de aceitar o adjetivo (p.85).
- Nao traduza um requisito de negocio em uma unica -ility: "agility" nao e time to market, e agility + testability + deployability (p.82).
- Perguntas de extracao, uma por bullet:
  - quantos usuarios simultaneos e em que janela de pico? (p.80)
  - qual SLA externo ja foi prometido a cliente ou contrato? (p.80)
  - qual volume de dado e taxa de crescimento? (p.80)
  - que evento de negocio concentra carga (inicio de campanha, fechamento, virada de mes)? (p.80)
  - que perda operacional o negocio ja declarou inaceitavel? (p.85)
- Caracteristica sem dono de negocio nao sobrevive a discussao de prioridade na etapa 2 (p.81).
- Marque cada item como "medido" ou "suposto". Suposicao sem medicao entra na etapa 4 como pendencia, nao como fato (p.92).

## 4. Etapa 2 — reduzir, pedindo apenas as 3 principais

- Objetivo declarado: manter a lista final o mais curta possivel (p.80).
- Peca aos stakeholders as tres mais importantes, em qualquer ordem (p.81).
- "Em qualquer ordem" e a parte que se ignora por engano: exigir ranking total transforma prioridade em disputa de posicao (p.81).
- Nao tente ranquear 15 itens; o resultado e empate tecnico e decisao por hierarquia (p.81).
- Fundamento do limite: nao existe arquitetura melhor, so a menos pior (p.78); o que se otimiza e a colecao de trade-offs (p.90).
- Se o stakeholder insiste que "tudo e critico", a resposta nao e lista maior: e escolher qual falha e aceitavel quando duas caracteristicas conflitam (§10) (p.90).
- Se as 3 escolhidas nao cobrem o risco real ja conhecido, registre a divergencia; nao acrescente uma quarta em silencio (p.80, p.81).
- Repita a extracao quando o requisito mudar de fase (piloto para producao); a lista de 3 e do momento, nao eterna (p.80).

## 5. Etapa 3 — definicao objetiva e composicao

- Toda caracteristica aceita precisa de definicao objetiva: metrica, alvo e condicao de medicao. Definicao objetiva resolve os tres problemas de discussao do livro (p.92).
- Forma minima, nesta ordem: metrica (o que se mede), alvo (o numero), condicao (janela, carga e ambiente em que o numero vale) (p.92).
- Faltando a condicao, o alvo e inauditavel: o mesmo "200 ms" significa coisas diferentes em p50 e p99, sob carga e em repouso (p.92).
- Caracteristica composta se decompoe antes de projetar. O livro decompoe agilidade em modularity, deployability e testability (p.82, p.92).
- Teste de mesa: duas pessoas leem a definicao e chegam ao mesmo veredito sobre um caso concreto. Se nao chegam, a definicao nao existe (p.92).
- Defina tambem o que **nao** conta; sem isso a medicao vira discussao de interpretacao depois do incidente (p.92).
- Recuse caracteristica que so existe como adjetivo no documento ("escalavel", "seguro") (p.92).

Erros de definicao que aparecem na pratica:

- metrica sem alvo: nao ha como reprovar nada, so descrever (p.92);
- alvo sem condicao: o mesmo numero vale para repouso e pico, o que nunca e verdade (p.92);
- metrica composta que ninguem sabe decompor: sinal de que a caracteristica ainda nao foi entendida (p.82, p.92);
- definicao copiada do fornecedor: descreve o produto, nao a sua aplicacao (p.92).

## 6. Etapa 5 — governar com fitness function, nao com revisao

- Definicao do livro: "qualquer mecanismo que fornece uma avaliacao objetiva de integridade de alguma caracteristica arquitetural ou combinacao delas" (p.97).
- Revisao de codigo ajuda, mas acontece tarde demais no ciclo para ser o mecanismo de governanca (p.99).
- O exemplo concreto do livro e regra de camadas escrita como teste de arquitetura em ArchUnit (p.101).
- Explique o proposito da verificacao ao time **antes** de impo-la; regra imposta sem motivo declarado e contornada (p.100).
- Regras operacionais:
  - uma fitness function por caracteristica critica; caracteristica sem verificacao nao esta governada (p.97);
  - a verificacao roda no build e falha o build; alerta em dashboard nao e gate (p.99, p.101);
  - verificacao manual so para o que nao e automatizavel, com dono e cadencia declarados (p.99);
  - a regra nasce da definicao objetiva da etapa 3, nao de preferencia do arquiteto (p.92, p.97).
- Efeito esperado: erosao silenciosa de camada e ciclo de dependencia param de chegar tarde, quando ja estao em producao (p.99).

A definicao de p.97 e ampla ("qualquer mecanismo"), entao a forma e escolha do arquiteto. A unica forma que o texto do livro exemplifica e teste de arquitetura, com regra de camadas em ArchUnit (p.101).

- Escolha a forma pela caracteristica, nao pela ferramenta: teste de arquitetura para dependencia/camada (p.101); qualquer outra forma so vale se produz avaliacao objetiva de integridade (p.97).
- A escolha da forma nao muda o gate: qualquer que seja, ela precisa poder falhar o build (p.99).
- O mesmo padrao vale para contrato de API: diff de spec que quebra compatibilidade falha o build e exige override consciente (ver `api-operations.md`).

## 7. Etapa 6 — escopar por quantum, nao por sistema

- Definicao do livro: quantum e o "artefato deployavel de forma independente com alta coesao funcional e connascence sincrona" (p.105).
- Os dados fazem parte do quantum: o banco entra no escopo (p.106).
- Sistema legado com um unico banco compartilhado e, por definicao, um quantum de um (p.106).
- Caracteristicas sao escopadas no nivel do quantum (p.111).
- Regra derivada: componentes ligados por chamada sincrona nao podem ter caracteristicas operacionais muito diferentes. Chamador que exige 99,9% sobre chamado de 99% herda 99% (p.105, p.106).
- Nao existe "o SLA do sistema": existe o SLA de cada quantum, e o caminho sincrono vale o elo mais fraco (p.106).
- Tres checagens de invalidacao do escopo:
  - o componente entra em mais de um deploy independente? entao ele pertence a mais de um quantum (p.105);
  - a caracteristica depende de dado compartilhado? o dono do dado define o quantum (p.106);
  - duas partes sincronas exigem numeros incompativeis? ou viram quanta distintos, ou uma das metas e ficção (p.105, p.106).
- Antes de prometer numero global, liste os quanta e o SLA de cada um; a promessa real e o produto do caminho (p.111).

Registro por quantum (o artefato da etapa 6):

```
quantum:        <nome do artefato deployavel>       # p.105
componentes:    <lista sincrona entre si>           # p.105
dados:          <banco/schema que o quantum possui> # p.106
caracteristicas: <as que valem para ESTE quantum>   # p.111
```

- Quantum sem lista de dados esta incompleto: o dado faz parte do escopo (p.106).
- Dois quanta com numeros incompativeis no caminho sincrono sao uma promessa falsa ao usuario (p.105, p.106).

## 8. Contar quanta para decidir monolito versus distribuido

- "A decisao fundamental repousa em quantos quanta a arquitetura descobre durante o design" (p.129).
- Monolito versus distribuido e a primeira determinacao do arquiteto, antes de escolher o estilo (p.276).
- Regra: se um unico conjunto de caracteristicas serve para todo o sistema, prefira monolito; distribuir so paga quando as caracteristicas divergem (p.129).
- Decida na fase de componentes, nao depois do codigo escrito: reverter topologia com sistema em producao custa a migracao inteira (p.129, p.276).
- Antes de aprovar distribuicao, meca a latencia real do ambiente (media e p95/p99) e multiplique pelas chamadas encadeadas do fluxo: 10 chamadas x 100 ms = 1 s adicionado (p.137).
- A cauda longa, nao a media, define a viabilidade do requisito de resposta (p.137).
- Distribuicao sem divergencia de caracteristica paga custo distribuido sem retorno; o criterio de p.129 e a contagem de quanta, nao a moda tecnologica.
- Se a divergencia e so de time (ownership) e nao de caracteristica, o problema e organizacional; resolver com fronteira de modulo, nao com deploy separado (p.129, p.276).

Pre-condicoes para aprovar distribuicao (todas verificaveis):

- [ ] a divergencia de caracteristica esta nomeada e medida dos dois lados (p.129);
- [ ] a latencia p95/p99 do ambiente esta medida, nao estimada (p.137);
- [ ] o numero de chamadas encadeadas do fluxo critico esta contado (p.137);
- [ ] o custo de reverter a topologia foi declarado (p.129, p.276).

## 9. Dividir um componente: a divergencia e a causa

- Motivo legitimo de dividir: divergencia de caracteristica ou de publico, nao gosto funcional (p.127).
- Caso do livro: Bid Capture e dividido em Bid Capture e Auctioneer Capture porque as caracteristicas exigidas diferem (p.127).
- Os dois extremos erram: granularidade fina demais gera comunicacao excessiva; grossa demais gera acoplamento interno alto e deploy dificil (p.123).
- Nao derive componentes do modelo de dados (entity trap): componente representa fluxo de trabalho de dominio, nao tabela (p.124).
- Se o desenho e um mapeamento objeto-relacional, o requisito real e CRUD e nao precisa de arquitetura (p.124).
- Componente unico por conveniencia funcional forca o mesmo SLA para o publico critico e o massivo — sintoma de fronteira errada (p.123, p.127).
- Fronteira de servico (transacao, isolamento de dados, iteracao) e assunto de `microservices-patterns`; aqui fica o criterio de caracteristica (p.123, p.127).

## 10. Trade-off, "least worst" e conflito entre caracteristicas

- Toda decisao tem contrapartida; se voce nao consegue nomear a perda, a analise nao terminou (p.36).
- O porque e mais importante que o como: registre o motivo, nao apenas o resultado (p.36).
- Otimize a colecao de trade-offs, nao cada caracteristica isolada (p.90).
- Declare o que foi sacrificado. Caracteristica omitida nao desaparece: ela volta como bug sem dono (p.90).
- Quando duas caracteristicas conflitam, o artefato de saida e a escolha explicita de qual falha e aceitavel, com o numero de cada lado (p.90).
- Trade-off nao escrito e trade-off que sera reaberto por quem nao participou — registre na ADR (p.36).

## 11. Da caracteristica para a decisao registrada

- A escolha das 3 caracteristicas e a topologia sao decisao estruturante: vao para ADR com contexto, opcoes descartadas e consequencias negativas (p.36).
- A ADR cita a metrica e a condicao da etapa 3; sem elas a decisao nao pode ser reavaliada depois (p.92).
- A fitness function e a parte executavel da decisao; a ADR aponta onde ela vive no build (p.97, p.101).
- Template e regras de redacao: `adr-guidelines.md`.

## 12. Anti-padroes

- **Arquitetura generica**: lista longa de caracteristicas, todas importantes, design sem foco (p.78, p.90).
- **Higiene elevada a arquitetura**: todo adjetivo do requisito virou necessidade estrutural (p.71, p.72).
- **Lista nao priorizada**: 12 caracteristicas sem as 3 principais declaradas (p.80, p.81).
- **Composta sem decompor**: "usabilidade" ou "agilidade" no design sem metrica derivada (p.82, p.92).
- **Definicao sem condicao**: alvo sem janela, carga e ambiente; inauditavel (p.92).
- **Governanca por revisao**: nenhuma verificacao no build; erosao descoberta meses depois (p.99).
- **Fitness function sem explicacao**: time contorna a regra que nao entende (p.100).
- **SLA global**: um numero para o sistema inteiro, ignorando quantum e dependencia de dados (p.106, p.111).
- **Distribuido sem divergencia**: microsservicos com um unico conjunto de caracteristicas (p.129).
- **Entity trap**: um componente por tabela, reproduzindo o banco na camada de aplicacao (p.124).
- **Latencia no papel**: distribuicao aprovada sem p95/p99 medido do ambiente (p.137).
- **Caracteristica sem dono**: item aceito que nenhum stakeholder reivindica na etapa 2 (p.81).
- **Metrica sem baseline**: alvo escrito sem valor atual medido; nao se sabe se melhorou ou piorou (p.92).

## 13. Checklist antes de fechar a lista

- [ ] Cada caracteristica passa nos tres criterios (p.71).
- [ ] Explicitas e implicitas levantadas; origem de cada item registrada (p.80, p.88).
- [ ] Cada item marcado como medido ou suposto (p.92).
- [ ] As 3 principais foram escolhidas pelo stakeholder, em qualquer ordem (p.81).
- [ ] Cada uma tem metrica + alvo + condicao; as compostas foram decompostas (p.82, p.92).
- [ ] Existe baseline medido, nao so alvo (p.92).
- [ ] Cada caracteristica critica tem fitness function no build, e o time sabe por que ela existe (p.97, p.100).
- [ ] Cada caracteristica esta atribuida a um quantum nomeado (p.111).
- [ ] Topologia (monolito x distribuido) decidida pela contagem de quanta (p.129).
- [ ] Latencia p50/p95/p99 do ambiente consta na decisao de distribuir (p.137).
- [ ] O trade-off esta escrito, com o que foi sacrificado (p.36, p.90).
- [ ] A decisao estruturante virou ADR com as consequencias negativas (p.36; ver `adr-guidelines.md`).

## Fontes

- **Fundamentals of Software Architecture — An Engineering Approach** (Mark Richards, Neal Ford), cap. 2 "Architectural Thinking" (p.36), cap. 4 "Architecture Characteristics Defined" (p.71, p.72, p.78), cap. 5 "Identifying Architectural Characteristics" (p.80, p.81, p.82, p.85, p.88, p.90), cap. 6 "Measuring and Governing Architecture Characteristics" (p.92, p.97, p.99, p.100, p.101), cap. 7 "Scope of Architecture Characteristics" (p.105, p.106, p.111), cap. 8 "Component-Based Thinking" (p.123, p.124, p.127), cap. 9 "Foundations" (p.137), cap. 17 "Microservices" (p.256), cap. 18 "Choosing the Appropriate Architecture Style" (p.276).
- **Mastering API Architecture** (James Gough, Daniel Bryant, Matthew Auburn), cap. 5 (p.160) — fitness function aplicada a contrato de API, referenciada no §6.
