# Domain Modeling (DDD)

Referência carregada por `../SKILL.md` §1. Cobre nível estratégico (fronteiras, linguagem) e tático (agregados, VOs, eventos). A ordem segue Evans: design estratégico (Parte IV) e ciclo de vida do objeto de domínio (cap. 6).

## 1. Quando o modelo ganha dinheiro

DDD é ferramenta de domínio complexo: o retorno vem de alinhar código e negócio por linguagem e fronteira. Em CRUD ou relatório sem regra, o custo de modelar supera o ganho — não use.

## 2. Linguagem ubíqua

- Um vocabulário só, usado pelo negócio e pelo código, sem sinônimos escondidos: `Cliente`, não `Cliente` + `Customer` + `User` para a mesma coisa.
- Termo ambíguo entre áreas vira **dois nomes diferentes** em contextos diferentes: `Cliente` de Vendas ≠ `Cliente` de Suporte.
- Nome descreve intenção de domínio, não o padrão aplicado: `CalculadoraDeFrete`, não `FreteStrategy`.

Teste rápido: um especialista lê um trecho do código e entende sem tradução. Se precisa de dicionário, a linguagem ainda não é ubíqua.

## 3. Subdomínios

| Tipo | Investimento | Exemplo |
|---|---|---|
| Core | melhor modelagem, time sênior | precificação, risco de crédito |
| Supporting | modelagem suficiente | cadastro interno específico |
| Generic | comprar ou reusar | autenticação, e-mail, geração de PDF |

Tratar autenticação como core e gastar modelagem onde não há vantagem competitiva é o erro simétrico de ignorar o core.

## 4. Bounded context

Fronteira onde **um** modelo e **uma** linguagem são válidos. Dentro, os termos têm significado único; fora, não.

- Um contexto idealmente tem um time e um deploy; fronteira sem dono degrada.
- Não compartilhe entidade entre contextos: integre por contrato (API/evento) com tradução.
- Tamanho saudável: o menor limite que mantém a linguagem consistente. Grande demais volta a ter ambiguidade; pequeno demais gera integração sem valor.

## 5. Context map

| Padrão | Relação | Custo |
|---|---|---|
| Partnership | times evoluem juntos, dependência mútua | agenda acoplada |
| Shared Kernel | pequeno modelo comum | quebra frequente |
| Customer/Supplier | montante atende jusante com prioridade negociada | negociação contínua |
| Conformist | jusante adota o modelo do montante sem traduzir | perde independência |
| Anti-Corruption Layer | jusante traduz o modelo externo | código de tradução |
| Open Host Service / Published Language | contrato estável e documentado | governança de versão |

Regra prática: quanto menos poder de negociação o time tem, mais barato é ACL do que Conformist — Conformist acopla o modelo interno ao externo.

## 6. Entity × Value Object

- **Entity**: identidade contínua, igualdade por ID (`Pedido`, `Conta`).
- **Value Object**: imutável, igualdade estrutural (`Dinheiro`, `Email`, `Periodo`).
- VO no lugar de primitivo remove validação repetida e erro de unidade (`Dinheiro` impede somar centavos com reais).

Go — VO como struct imutável com construtor validante:

```go
package domain

type Dinheiro struct {
	centavos int64
	moeda    string
}

func NovoDinheiro(centavos int64, moeda string) (Dinheiro, error) {
	if moeda == "" {
		return Dinheiro{}, ErrMoedaObrigatoria
	}
	return Dinheiro{centavos: centavos, moeda: moeda}, nil
}

func (d Dinheiro) Somar(o Dinheiro) (Dinheiro, error) {
	if o.moeda != d.moeda {
		return Dinheiro{}, ErrMoedaDivergente
	}
	return Dinheiro{centavos: d.centavos + o.centavos, moeda: d.moeda}, nil
}
```

TypeScript — mesma regra, construtor privado e fábrica estática:

```ts
export class Dinheiro {
  private constructor(
    private readonly centavos: number,
    private readonly moeda: string,
  ) {}

  static de(centavos: number, moeda: string): Dinheiro {
    if (!Number.isInteger(centavos)) throw new Error('centavos inteiros')
    if (!moeda) throw new Error('moeda obrigatória')
    return new Dinheiro(centavos, moeda)
  }

  somar(o: Dinheiro): Dinheiro {
    if (o.moeda !== this.moeda) throw new Error('moeda divergente')
    return new Dinheiro(this.centavos + o.centavos, this.moeda)
  }
}
```

## 7. Agregado e aggregate root

