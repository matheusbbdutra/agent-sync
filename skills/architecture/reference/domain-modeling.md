# Domain Modeling (DDD)

Reference loaded by `../SKILL.md` §1. Covers the strategic level (boundaries, language) and the tactical one (aggregates, VOs, events). The order follows Evans: strategic design (Part IV) and domain object lifecycle (ch. 6).

## 1. When the model earns money

DDD is a tool for complex domains: the return comes from aligning code and business through language and boundary. In CRUD or a report without rules, the cost of modeling exceeds the gain — do not use it.

## 2. Ubiquitous language

- A single vocabulary, used by the business and by the code, with no hidden synonyms: `Cliente`, not `Cliente` + `Customer` + `User` for the same thing.
- An ambiguous term between areas becomes **two different names** in different contexts: Sales' `Cliente` ≠ Support's `Cliente`.
- The name describes domain intent, not the applied pattern: `CalculadoraDeFrete`, not `FreteStrategy`.

Quick test: a specialist reads a stretch of the code and understands it without translation. If a dictionary is needed, the language is not ubiquitous yet.

## 3. Subdomains

| Type | Investment | Example |
|---|---|---|
| Core | best modeling, senior team | pricing, credit risk |
| Supporting | sufficient modeling | specific internal registry |
| Generic | buy or reuse | authentication, e-mail, PDF generation |

Treating authentication as core and spending modeling where there is no competitive advantage is the symmetrical error of ignoring the core.

## 4. Bounded context

The boundary where **one** model and **one** language are valid. Inside, the terms have a single meaning; outside, they do not.

- A context ideally has one team and one deploy; a boundary without an owner degrades.
- Do not share an entity between contexts: integrate by contract (API/event) with translation.
- Healthy size: the smallest limit that keeps the language consistent. Too large and it has ambiguity again; too small and it generates integration with no value.

## 5. Context map

| Pattern | Relation | Cost |
|---|---|---|
| Partnership | teams evolve together, mutual dependency | coupled schedule |
| Shared Kernel | small common model | frequent breakage |
| Customer/Supplier | upstream serves downstream with negotiated priority | continuous negotiation |
| Conformist | downstream adopts the upstream model without translating | loses independence |
| Anti-Corruption Layer | downstream translates the external model | translation code |
| Open Host Service / Published Language | stable and documented contract | version governance |

Practical rule: the less negotiation power the team has, the cheaper an ACL is than Conformist — Conformist couples the internal model to the external one.

## 6. Entity × Value Object

- **Entity**: continuous identity, equality by ID (`Pedido`, `Conta`).
- **Value Object**: immutable, structural equality (`Dinheiro`, `Email`, `Periodo`).
- A VO in place of a primitive removes repeated validation and unit errors (`Dinheiro` prevents adding cents to reais).

Go — VO as an immutable struct with a validating constructor:

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

TypeScript — same rule, private constructor and static factory:

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

## 7. Aggregate and aggregate root

- **Aggregate**: a cluster treated as a unit of consistency. The root is the only external access and guarantees the invariants.
- **One transaction changes one aggregate.** Between aggregates, eventual consistency by event.
- Reference another aggregate **by ID**, not by object: it avoids loading the entire graph and locking two roots.
- A small aggregate reduces concurrency conflict; if two rules need to be atomic, they are probably the same aggregate.

Go — root controlling the invariants and emitting an event:

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

TypeScript — same invariant, encapsulated collection:

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

A business fact in the past (`PedidoConfirmado`, `PagamentoRecusado`). Name it in the past, carry enough data so that the consumer does not go back to the producer, version the schema. Use: integrate contexts and trigger a side effect without coupling aggregates.

## 9. Domain service

A business operation that does not belong to an entity or VO (e.g.: transfer between two accounts, with two roots). Parsimony: a service that concentrates all the rules is a symptom of an anemic model.

## 10. Repository and Factory

- **Repository**: collection of aggregates by identity. It is not a DAO per table; it returns the entire aggregate or nothing.
- **Factory**: encapsulates complex creation guaranteeing a valid object. Reconstitution from the database may use its own documented path.

## 11. Specification / Policy

A combinable and named rule (`ClientePodeParcelar`, `PedidoIsentoDeFrete`), testable in isolation. It is worth it when the criterion varies by context and is reused; a simple `if` does not need a Specification.

## 12. Consistency between aggregates

- Inside: immediate transaction (local ACID).
- Between: event + idempotent consumer + compensation. Do not open a distributed transaction to join aggregates — idempotency is a requirement, not a detail.

## 13. Common errors

- Anemic model: data in the class, rule in a giant "service".
- Entity or table shared between bounded contexts.
- God aggregate with dozens of entities and a concurrency lock.
- Mutable VO treated as an Entity (or the reverse).
- SQL/HTTP leaking into the domain.
- DDD applied to CRUD: high cost, null benefit.

## 14. Checklist

- [ ] Ubiquitous language in the code, with no synonyms.
- [ ] Bounded contexts with explicit boundary and owner.
- [ ] Each rule with a single owner (aggregate/VO/service).
- [ ] Small aggregates, single root, reference by ID.
- [ ] Transaction respects the aggregate boundary; integration by event/ACL.
- [ ] VOs replaced relevant domain primitives.

## Sources

- Evans, *Domain-Driven Design* (2003): strategic design (bounded context, context map, distillation) and domain object lifecycle (aggregate, factory, repository). Paraphrase; no transcription.
- Vernon, *Implementing Domain-Driven Design* (2013): aggregate design rules (ch. 10).
- Local consolidation: body integrated into this file on 2026-11-16 (DDD skill absorbed in this consolidation; content preserved).
- Canonical references used in §4-5: Cockburn, Hexagonal Architecture — https://alistair.cockburn.us/hexagonal-architecture/ · Martin, Clean Architecture — https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html (URLs not re-fetched in this session).
