---
name: ddd
description: "Use when modeling a complex business domain that needs ubiquitous language and bounded contexts. Domain-Driven Design estratégico e tático: linguagem ubíqua, bounded contexts, entidades e agregados."
---

# Domain-Driven Design (DDD)

DDD é uma abordagem para **domínios complexos**: o valor está em alinhar código e negócio por meio da linguagem e de fronteiras bem definidas. Para CRUD simples, DDD agrega custo sem retorno — nesse caso, não use.

## Use this skill when

- O domínio tem regras e exceções de negócio não triviais.
- É preciso definir fronteiras de módulos, serviços ou times.
- Há ambiguidade de termos entre áreas (o mesmo nome com significados diferentes).
- Espera-se evoluir o modelo junto com o negócio por anos.

## Do not use this skill when

- O sistema é majoritariamente CRUD/relatórios sem regra relevante.
- Não há acesso a especialistas de domínio nem linguagem a descobrir.
- A equipe é pequena e o custo das fronteiras supera o benefício.

## Instructions

1. **Descoberta:** converse com especialistas e escreva a linguagem ubíqua; resolva ambiguidades de termos.
2. **Estratégico:** divida o negócio em subdomínios e defina bounded contexts com limites explícitos.
3. **Mapa:** documente como os contextos se relacionam (context map).
4. **Tático:** modele agregados em torno de invariantes; use Value Objects para conceitos imutáveis.
5. **Verificação:** cada regra de negócio deve ter um único dono no modelo.

## Nível estratégico

### Linguagem ubíqua

- Vocabulário único, compartilhado por negócio e código.
- Se um termo é ambíguo, **dê nomes diferentes** em contextos diferentes (ex.: `Cliente` de Vendas ≠ `Cliente` de Suporte).
- O código deve usar exatamente esses nomes (sem sinônimos escondidos).

### Subdomínios

- **Core:** onde está a vantagem competitiva; invista a melhor modelagem.
- **Supporting:** necessário, específico, mas não diferencial.
- **Generic:** resolvido por compra/reuso (ex.: autenticação, e-mail).

### Bounded Context

- Fronteira onde um modelo e uma linguagem ubíqua são válidos.
- Um contexto idealmente pertence a um time e a um deploy.
- Não compartilhe entidades entre contextos; comunique-se por contratos/eventos.

### Context Map (padrões de relação)

- **Partnership:** times evoluem juntos, com dependência mútua.
- **Shared Kernel:** pequeno modelo compartilhado (cuidado, gera acoplamento).
- **Customer/Supplier:** a montante atende a jusante com prioridade negociada.
- **Conformist:** a jusante adota o modelo a montante sem traduzir.
- **Anti-Corruption Layer (ACL):** traduza o modelo externo para proteger o seu.
- **Open Host Service / Published Language:** exponha um contrato estável e documentado.

## Nível tático

### Entity × Value Object

- **Entity:** possui identidade e ciclo de vida (`Pedido`, `Conta`). Igualdade por ID.
- **Value Object:** imutável, sem identidade, igualdade por valor (`Dinheiro`, `Email`, `Periodo`). Substitua primitivos de domínio por VOs.

### Aggregate e Aggregate Root

- **Agregado:** cluster de objetos tratado como unidade de consistência.
- A **raiz** é o único ponto de acesso e garante as **invariantes** em toda alteração.
- **Regra:** uma transação altera **um** agregado. Entre agregados, use consistência eventual (eventos).
- Agregados pequenos reduzem conflitos de concorrência.

```python
class Pedido:  # Aggregate Root
    def __init__(self, id, itens):
        self._id = id
        self._itens = Itens(itens)
        if self._itens.quantidade() == 0:
            raise ValueError("pedido sem itens")

    def adicionar_item(self, item):
        if self._itens.possui(item.produto_id):
            raise ValueError("produto duplicado")
        self._itens = self._itens.adicionar(item)
```

### Domain Event

- Fato relevante do negócio no passado (`PedidoConfirmado`, `PagamentoRecusado`).
- Usado para integrar contextos e disparar efeitos colaterais sem acoplar agregados.
- Nomeie no passado e carregue dados suficientes para o consumidor.

### Domain Service

- Operação de negócio que não pertence naturalmente a uma entidade/VO.
- Use com parcimônia; excesso de serviços indica modelo anêmico.

### Repository e Factory

- **Repository:** acesso a agregados por identidade, coleção de domínio (não um DAO por tabela).
- **Factory:** encapsula a criação complexa de agregados e VOs válidos.

### Specification / Policy

- Regras de negócio combináveis e nomeadas (`ClientePodeParcelar`, `PedidoIsentoDeFrete`).
- Úteis quando critérios variam por contexto e precisam ser reutilizados/testados.

## Consistência e transações

- Invariantes **dentro** do agregado: transação única e imediata.
- Invariantes **entre** agregados: consistência eventual, via eventos e política de compensação.
- Se duas regras precisam ser atômicas, provavelmente pertencem ao mesmo agregado.

## Erros comuns

- Modelo anêmico: dados numa classe, regras num "service" gigante.
- Compartilhar entidades/tabelas entre bounded contexts.
- Criar um "agregado deus" com dezenas de entidades e travas de concorrência.
- Usar DDD em CRUD simples (custo alto, benefício nulo).
- Confundir Value Object mutável com Entity.
- Vazamento da linguagem técnica (SQL/HTTP) para dentro do domínio.

## Checklist

- [ ] A linguagem ubíqua está no código, sem sinônimos?
- [ ] As fronteiras dos bounded contexts estão claras e com donos?
- [ ] Cada regra tem um único responsável (agregado/VO/serviço)?
- [ ] Agregados são pequenos e com raiz única de acesso?
- [ ] Transações respeitam o limite do agregado?
- [ ] Integrações usam eventos/ACL, sem compartilhamento de tabelas?
- [ ] Value Objects substituíram primitivos de domínio relevantes?
