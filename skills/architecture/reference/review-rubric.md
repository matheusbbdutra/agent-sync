# Review Rubric

> Absorvida por architecture (2026-11-16). Conteúdo original preservado: método de revisão, atribuição de impacto, gates, boas práticas e armadilhas.

Referência carregada por `../SKILL.md` §4. Persona, catálogo de tecnologias e exemplos de prompt da origem não são instrução e foram descartados.

## 1. Método (8 passos, nesta ordem)

1. **Contexto** — levante o estado atual: fronteiras existentes, restrições conhecidas, decisão já registrada. Cite `path:line`.
2. **Impacto** — atribua Alto, Médio ou Baixo (tabela abaixo) antes de opinar. Impacto não declarado vira gosto.
3. **Aderência** — confira se a proposta respeita a dependency rule e os princípios de `../SKILL.md` §2.
4. **Violações** — identifique anti-padrão concreto, não impressão geral.
5. **Recomendação** — proponha refatoração nomeada e delimitada, com o trade-off de cada opção.
6. **Escala** — diga o que acontece com 10× o volume, o time e a frequência de deploy.
7. **Registro** — exija ADR quando a mudança toca contrato, fronteira ou tecnologia ([adr.md](adr.md)).
8. **Próximo passo** — feche com ação verificável (comando, teste, gate) e o que ainda é hipótese.

## 2. Atribuição de impacto

| Impacto | Critério | Efeito no veredito |
|---|---|---|
| Alto | muda contrato público, fronteira de contexto, modelo de dados compartilhado ou tecnologia de persistência | exige plano de validação determinístico **antes** de aprovar |
| Médio | novo módulo, nova dependência externa, mudança de camada | aprova com condição e ADR |
| Baixo | detalhe interno, renomeação, extração local | aprova com teste que cobre o comportamento |

## 3. Rubrica de qualidade (eixos)

- **Fronteiras** — cada contexto tem dono, deploy e modelo próprios? Regra sem dono único é blocker.
- **Acoplamento** — o núcleo importa infraestrutura? Tipo de fornecedor aparece na regra de negócio? Falta ACL?
- **Escalabilidade** — gargalo declarado é o real (N+1, transação longa, lock, chamada serializada) ou é palpite?
- **Consistência** — transação respeita o limite do agregado? Existe consumidor idempotente e compensação?
- **Testabilidade** — a regra roda sem banco, HTTP e rede? Se precisa de infraestrutura, a dependência aponta para o lado errado.
- **Origem da afirmação** — cada achado diz se vem da skill, de decisão registrada em memória/ADR ou de análise nova do código (`path:line`). Opinião sem fonte não entra no relatório.

## 4. Gates de aprovação

Reprova sem discussão:

- Aprovar mudança de impacto Alto sem plano de validação (`go test`, smoke, critério de aceite).
- Achado sem evidência `path:line` ou sem alternativa com trade-off.
- Suposição de comportamento não verificada no código apresentada como fato.
- Hipótese pendente entregue como conclusão; premissa descartada reintroduzida sem justificativa.

## 5. O que o review deve confirmar

1. **Dependency rule** — dependência sempre aponta para dentro.
2. **Interface segregation** — interfaces pequenas e focadas, não `interface`-Deus.
3. **Regra no domínio** — framework fora do núcleo.
4. **Teste independente** — núcleo testável sem infraestrutura.
5. **Bounded context** — fronteira de domínio explícita.
6. **Linguagem ubíqua** — terminologia consistente entre código e negócio.
7. **Controller fino** — delega ao caso de uso.
8. **Modelo rico** — comportamento junto do dado.

## 6. Armadilhas comuns

- **Modelo anêmico** — entidade só com dado; regra num `service` gigante.
- **Acoplamento a framework** — regra de negócio dependendo do framework.
- **Controller gordo** — regra de negócio no handler HTTP.
- **Vazamento de repository** — objeto do ORM exposto para fora do adaptador.
- **Abstração ausente** — dependência concreta dentro do núcleo.
- **Over-engineering** — Clean Architecture aplicada a CRUD simples.
