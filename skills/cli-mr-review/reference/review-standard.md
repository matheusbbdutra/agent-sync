# Padrão de revisão

Fonte: Google `eng-practices` (CC BY 3.0). Reescrito; trechos literais citados entre aspas.
Citação por arquivo#seção.

Complementa `code-review-checklist.md` (o que inspecionar); aqui está **o critério de aprovação,
a severidade do achado e o que fazer quando autor e revisor discordam**.

## O padrão de aprovação

> "reviewers should favor approving a CL once it is in a state where it definitely improves the
> overall code health of the system being worked on, even if the CL isn't perfect"
> — `reviewer/standard.md#The Standard of Code Review`

Consequências práticas:

- Não existe código "perfeito", só código **melhor**. Segurar merge por polimento é perda da equipe,
  não rigor (`standard.md#The Standard of Code Review`).
- O contrapeso é a proibição de degradar: nada autoriza mergear mudança que **piora** o code health,
  exceto emergência (`standard.md#The Standard of Code Review`).
- Melhoria incremental justifica aprovação; "não é o design que eu faria" não justifica bloqueio.

## Severidade do comentário

Marque o comentário para o autor saber o que é obrigatório (`reviewer/comments.md#Label comment severity`):

| Marcador | Significado | Autor pode ignorar? |
|---|---|---|
| (sem prefixo) | precisa ser resolvido para aprovar | não |
| `Nit:` | polimento, correto mas não essencial | sim |
| `Optional:` / `Consider:` | sugestão, alternativa válida | sim |
| `FYI:` | informação, sem ação | sim |

Regras de comentário:

- Explique **por quê**, não só o quê (`comments.md#Explain Why`). Achado sem razão vira preferência
  pessoal e abre discussão improdutiva.
- Comentário puramente educativo já nasce `Nit:` (`standard.md#Mentoring`).
- Pedir explicação de trecho que você não entendeu é legítimo, desde que não seja apenas
  complexidade desnecessária (`comments.md`).

## O que conta como problema

Fatos técnicos vencem opinião e preferência (`standard.md#Principles`). Sobre estilo, o style guide
do projeto é autoridade; ponto de estilo ausente do guia é preferência — se não há precedente no
código, aceite o do autor (`standard.md#Principles`).

Em design, "quase nunca é questão de estilo": se o autor demonstra com dado ou princípio de
engenharia que as duas opções são válidas, o revisor aceita a dele; caso contrário, decide o
princípio de design (`standard.md#Principles`).

Lista do que se procura (`reviewer/looking-for.md`):

- design adequado ao sistema, não genérico;
- funcionalidade faz o que o autor quis, inclusive para o usuário final;
- mudança de UI faz sentido e não quebra fluxo;
- concorrência/paralelismo seguros;
- **não há complexidade além do necessário**, nem código "para o futuro" que ainda não é preciso;
- testes unitários presentes **e bem desenhados**;
- nomes claros para tudo;
- comentários explicam **por quê**, não **o quê**;
- documentação atualizada;
- conformidade com o style guide.

## Tamanho da mudança

Uma CL deve mudar **uma coisa** (`developer/small-cls.md`). Mudança pequena é revisada mais rápido,
mais a fundo, com menos bug, menos retrabalho, merge e rollback mais simples
(`small-cls.md`).

Limites úteis, todos do mesmo arquivo:

- inclua o código de teste relacionado na mesma mudança;
- tudo que o revisor precisa para entender a mudança entra junto (exceto trabalho futuro);
- o sistema continua funcionando para usuários e para o time depois da mudança;
- não tão pequena que o efeito global fique incompreensível;
- apagar um arquivo inteiro conta como uma linha.

## Velocidade

> "One business day is the maximum time it should take to respond to a code review request"
> — `reviewer/speed.md`

- O que se otimiza é a velocidade do **time**, não a conveniência do revisor
  (`speed.md`).
- Revisão lenta não é neutra: aumenta a chance de degradação do code health pelo acúmulo de atalhos
  (`speed.md`).
- Resposta rápida ao longo do processo é mais barata para o time do que uma revisão "perfeita" tardia
  (`speed.md`).
- Aprovar quando: o revisor confia que os pontos serão tratados, e os comentários restantes não
  **precisam** ser resolvidos (`speed.md`).

## Conflito e escalada

Ordem de resolução (`standard.md#Resolving Conflicts`):

1. consenso com base no guia de revisão;
2. conversa direta (presencial/vídeo) quando o comentário não resolve — e o resultado **precisa**
   ser registrado como comentário na mudança;
3. escalada: discussão de time, tech lead, maintainer ou eng manager.

> "Don't let a CL sit around because the author and the reviewer can't come to an agreement."
> — `standard.md#Resolving Conflicts`

## Tradução para revisão local (`cli-mr-review`)

- Todo achado do relatório recebe severidade: sem prefixo = bloqueia aprovação; `Nit:` não bloqueia.
- Achado sem "por quê" e sem referência (linha, regra, doc) volta para reescrita, não para o autor.
- Aprovação se decide por "melhora o code health", não por checklist completo.
- Mudança grande demais para revisar de verdade é achado de primeira ordem: peça split
  (`small-cls.md`), não aprove por cansaço.
