# Trade-offs de escala

Fonte: `donnemartin/system-design-primer` (CC BY 4.0). Reescrito; citação por seção.

Esta reference não ensina padrão novo — ela fixa **o custo de cada escolha** e o sintoma que indica
qual escolher. Não duplica: estilos arquiteturais ficam em `architectural-styles.md`,
características e fitness functions em `architecture-characteristics.md`, DDD em `domain-modeling.md`.

## Diagnóstico antes de escalar

| Pergunta | Resposta | O que significa |
|---|---|---|
| Lento para **um** usuário? | problema de **performance** | otimize caminho crítico, índice, cache |
| Rápido com 1 usuário e lento sob carga? | problema de **escalabilidade** | particione, replique, distribua |
| Tempo de uma ação alta? | problema de **latência** | aproximar dados/execução do ponto de uso |
| Ações por segundo baixas? | problema de **throughput** | paralelismo, batching, fila |

Definição operacional da fonte: um serviço é escalável se a performance cresce **proporcionalmente**
aos recursos adicionados; o objetivo é throughput máximo com latência aceitável
(`README.md#Performance vs scalability`, `README.md#Latency vs throughput`).

Corolário prático: aumentar recurso sem mudar a arquitetura melhora performance e não escala — e
trocar por escala sem medir latência produz vazão alta com usuário esperando.

## Disponibilidade em número

| Nível | Indisponibilidade/ano | /mês | /semana | /dia |
|---|---|---|---|---|
| 99,9% (3 nines) | 8h 45min 57s | 43m 49,7s | 10m 4,8s | 1m 26,4s |
| 99,99% (4 nines) | 52min 35,7s | 4m 23s | 1m 5s | 8,6s |

(`README.md#Availability in numbers`)

Composição:

- **Em série**: `A_total = A(a) × A(b)`. Dois componentes de 99,9% em série dão **99,8%** — a
  disponibilidade **cai**.
- **Em paralelo**: `A_total = 1 − (1 − A(a)) × (1 − A(b))`. Dois de 99,9% em paralelo dão **99,9999%**
  (`README.md#Availability in parallel vs in sequence`).

Consequência que quase sempre se ignora: cada dependência síncrona no caminho crítico **multiplica**
indisponibilidade. Redundância só compensa em paralelo, com failover que realmente troca.

## CAP: a escolha que não é opcional

Em sistema distribuído você sustenta duas das três garantias — consistência (toda leitura vê a
última escrita ou erro), disponibilidade (toda requisição responde, possivelmente desatualizada) e
tolerância a partição (`README.md#CAP theorem`).

A rede não é confiável, então tolerância a partição é premissa: **a decisão real é CP ou AP**.

| Escolha | Comportamento na partição | Quando serve |
|---|---|---|
| **CP** | nó particionado pode dar timeout | leitura/escrita atômica é requisito de negócio |
| **AP** | responde a versão mais disponível, possivelmente antiga | sistema precisa continuar e aceita consistência eventual |

(`README.md#CAP theorem`)

## Consistência

| Padrão | Depois da escrita | Onde aparece | Serve para |
|---|---|---|---|
| Fraca | leitura pode ou não ver | memcached | tempo real tolerante (voz, jogo) |
| Eventual | leitura vê depois (tipicamente ms) | DNS, e-mail | alta disponibilidade |
| Forte | leitura vê imediatamente (réplica síncrona) | FS, RDBMS | precisa de transação |

(`README.md#Consistency patterns`)

Erro comum: chamar de "eventual" o que é "fraca". Eventual promete convergência; fraca não promete
nada.

## Failover

| Padrão | Como funciona | Custo |
|---|---|---|
| **Active-passive** | heartbeat; standby assume quando o ativo cala | precisa de standby quente para não pagar boot; perda de dados se o ativo cair antes da replicação |
| **Active-active** | ambos servem tráfego e dividem carga | exige DNS com ambos os IPs ou balanceador interno; conflito de escrita |

(`README.md#Fail-over`)

