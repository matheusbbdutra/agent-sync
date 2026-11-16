# Architectural Styles

> Absorbed by architecture (2026-11-16). Original content preserved; end-to-end examples ported to Go and TypeScript.

Reference loaded by `../SKILL.md` §2. Domain modeling (aggregate, VO, event, repository) stays in [domain-modeling.md](domain-modeling.md); here the subject is **where the code lives and who may import whom**.

## 1. Clean Architecture

Layers, with the dependency pointing inward:

| Layer | Content | May import |
|---|---|---|
| Entities | business rule that does not change with the app | nothing from outside |
| Use Cases | orchestration of the application rule | entities |
| Interface Adapters | controllers, presenters, gateways | use cases |
| Frameworks & Drivers | UI, database, external services | everything above |

Principles:

- The dependency points inward; an internal layer does not know an external layer.
- Business rule independent of framework.
- Testable without UI, database or external service.

## 2. Hexagonal (ports and adapters)

- **Core**: business rule.
- **Ports**: interfaces that declare the interaction.
- **Adapters**: implementation of the port (database, HTTP, queue).

Gains: changing implementation becomes changing the adapter, a core agnostic of technology, explicit separation of responsibility.

**Golden rule:** the interface belongs to the **core**, not to the adapter. A port declared in the infrastructure package is just coupling with a pretty name.

## 3. Directory structure

Go — `internal/` prevents external import and makes the dependency rule verifiable by `go list`:

```
cmd/api/main.go                    # composition root: monta adaptadores e injeta
internal/domain/                   # entidades, VOs, eventos, erros de domínio
  usuario.go  pedido.go  dinheiro.go  email.go
internal/app/                      # casos de uso + declaração das portas
  porta_repositorio.go  criar_usuario.go  confirmar_pedido.go
internal/adapters/postgres/        # adaptador de repositório
  usuario_repo.go
internal/adapters/http/            # adaptador de entrada
  usuario_handler.go  rota.go
internal/adapters/stripe/          # adaptador de gateway
  gateway.go
```

TypeScript — same names, `domain/` with no `npm` import:

```
src/domain/usuario.ts  pedido.ts  dinheiro.ts  email.ts
src/application/portas.ts  criar-usuario.ts  confirmar-pedido.ts
src/infrastructure/postgres/repositorio-usuario.ts
src/infrastructure/stripe/gateway.ts
src/http/usuario-rota.ts
src/main.ts                        # composition root
```

A sign that the structure is decorative: `app/` importing `adapters/`, `domain/` importing `pg`/`http`, or a `service/` that imports every package.

## 4. End-to-end example — Go

Entity with behavior, no framework:

```go
// internal/domain/usuario.go
package domain

type UsuarioID string

type Usuario struct {
	ID    UsuarioID
	Email Email
	Nome  string
	ativo bool
}

func NovoUsuario(id UsuarioID, email Email, nome string) (*Usuario, error) {
	if nome == "" {
		return nil, ErrNomeObrigatorio
	}
	return &Usuario{ID: id, Email: email, Nome: nome, ativo: true}, nil
}

func (u *Usuario) Desativar() { u.ativo = false }

func (u Usuario) Ativo() bool { return u.ativo }

func (u Usuario) PodeFazerPedido() bool { return u.ativo }
```

Port declared by the core (`app`), without knowing SQL:

```go
// internal/app/porta_repositorio.go
package app

type RepositorioUsuario interface {
	PorID(ctx context.Context, id domain.UsuarioID) (*domain.Usuario, error)
	PorEmail(ctx context.Context, email domain.Email) (*domain.Usuario, error)
	Salvar(ctx context.Context, u *domain.Usuario) (*domain.Usuario, error)
	Remover(ctx context.Context, id domain.UsuarioID) (bool, error)
}
```

Use case: orchestrates, validates duplication, delegates persistence:

```go
// internal/app/criar_usuario.go
package app

type CriarUsuarioEntrada struct {
	Email string
	Nome  string
}

type CriarUsuario struct {
	usuarios RepositorioUsuario
	ids      GeradorID
}

func (c CriarUsuario) Executar(ctx context.Context, in CriarUsuarioEntrada) (*domain.Usuario, error) {
	email, err := domain.NovoEmail(in.Email)
	if err != nil {
		return nil, err
	}
	existente, err := c.usuarios.PorEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNaoEncontrado) {
		return nil, fmt.Errorf("consultar e-mail: %w", err)
	}
	if existente != nil {
		return nil, ErrEmailJaExiste
	}
	novo, err := domain.NovoUsuario(domain.UsuarioID(c.ids.Novo()), email, in.Nome)
	if err != nil {
		return nil, err
	}
	salvo, err := c.usuarios.Salvar(ctx, novo)
	if err != nil {
		return nil, fmt.Errorf("salvar usuário: %w", err)
	}
	return salvo, nil
}
```

