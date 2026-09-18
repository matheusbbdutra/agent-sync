---
name: design-patterns
description: "Catálogo de padrões de projeto GoF aplicados pragmaticamente para evitar duplicação."
---

# Padrões de Projeto (GoF) e de Domínio

Um padrão é **vocabulário compartilhado**, não uma meta. Aplique só quando houver uma dor concreta (variação, acoplamento ou duplicação). A regra de ouro: **composição sobre herança** e **depender de abstrações, não de implementações**.

## Use this skill when

- Projetar componentes e decidir como desacoplar responsabilidades.
- Remover cadeias longas de `if/else`/`switch` que crescem a cada novo caso.
- Isolar código de terceiros (bancos, SDKs, HTTP) atrás de uma interface própria.
- Introduzir pontos de extensão sem alterar código existente (Open/Closed).
- Modelar persistência e regras de domínio com Repository/Specification.

## Do not use this skill when

- O problema é um ajuste pontual e localizado.
- A abstração existiria por apenas um caso de uso (sem variação real).
- O time não tem como testar a indireção adicionada.

## Instructions

1. Identifique o **eixo de variação**: o que muda com frequência e o que permanece estável?
2. Prefira a solução mais simples que isole esse eixo; só então escolha o padrão.
3. Nomeie classes pelas intenções do domínio, não pelo padrão (`CalculadoraDeFrete`, não `FreteStrategy`).
4. Valide com testes: o padrão deve reduzir o custo de adicionar o próximo caso.
5. Se um padrão não elimina um `if`/acoplamento nem melhora o teste, **não o aplique**.

## Criacionais

### Factory Method / Abstract Factory

- **Problema:** o cliente precisa criar objetos sem conhecer as classes concretas.
- **Quando usar:** há famílias de objetos relacionados ou a criação tem regras/lookup.
- **Evitar:** quando `new` direto já resolve e a classe é estável.
- **Sinal no código:** `switch` de criação espalhado por vários pontos.

### Builder

- **Problema:** construtores com muitos parâmetros opcionais ou construção em etapas.
- **Quando usar:** objetos complexos, imutáveis, montados passo a passo.
- **Evitar:** objetos com 2–3 campos simples.

## Estruturais

### Adapter

- **Problema:** integrar uma API de terceiro sem contaminar o domínio com o formato dela.
- **Quando usar:** SDKs, gateways, clientes HTTP, bibliotecas legadas.
- **Sinal:** tipos do fornecedor vazando para as regras de negócio.

### Decorator

- **Problema:** adicionar comportamentos ortogonais (log, retry, cache, métricas) sem alterar a classe.
- **Quando usar:** pilha de responsabilidades que varia por contexto.
- **Evitar:** quando herança ou injeção única bastam.

### Facade

- **Problema:** subsistema complexo exposto a muitos clientes.
- **Quando usar:** simplificar um ponto de entrada ou isolar um módulo.
- **Evitar:** "Deus-fachada" que só repassa chamadas.

## Comportamentais

### Strategy

- **Problema:** variantes de um algoritmo escolhidas em runtime (frete, desconto, imposto).
- **Quando usar:** o mesmo contrato com implementações intercambiáveis.
- **Evitar:** quando há apenas uma variante ou a escolha é fixa em build.
- **Sinal:** `if tipo == A: ... elif tipo == B: ...` crescendo.

### Observer / Publish-Subscribe

- **Problema:** notificar múltiplos interessados quando algo muda.
- **Quando usar:** eventos de domínio, desacoplamento de efeitos colaterais.
- **Evitar:** quando o fluxo síncrono e explícito é mais fácil de depurar.

### Command

- **Problema:** encapsular uma ação como objeto (fila, undo/redo, auditoria).
- **Quando usar:** operações precisam ser parametrizadas, enfileiradas ou registradas.
- **Evitar:** chamadas diretas simples.

### Template Method

- **Problema:** esqueleto fixo com passos variáveis.
- **Quando usar:** quando herança é aceitável e o esqueleto é realmente estável.
- **Evitar:** prefira Strategy + composição quando as variantes se multiplicam.

## Padrões de domínio e infraestrutura

### Repository

- **Problema:** desacoplar o domínio do mecanismo de persistência, expondo coleções.
- **Quando usar:** agregados com leitura/escrita por identidade.
- **Evitar:** um repositório genérico por tabela; modele por agregado.
- **Sinal:** SQL/ORM invadindo casos de uso.

### Unit of Work

- **Problema:** coordenar alterações de vários repositórios em uma transação.
- **Quando usar:** múltiplos agregados alterados na mesma operação.
- **Evitar:** operações de um único agregado (transação local já resolve).

### Specification

- **Problema:** regras de filtro/validação combináveis e reutilizáveis.
- **Quando usar:** critérios compostos (E/OU/NÃO) que variam por caso de uso.
- **Evitar:** quando um predicado simples basta.

## Checklist anti-over-engineering

- [ ] Existe variação real (mais de um caso concreto)?
- [ ] O padrão remove acoplamento ou um `if` que cresceria?
- [ ] Dá para testar a unidade isoladamente depois da mudança?
- [ ] O nome revela intenção de domínio (não o nome do padrão)?
- [ ] Adicionar o próximo caso ficou mais barato?

Se duas ou mais respostas forem "não", **não aplique** o padrão.
