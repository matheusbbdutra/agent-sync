---
name: doctrine
description: Especialista no Doctrine ORM (PHP) cobrindo mapeamento por atributos, Identity Map/Unit of Work, repositórios, DQL/QueryBuilder parametrizado, migrations, herança, embeddables e otimização (N+1, fetch join, batch). Use ao modelar entidades, escrever consultas, criar migrations ou depurar performance/N+1 no Symfony.
---

# Doctrine ORM

Você é um engenheiro especialista em Doctrine. Modele o domínio e controle explicitamente a persistência, evitando surpresas de lazy loading e queries em loop.

## Use this skill when

- Mapear entidades, value objects, embeddables e relações.
- Escrever DQL/QueryBuilder, repositórios ou consultas otimizadas.
- Criar/revisar migrations e evolução de schema.
- Investigar lentidão, N+1, hydration ou consumo de memória.

## Do not use this skill when

- A camada de persistência não é Doctrine (ex.: DBAL puro, outro ORM).
- O problema é SQL/índice de banco independente de ORM → use `sql-optimization-patterns`/`postgresql`.
- É um CRUD trivial que `EntityManager`/repositório padrão já resolve bem.

## Instructions

1. Identifique a versão do Doctrine e o driver (config `doctrine.yaml`).
2. Mapeie por atributos PHP (`#[ORM\Entity]`, `#[ORM\Column]`); entidades com identidade.
3. Modele agregados: repositório por agregado, não uma entidade por tabela.
4. Sempre parametrize DQL/QueryBuilder — nunca concatene input.
5. Meça N+1 e memória antes de otimizar; use fetch join/paginação só onde é hot path.

## Mapeamento

- **Entidade:** classe com estado e comportamento; identidade por `#[ORM\Id]`/`#[ORM\GeneratedValue]`.
- **Value Object:** use `#[ORM\Embedded]`/`Embeddable` (ex.: `Dinheiro`, `Endereco`); evite `float` para dinheiro (use decimal ou inteiro de centavos).
- **Relações:** defina a owning side, `#[ORM\JoinColumn]`, `cascade` e `orphanRemoval` conscientemente.
- **Coleções:** encapsule em classes de coleção; não exponha `Collection` cru para o domínio.
- **Herança:** `Single Table` (simples, muitos nulos) ou `Joined` (normalizada, mais joins); evite modelar tipos por flags.

## Unit of Work e ciclo de vida

- O `EntityManager` mantém o **Identity Map**; entidades gerenciadas são persistidas no `flush()`.
- `persist()` agenda; `flush()` executa as mudanças em uma transação.
- Evite `flush()` dentro de loops: acumule e faça flush em lote (`$em->flush()` a cada N + `clear()`).
- Cuidado com entidades **detached** e `merge()` (evite; prefira recarregar/atualizar por DTO).
- Transações: `$em->wrapInTransaction()` para operações atômicas; mantenha a fronteira no caso de uso.

## Repositórios e consultas

- `ServiceEntityRepository` para repositórios custom; coloque consultas de domínio aqui.
- **DQL/QueryBuilder** com parâmetros (`->setParameter('id', $id)`); nunca interpole strings.
- Projete **DTOs** (`SELECT NEW`) quando não precisar de entidades gerenciadas.
- Use `getSingleResult`/`getOneOrNullResult` com cautela; `getResult` para listas.
- Hydration: `HYDRATE_OBJECT` (padrão) vs `HYDRATE_ARRAY`/`HYDRATE_SCALAR` (mais leve).

## Performance

- **N+1:** o sintoma clássico de lazy loading em loop. Resolva com `JOIN FETCH`, `addSelect` ou EAGER seletivo.
- **Bulk:** `IterableResult` + `clear()` para processar grandes volumes sem estourar memória.
- **Paginação:** sempre pagine; use Pagerfanta; cuidado com `JOIN FETCH` em coleções + paginação (use `Paginator` do Doctrine).
- **Índices:** índices para colunas de filtro/ordenação; confira o plano SQL.
- **Cache:** consultas/resultado e, se necessário, second-level cache — meça antes.

## Migrations

- `doctrine/migrations`: `make:migration`/`doctrine:migrations:diff` → revise o SQL gerado.
- Migrations idempotentes e versionadas no VCS; nunca altere migration já aplicada.
- Em produção: `migrations:migrate --no-interaction`; faça backup e planeje rollback.
- Cuidado com rename/drop destrutivo: faça em passos compatíveis (expand/contract).

## Segurança e consistência

- Nunca concatene input em DQL/SQL: use parâmetros (previne injeção).
- Use constraints de banco (unique, FK, not null) além da validação de aplicação.
- **Locking:** otimista (`#[ORM\Version]`) para conflitos de concorrência; pessimista (`PESSIMISTIC_WRITE`) só quando necessário.
- Valide invariantes no domínio; o banco é a última linha, não a única.

## Erros comuns

- Lazy loading em loops (N+1) dentro de templates/serialização.
- Deixar o EntityManager de longa vida vivo entre requests.
- `cascade: all` indiscriminado e `orphanRemoval` sem intenção.
- Dinheiro em `float`; datas como string.
- Expor entidades diretamente na API (acoplamento + lazy loading na serialização).

## Referências do repositório

- Integração com framework: `symfony`; testes: `phpunit-symfony`; SQL puro: `sql-optimization-patterns`.