Output adapter: SQL lives here and nowhere else:

```go
// internal/adapters/postgres/usuario_repo.go
package postgres

type UsuarioRepo struct{ db *sql.DB }

func (r UsuarioRepo) PorEmail(ctx context.Context, email domain.Email) (*domain.Usuario, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, email, nome, ativo FROM usuarios WHERE email = $1`, email.String())
	return scanUsuario(row)
}

func (r UsuarioRepo) Salvar(ctx context.Context, u *domain.Usuario) (*domain.Usuario, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO usuarios (id, email, nome, ativo) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		SET email = excluded.email, nome = excluded.nome, ativo = excluded.ativo`,
		string(u.ID), u.Email.String(), u.Nome, u.Ativo())
	if err != nil {
		return nil, fmt.Errorf("upsert usuário %s: %w", u.ID, err)
	}
	return u, nil
}
```

Input adapter: thin controller, it only translates HTTP ↔ use case:

```go
// internal/adapters/http/usuario_handler.go
type criarUsuarioDTO struct {
	Email string `json:"email"`
	Nome  string `json:"nome"`
}

func CriarUsuarioHandler(criar app.CriarUsuario) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var dto criarUsuarioDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
			http.Error(w, "corpo inválido", http.StatusBadRequest)
			return
		}
		usuario, err := criar.Executar(r.Context(), app.CriarUsuarioEntrada{Email: dto.Email, Nome: dto.Nome})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(usuario)
	}
}
```

## 5. End-to-end example — TypeScript

```ts
// src/domain/usuario.ts
export class Usuario {
  private constructor(
    readonly id: UsuarioId,
    readonly email: Email,
    readonly nome: string,
    private ativo = true,
  ) {}

  static criar(id: UsuarioId, email: Email, nome: string): Usuario {
    if (!nome) throw new Error('nome obrigatório')
    return new Usuario(id, email, nome)
  }

  desativar(): void {
    this.ativo = false
  }

  podeFazerPedido(): boolean {
    return this.ativo
  }
}
```

```ts
// src/application/portas.ts
export interface RepositorioUsuario {
  porId(id: UsuarioId): Promise<Usuario | null>
  porEmail(email: Email): Promise<Usuario | null>
  salvar(u: Usuario): Promise<Usuario>
  remover(id: UsuarioId): Promise<boolean>
}
```

```ts
// src/application/criar-usuario.ts
export class CriarUsuario {
  constructor(private readonly usuarios: RepositorioUsuario) {}

  async executar(entrada: { email: string; nome: string }): Promise<Usuario> {
    const email = Email.de(entrada.email)
    if (await this.usuarios.porEmail(email)) throw new Error('e-mail já existe')
    const novo = Usuario.criar(UsuarioId.novo(), email, entrada.nome)
    return this.usuarios.salvar(novo)
  }
}
```

```ts
// src/infrastructure/postgres/repositorio-usuario.ts
export class RepositorioUsuarioPg implements RepositorioUsuario {
  constructor(private readonly pool: Pool) {}

  async porEmail(email: Email): Promise<Usuario | null> {
    const { rows } = await this.pool.query(
      'SELECT id, email, nome, ativo FROM usuarios WHERE email = $1',
      [email.toString()],
    )
    return rows.length === 0 ? null : this.paraEntidade(rows[0])
  }

  async salvar(u: Usuario): Promise<Usuario> {
    await this.pool.query(
      `INSERT INTO usuarios (id, email, nome, ativo) VALUES ($1, $2, $3, $4)
       ON CONFLICT (id) DO UPDATE SET email = $2, nome = $3, ativo = $4`,
      [u.id.toString(), u.email.toString(), u.nome, u.podeFazerPedido()],
    )
    return u
  }

  private paraEntidade(row: UsuarioRow): Usuario {
    return Usuario.criar(UsuarioId.de(row.id), Email.de(row.email), row.nome)
  }
}
```

```ts
// src/http/usuario-rota.ts
export function rotaCriarUsuario(criar: CriarUsuario): Router {
  const router = Router()
  router.post('/usuarios', async (req, res) => {
    try {
      const usuario = await criar.executar({ email: req.body.email, nome: req.body.nome })
      res.status(201).json(usuario)
    } catch (err) {
      res.status(400).json({ erro: (err as Error).message })
    }
  })
  return router
}
```

## 6. Ports, adapters and test fake

A domain service receiving the three ports — no infrastructure import. The repository adapter persists the aggregate **and only then** publishes the accumulated events, clearing the list afterwards; publishing before the commit leaves the consumer reacting to a fact that may not have been recorded.

```go
// Portas: o núcleo declara, a infraestrutura implementa.
type RepositorioPedido interface {
	PorID(ctx context.Context, id domain.PedidoID) (*domain.Pedido, error)
	Salvar(ctx context.Context, p *domain.Pedido) error
}

type GatewayPagamento interface {
	Cobrar(ctx context.Context, valor domain.Dinheiro, clienteID string) (Pagamento, error)
}

type Notificador interface {
	Enviar(ctx context.Context, destino, assunto, corpo string) error
}

type ServicoPedido struct {
	pedidos   RepositorioPedido
	pagamento GatewayPagamento
	notificar Notificador
}

func (s ServicoPedido) Confirmar(ctx context.Context, id domain.PedidoID) error {
	p, err := s.pedidos.PorID(ctx, id)
	if err != nil {
		return fmt.Errorf("carregar pedido %s: %w", id, err)
	}
	pag, err := s.pagamento.Cobrar(ctx, p.Total(), p.ClienteID())
	if err != nil {
		return fmt.Errorf("cobrar pedido %s: %w", id, err)
	}
	if err := p.MarcarPago(pag.ID); err != nil {
		return err
	}
	if err := s.pedidos.Salvar(ctx, p); err != nil {
		return fmt.Errorf("salvar pedido %s: %w", id, err)
	}
	return s.notificar.Enviar(ctx, p.EmailCliente(), "Pedido confirmado", id.String())
}
```

Real adapter (Stripe) — translates the vendor into the domain type and classifies the error:

```go
// internal/adapters/stripe/gateway.go
type Gateway struct {
	chave string
	http  *http.Client
}

func (g Gateway) Cobrar(ctx context.Context, valor domain.Dinheiro, clienteID string) (app.Pagamento, error) {
	corpo := url.Values{
		"amount":   {strconv.FormatInt(valor.Centavos(), 10)},
		"currency": {valor.Moeda()},
		"customer": {clienteID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.stripe.com/v1/charges", strings.NewReader(corpo.Encode()))
	if err != nil {
		return app.Pagamento{}, fmt.Errorf("montar cobrança: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.chave)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.http.Do(req)
	if err != nil {
		return app.Pagamento{}, fmt.Errorf("cobrar cliente %s: %w", clienteID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return app.Pagamento{}, fmt.Errorf("stripe status %d: %w", resp.StatusCode, app.ErrPagamentoRecusado)
	}
	var cobranca struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cobranca); err != nil {
		return app.Pagamento{}, fmt.Errorf("decodificar cobrança: %w", err)
	}
	return app.Pagamento{ID: cobranca.ID, Aprovado: true}, nil
}
```

Test fake in place of a mock framework — same contract, zero network:

```go
// internal/app/confirmar_pedido_test.go
type gatewayFake struct {
	aprovado bool
	chamadas []domain.Dinheiro
}

func (f *gatewayFake) Cobrar(_ context.Context, valor domain.Dinheiro, _ string) (Pagamento, error) {
	f.chamadas = append(f.chamadas, valor)
	if !f.aprovado {
		return Pagamento{}, ErrPagamentoRecusado
	}
	return Pagamento{ID: "fake-123", Aprovado: true}, nil
}
```

Same design in TypeScript — constructor injection, fake in the test:

```ts
export interface GatewayPagamento {
  cobrar(valor: Dinheiro, clienteId: string): Promise<Pagamento>
}

export class ServicoPedido {
  constructor(
    private readonly pedidos: RepositorioPedido,
    private readonly pagamento: GatewayPagamento,
    private readonly notificar: Notificador,
  ) {}

  async confirmar(id: PedidoId): Promise<void> {
    const p = await this.pedidos.porId(id)
    if (!p) throw new Error(`pedido ${id} não encontrado`)
    const pag = await this.pagamento.cobrar(p.total(), p.clienteId)
    p.marcarPago(pag.id)
    await this.pedidos.salvar(p)
    await this.notificar.enviar(p.emailCliente, 'Pedido confirmado', String(id))
  }
}

// teste
class GatewayFake implements GatewayPagamento {
  constructor(private readonly aprovado = true) {}
  async cobrar(valor: Dinheiro): Promise<Pagamento> {
    if (!this.aprovado) throw new Error('pagamento recusado')
    return { id: 'fake-123', valor }
  }
}
```

## 7. Composition root and test

Only `main` knows the concrete implementation (database, gateway, repository); the rest receives an interface. The use case runs with an in-memory repository — if you had to bring up a database to test the business rule, the dependency points to the wrong side.

## 8. Anti-patterns of this axis

- A `service/` layer that imports `adapters/` — inverted dependency rule.
- A domain entity with an ORM tag or `serializer` — the domain knows persistence.
- Controller with business rule (invariant validation, total calculation).
- A port with an implementation that never varies and no test using the double.
- "Hexagonal" with the interface declared inside the adapter.
- Repository returning a table row or an ORM object.