- **Agregado**: cluster tratado como unidade de consistência. A raiz é o único acesso externo e garante as invariantes.
- **Uma transação altera um agregado.** Entre agregados, consistência eventual por evento.
- Referencie outro agregado **por ID**, não por objeto: evita carregar o grafo inteiro e travar duas raízes.
- Agregado pequeno reduz conflito de concorrência; se duas regras precisam ser atômicas, provavelmente são o mesmo agregado.

Go — raiz controlando invariantes e emitindo evento:

```go
type Pedido struct {
	id      PedidoID
	itens   []Item
	status  Status
	eventos []Evento
}

func NovoPedido(id PedidoID, itens []Item) (*Pedido, error) {
	if len(itens) == 0 {
		return nil, ErrPedidoSemItens
	}
	p := &Pedido{id: id, itens: append([]Item(nil), itens...), status: Rascunho}
	p.eventos = append(p.eventos, PedidoCriado{ID: id})
	return p, nil
}

func (p *Pedido) AdicionarItem(it Item) error {
	if p.status != Rascunho {
		return ErrPedidoFechado
	}
	if it.Quantidade <= 0 {
		return ErrQuantidadeInvalida
	}
	for _, atual := range p.itens {
		if atual.ProdutoID == it.ProdutoID {
			return ErrProdutoDuplicado
		}
	}
	p.itens = append(p.itens, it)
	return nil
}
```

TypeScript — mesma invariante, coleção encapsulada:

```ts
export class Pedido {
  private readonly itens: Item[] = []
  private status: Status = 'rascunho'

  private constructor(readonly id: PedidoId) {}

  static criar(id: PedidoId): Pedido {
    return new Pedido(id)
  }

  adicionarItem(it: Item): void {
    if (this.status !== 'rascunho') throw new Error('pedido fechado')
    if (it.quantidade <= 0) throw new Error('quantidade inválida')
    if (this.itens.some((a) => a.produtoId === it.produtoId)) {
      throw new Error('produto duplicado')
    }
    this.itens.push(it)
  }
}
```

## 8. Domain event

Fato do negócio no passado (`PedidoConfirmado`, `PagamentoRecusado`). Nomeie no passado, carregue dados suficientes para o consumidor não voltar ao produtor, versione o schema. Uso: integrar contextos e disparar efeito colateral sem acoplar agregados.

## 9. Domain service

Operação de negócio que não pertence a uma entidade ou VO (ex.: transferência entre duas contas, com duas raízes). Parcimônia: serviço que concentra todas as regras é sintoma de modelo anêmico.

## 10. Repository e Factory

- **Repository**: coleção de agregados por identidade. Não é DAO por tabela; devolve agregado inteiro ou nada.
- **Factory**: encapsula criação complexa garantindo objeto válido. Reconstituição a partir do banco pode usar caminho próprio, documentado.

## 11. Specification / Policy

Regra combinável e nomeada (`ClientePodeParcelar`, `PedidoIsentoDeFrete`), testável isoladamente. Vale quando o critério varia por contexto e é reutilizado; um `if` simples não precisa de Specification.

## 12. Consistência entre agregados

- Dentro: transação imediata (ACID local).
- Entre: evento + consumidor idempotente + compensação. Não abra transação distribuída para juntar agregados — idempotência é requisito, não detalhe.

## 13. Erros comuns

- Modelo anêmico: dados na classe, regra num "service" gigante.
- Entidade ou tabela compartilhada entre bounded contexts.
- Agregado deus com dezenas de entidades e trava de concorrência.
- VO mutável tratado como Entity (ou o inverso).
- SQL/HTTP vazando para dentro do domínio.
- DDD aplicado a CRUD: custo alto, benefício nulo.

## 14. Checklist

- [ ] Linguagem ubíqua no código, sem sinônimos.
- [ ] Bounded contexts com fronteira e dono explícitos.
- [ ] Cada regra com um único responsável (agregado/VO/serviço).
- [ ] Agregados pequenos, raiz única, referência por ID.
- [ ] Transação respeita o limite do agregado; integração por evento/ACL.
- [ ] VOs substituíram primitivos de domínio relevantes.

## Fontes

- Evans, *Domain-Driven Design* (2003): design estratégico (bounded context, context map, distillation) e ciclo de vida do objeto de domínio (agregado, factory, repository). Paráfrase; nenhuma transcrição.
- Vernon, *Implementing Domain-Driven Design* (2013): regras de projeto de agregados (cap. 10).
- Consolidação local: corpo integrado a este arquivo em 2026-11-16 (skill de DDD absorvida nesta consolidação; conteúdo preservado).
- Referências canônicas usadas na §4-5: Cockburn, Hexagonal Architecture — https://alistair.cockburn.us/hexagonal-architecture/ · Martin, Clean Architecture — https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html (URLs não re-fetchadas nesta sessão).