Standby frio não é failover — é restauração. O tempo de indisponibilidade é decidido pelo estado do
passivo (`#Fail-over`).

## Escalar banco

Ordem de custo crescente, do menos ao mais invasivo:

| Técnica | Ganho | Custo real |
|---|---|---|
| Réplica master-slave (leitura) | distribui leitura | réplica desatualizada; leitura crítica precisa ir ao master |
| **Federação** (por função) | menos tráfego por banco, mais cache hit | lógica de aplicação decide o banco; join entre bancos vira `server link`; mais hardware |
| **Sharding** (por dado) | menos tráfego, índice menor, mais cache hit | lógica de shard na aplicação; distribuição desigual (power user num shard); rebalanceamento exige consistent hashing; join entre shards complicado |
| **Desnormalização** | leitura rápida | escrita mais cara; constraint para manter cópia em sincronia aumenta complexidade do schema |

(`README.md#Federation`, `README.md#Sharding`, `README.md#Denormalization`)

Fato que justifica a desnormalização: leitura pode superar escrita em **100:1** ou **1000:1**, e
join caro consome tempo desproporcional (`#Denormalization`).

Regra: shard por chave que aparece em **toda** consulta quente. Se a consulta precisa cruzar shards,
você comprou latência e complexidade e vai pagar nas duas.

## Cache: escolha pela falha, não pela velocidade

| Estratégia | Como escreve | Falha característica |
|---|---|---|
| **Cache-aside** | app lê cache, em miss carrega do banco e popula | 3 viagens por miss; dado obsoleto até o TTL; nó novo entra vazio |
| **Write-through** | cache escreve síncrono no store | nó novo não cacheia até o dado ser atualizado; pode gravar o que nunca será lido |
| **Write-behind** | cache confirma e grava assíncrono | perda de dado se o cache cair antes do flush; implementação mais complexa |
| **Refresh-ahead** | cache atualiza antes de expirar | previsão errada piora a performance; exige invalidação coerente com a fonte |

(`README.md#Cache-aside`, `README.md#Write-through`, `README.md#Write-behind (write-back)`, `README.md#Refresh-ahead`)

Teste de decisão: "se o cache cair agora, o que acontece com o dado?" Se a resposta for "perco
escrita", você escolheu write-behind e precisa declarar isso.

## Assincronismo e back pressure

Fila desacopla produtor de consumidor; ela **não** elimina capacidade insuficiente: se a fila cresce
sem limite, o tamanho passa da memória e o sistema degrada por swap/leitura de disco
(`README.md#Back pressure`).

Regras:

- Fila precisa de limite e política ao encher (rejeitar, degradar, derrubar) — declarada antes do
  incidente.
- Back pressure é resposta ao consumidor lento, não aumento de fila.
- Consumidor idempotente é pré-requisito: entrega pelo menos uma vez duplica.

## Tabela de decisão

| Sintoma medido | Primeira tentativa | Se não bastar |
|---|---|---|
| p99 alto, usuário único | índice, N+1, cache-aside | réplica de leitura |
| p99 alto sob carga | balanceador L7 + escala horizontal | sharding |
| escrita saturada | batching / fila | write-behind com perda declarada |
| banco grande demais para 1 host | federação | sharding por chave quente |
| leitura dominante com join caro | cache + desnormalização | materialized view / CQRS |
| dependência derruba o sistema | redundância em paralelo | degradação graciosa com cache local |

## Checklist de trade-off

- [ ] O sintoma foi classificado (performance, escalabilidade, latência, throughput)?
- [ ] A disponibilidade alvo está escrita em nines, com o downtime correspondente aceito?
- [ ] Está explícito se o sistema é CP ou AP na partição?
- [ ] Cada dependência síncrona no caminho crítico teve a disponibilidade multiplicada?
- [ ] A técnica de escala escolhida tem o custo nomeado (lógica na aplicação, join, rebalanceamento)?
- [ ] A estratégia de cache declara o que se perde quando o cache cai?
- [ ] A fila tem limite e política de descarte?
- [ ] Existe fitness function medindo o que a decisão prometeu?