# Padrões de Projeto (GoF) e de Domínio

> Absorvida por architecture (2026-11-16). Conteúdo original preservado.

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

## Completando o catálogo GoF

Os 13 padrões que faltavam, no mesmo formato — para não escolher padrão por catálogo, e sim por sintoma.

### Criacionais restantes

#### Prototype

- **Problema:** criar variações de um objeto caro sem repetir a construção.
- **Quando usar:** configuração base com pequenas diferenças; cópia profunda com semântica definida.
- **Evitar:** quando um construtor com parâmetros explícitos é mais legível.
- **Custo:** `clone` raso é armadilha — referência compartilhada vira bug silencioso.

#### Singleton

- **Problema:** recurso único no processo (pool, cache, registro).
- **Quando usar:** quase nunca como padrão; prefira injetar uma instância única.
- **Evitar:** estado global disfarçado, dependência oculta, teste que não isola.
- **Sinal de erro:** código de teste precisa resetar o singleton entre casos.

### Estruturais restantes

#### Bridge

- **Problema:** duas dimensões variam de forma independente (abstração × implementação).
- **Quando usar:** evitar explosão de subclasses `m×n`.
- **Evitar:** quando uma das dimensões nunca varia.
- **Sinal:** classes do tipo `RelatorioPDFMensal`, `RelatorioHTMLMensal`, `RelatorioPDFAnual`…

#### Composite

- **Problema:** tratar folha e grupo da mesma forma (árvore de partes).
- **Quando usar:** hierarquia recursiva (menu, arquivo, expressão, organograma).
- **Evitar:** quando a estrutura é plana e estável.
- **Custo:** operações que só fazem sentido em folha ficam mal localizadas.

#### Flyweight

- **Problema:** milhares de objetos com estado intrínseco repetido consumindo memória.
- **Quando usar:** depois de **medir** consumo; cache de partes imutáveis compartilhadas.
- **Evitar:** otimização preventiva — é o padrão que mais aparece sem necessidade.
- **Sinal:** perfil de memória apontando o mesmo valor repetido.

#### Proxy

- **Problema:** interpor controle no acesso a um objeto (lazy, permissão, cache, remoto).
- **Quando usar:** o controle é ortogonal ao objeto e não deve morar nele.
- **Evitar:** quando Decorator já expressa o mesmo (Proxy controla, Decorator adiciona).
- **Custo:** indireção esconde latência; propague erro e timeout explicitamente.

### Comportamentais restantes

#### Chain of Responsibility

- **Problema:** vários tratadores possíveis, e o escolhido depende do conteúdo.
- **Quando usar:** pipeline de validação, middleware, aprovação por alçada.
- **Evitar:** quando a ordem é irrelevante ou há exatamente um tratador.
- **Custo:** cadeia mal configurada engole a requisição sem erro — termine com tratador que falha alto.

#### Mediator

- **Problema:** `n` objetos conversando entre si formam `n²` acoplamentos.
- **Quando usar:** coordenação central de interações complexas (UI, orquestração).
- **Evitar:** quando o mediador vira Deus-objeto que concentra toda a regra.
- **Sinal:** cada novo participante obriga mudar todos os outros.

#### Memento

- **Problema:** desfazer/restaurar estado sem expor os internos do objeto.
- **Quando usar:** undo, snapshot antes de operação arriscada, checkpoint.
- **Evitar:** estado grande ou frequente — custo de memória domina.
- **Custo:** snapshot profundo de agregado inteiro é caro; capture só o que a restauração exige.

#### State

- **Problema:** comportamento muda conforme o estado, e hoje isso é um `switch` gigante.
- **Quando usar:** transições explícitas e finitas (pedido, assinatura, job).
- **Evitar:** quando há 2 estados triviais — um enum e um `if` bastam.
- **Sinal:** `switch (status)` repetido em vários métodos da mesma classe.

#### Visitor

- **Problema:** adicionar operação nova a uma hierarquia fechada sem alterar as classes.
- **Quando usar:** estrutura estável, operações que crescem (relatório, validação, exportação).
- **Evitar:** quando a hierarquia muda com frequência — cada novo tipo quebra todos os visitors.
- **Custo:** `double dispatch` é difícil de ler; em Go, prefira type switch localizado.

#### Interpreter

- **Problema:** avaliar uma linguagem pequena recorrente (regra, filtro, expressão).
- **Quando usar:** gramática pequena e estável; Specification já não dá conta de composição.
- **Evitar:** quando um parser pronto (ou DSL existente) resolve sem escrever gramática.
- **Sinal de erro:** reimplementar parser de expressão à mão.

#### Iterator

- **Problema:** percorrer coleção sem expor sua estrutura interna.
- **Quando usar:** quase nunca explicitamente — a linguagem já oferece (`for range`, `range`-protocol).
- **Evitar:** criar tipo próprio quando slice/stream nativo resolve.
- **Sinal de valor:** só quando a coleção é preguiçosa, remota ou paginada.

## Seleção por sintoma

Use a coluna "primeiro tente" antes de aceitar a indireção do padrão.

| Sintoma no código | Primeiro tente | Padrão se persistir |
|---|---|---|
| `if/switch` crescendo por variante | parâmetro + função | Strategy / State |
| `switch` de criação espalhado | construtor explícito | Factory Method |
| Tipo de terceiro vazando no domínio | função de conversão | Adapter (+ ACL) |
| Pilha de log/retry/cache ao redor de chamada | injeção única | Decorator / Proxy |
| Viagem de requisição por N camadas de repasse | chamada direta | Facade (ou remover camada) |
| `m×n` subclasses por duas dimensões | uma dimensão só | Bridge |
| Árvore de partes com operações uniformes | recursão simples | Composite |
| Milhares de objetos com mesmo valor interno | medir memória | Flyweight |
| Permissão/lazy/remoto no acesso a objeto | checagem no chamador | Proxy |
| `n²` acoplamentos entre participantes | eventos simples | Mediator |
| Operação nova em hierarquia fechada | função livre | Visitor |
| Undo/restauração de estado | cópia explícita dos campos | Memento |
| Sequência de tratadores por conteúdo | `if` ordenado | Chain of Responsibility |
| Percorrer coleção paginada/remota | slice já carregado | Iterator |

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
