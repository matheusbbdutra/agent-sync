# API Operations (gateway, trafego, compatibilidade e observabilidade)

> Carregue quando a API ja existe e a pergunta for operar, evoluir ou proteger: versao, deprecacao, contract testing, ingress, rate limiting, telemetria.
> Mecanica de contrato (nome de recurso, paginacao, header de rate limit, idempotency key) ja esta em `reference/rest-best-practices.md`; aqui esta a **politica**.
> Resiliencia por chamada (retry, timeout, circuit breaker) tem arquivo proprio: `error-handling-patterns/reference/resilience.md`.
> Citacoes usam o marcador `p.N` do digest de origem.

## 1. Semver e o contrato de versao

- Aplique Major.Minor.Patch com semantica operacional: major quebra, minor e aditivo compativel, patch e correcao (p.50).
- Subir de major e decisao **ativa do consumidor**, nao do produtor — o consumidor escolhe quando migrar (p.50).
- Minor e patch nunca podem exigir acao do cliente; se exigem, sao major disfarcada (p.50).
- Exponha apenas o major ao consumidor: na URL (`/v1/attendees`) ou em header roteado no ingress (`Version: v1`) (p.160).
- Escolha URL **ou** header e mantenha; alternar entre as duas obriga o consumidor a suportar dois mecanismos (p.160).
- Versao nao e feature flag: a versao antiga precisa de caminho de saida declarado (§2) (p.159).

Tipo de mudanca para numero de versao (p.50):

| Mudanca | Numero | Acao exigida do consumidor | Fonte |
| --- | --- | --- | --- |
| campo/campo opcional novo, endpoint novo | minor | nenhuma | p.50 |
| correcao de defeito sem mudar contrato | patch | nenhuma | p.50 |
| remocao, renomeacao, troca de tipo, mudanca de semantica | major | decisao ativa de migrar | p.50 |

## 2. Lifecycle e janela de deprecacao

- Mantenha cada versao em um estado explicito: Planned, Beta, Live, Deprecated, Retired (p.159).
- Beta pode quebrar sem aviso; Live nao (p.159).
- Mudanca major exige convivencia Live + Deprecated por semanas ou meses, com tempo real para o consumidor migrar (p.159).
- Mudanca minor so fica Deprecated ate a nova versao ser validada em producao (p.159).
- Retire apenas depois de trancar a janela de migracao; remover antes e a causa raiz de incidente de consumidor (p.159).
- Responder "posso remover este campo?" exige responder antes: em que estado esta a versao e quanto falta para o fim da janela (p.159).
- Beta eterno nao e contrato: ou promove para Live, ou vira Planned de novo (p.159).

Estado e o que permite quebrar ou nao (p.159):

| Estado | Pode quebrar sem aviso? | Saida | Fonte |
| --- | --- | --- | --- |
| Planned | sim (nao ha consumidor) | Beta ou Live | p.159 |
| Beta | sim | Live | p.159 |
| Live | nao | Deprecated quando houver major | p.159 |
| Deprecated | nao; janela de semanas ou meses | Retired | p.159 |
| Retired | nao ha mais contrato | — | p.159 |

## 3. Compatibilidade de schema binario (protobuf)

- Em `.proto`, so adicione campo, servico ou metodo; nunca remova, renomeie ou troque tipo (p.52).
- Nunca renumere field number: o numero e a identidade do campo no wire (p.52).
- Todo campo novo deve ser opcional; campo obrigatorio novo quebra backward compatibility (p.52).
- O compilador nao pega esse erro: a corrupcao e silenciosa e aparece em runtime (p.52).
- Versionar proto segue a propria regra de versao do schema, nao a versao da API REST correspondente (§4) (p.52, p.58).

## 4. Nao derive uma especificacao da outra

- Mantenha spec REST (OAS) e schema RPC (proto) como artefatos independentes (p.58).
- Gerar uma a partir da outra acopla os dois dominios: um campo novo no OAS reordena e renumera fields do proto e quebra compatibilidade (p.56).
- Se ambos existem, cada um versiona pela sua propria regra (p.58).
- Proposta de "ter as duas APIs de graca" via `openapi2proto` e o gatilho desse anti-padrao, nao uma otimizacao (p.56).

## 5. Gate de compatibilidade no CI

- Rode diff de spec contra a versao publicada no pipeline e falhe o build quando o resultado for "broke backward compatibility" (p.51).
- O build deve impedir a breaking change de entrar sem um override consciente (p.160).
- Trate essa verificacao como fitness function de arquitetura: um teste de unidade/integracao para a decisao arquitetural (p.227).
- Override e ato explicito e registrado; "alguem revisou a spec" nao e gate (p.160).
- O gate cobre o que o schema descreve; comportamento ainda precisa de contract test (§6) (p.47).

Passos do gate, nesta ordem (p.51, p.160, p.227):

1. gere a spec do codigo ou valide a spec versionada (p.51).
2. rode o diff contra a spec da ultima versao publicada (p.51).
3. se o resultado e breaking, falhe o build (p.160).
4. override exige registro explicito com motivo (p.160).
5. trate a verificacao como teste de arquitetura: ela protege a decisao, nao o codigo (p.227).

## 6. Contract testing: contrato nao e schema

- OpenAPI descreve forma, nao semantica: a spec nao modela o comportamento esperado sob diferentes condicoes (p.47).
- Contract testing nao e o mesmo que dizer que a API conforma a um schema (p.68). Os dois testes sao necessarios e diferentes.
- Contrato de interacao deve ter request e response de exemplo, e desses exemplos saem testes e stub server (p.68).
- Metodologia por publico:
  - API externa, publico amplo: **producer contracts** — nenhum consumidor individual consegue pedir mudanca que afete a API inteira (p.71).
  - API interna entre times: **consumer-driven contracts (CDC)** — o consumidor submete o contrato e o produtor aceita ou rejeita (p.73).
- Comecar em producer contracts e evoluir para CDC e aceitavel e explicitamente recomendado (p.73).
- Contrato unilateral mantido sem consumidor real testado e o modo de falha que o CDC evita (p.71, p.73).
- A escolha da metodologia e decisao de arquitetura (Type 1): registre em ADR (p.73; ver `adr-guidelines.md`).

## 7. Ingress: proxy, load balancer ou gateway

- Escolha a solucao mais simples que cubra os requisitos conhecidos, com atencao ao futuro imediato (p.88).
- Reverse proxy/load balancer cobre: um backend, TLS, multiplos backends e service discovery (p.88).
- Sao territorio de API gateway: authorization, retry, rate limiting, logging/tracing e circuit breaking (p.88).
- Superdimensionar (gateway enterprise para duas rotas) e subdimensionar (proxy caseiro acumulando auth, retry e rate limit ad hoc) sao o mesmo erro em direcoes opostas (p.88).
- Ao revisar a borda, liste por requisito qual dessas funcoes e realmente necessaria **hoje**; o resto nao entra (p.88).

Matriz de funcao por tipo de componente (p.88):

| Funcao | Reverse proxy / LB | API gateway | Fonte |
| --- | --- | --- | --- |
| TLS | sim | sim | p.88 |
| um backend | sim | sim | p.88 |
| multiplos backends | sim | sim | p.88 |
| service discovery | sim | sim | p.88 |
| authorization | nao | sim | p.88 |
| retry logic | nao | sim | p.88 |
| rate limiting | nao | sim | p.88 |
| circuit breaking | nao | sim | p.88 |
| logging / tracing | nao | sim | p.88 |

- Use a matriz como checklist de requisito: cada linha marcada "sim" no gateway precisa de justificativa de projeto, nao de catalogo de fornecedor (p.88).

## 8. Rate limiting e load shedding sao politicas distintas

- **Rate limiting** olha a propriedade do request: usuario, client, IP (p.189).
- **Load shedding** olha o estado do sistema: banco no limite, sem worker disponivel (p.189).
- Definir uma e chamar de outra produz comportamento indefinido sob carga (p.189).
- Muitas aplicacoes, web servers e gateways nao implementam nenhuma das duas por default, e o modo de falha correspondente fica indefinido (p.189).
- Declare explicitamente se o componente **falha aberto** (libera trafego) ou **fechado** (bloqueia) (p.159).
- Para API financeira, o livro registra fail closed como default esperado (p.159).
- Componente de seguranca que falha aberto em silencio e um dos modos de falha mais caros: ele protege quando testado e nao protege quando importa (p.110).

Sintoma para politica (p.189):

| Sintoma | Politica correta | Politica errada | Fonte |
| --- | --- | --- | --- |
| um client consome 80% da cota | rate limiting por origem | load shedding | p.189 |
| banco no limite, sem workers | load shedding | cota por usuario | p.189 |
| loop entre dois servicos internos | limite interno + monitoracao de erro | limite apenas na borda | p.189 |
| falha do componente de protecao | fail closed declarado | comportamento indefinido | p.159 |

## 9. Escolha o algoritmo de rate limiting pelo padrao de trafego

- Nomeie o algoritmo antes de configurar: fixed window, sliding window, token bucket ou leaky bucket (p.190).
- **Fixed window**: cota por periodo; simples e permite rajada na virada da janela (p.190).
- **Sliding window**: janela movel; suaviza a virada ao custo de estado (p.190).
- **Token bucket**: permite rajada com reposicao a uma taxa (p.190).
- **Leaky bucket**: taxa fixa de saida, absorvendo rajada na entrada (p.190).
- Regra de escolha: trafego com rajada em fixed window passa o dobro do limite na virada — o limite deixa de ser limite (p.190).
- A politica precisa ser justificavel ao consumidor: algoritmo nomeado, cota e janela declaradas (p.190).

Propriedade que decide a escolha (p.190):

| Propriedade do trafego | Algoritmo | Efeito colateral aceito | Fonte |
| --- | --- | --- | --- |
| cota simples por periodo, trafego uniforme | fixed window | rajada na virada da janela | p.190 |
| cota por periodo com trafego em rajada | sliding window | mais estado para manter | p.190 |
| rajada permitida, com reposicao por taxa | token bucket | pico momentaneo acima da taxa media | p.190 |
| saida sempre na mesma taxa | leaky bucket | fila na entrada | p.190 |

## 10. Limite e monitore tambem a chamada interna

- Aplique rate limiting e monitoracao de erro em chamadas service-to-service, nao apenas na borda (p.189).
- O risco nomeado e "friendly fire DoS": os proprios sistemas, em loop de chamadas, derrubam a plataforma de dentro para fora (p.189).
- Dependencia circular entre servicos e a causa tipica; o limite interno e o que impede o loop de consumir toda a capacidade (p.189).
- Borda protegida com interno desprotegido protege apenas o primeiro hop (p.189).

## 11. Resiliencia na camada comum, nao por biblioteca

- Concentre retries, timeouts, circuit breakers, bulkheads e fallbacks em gateway/service mesh (p.129).
- Implementar por biblioteca em codigo leva a comportamento inconsistente, especialmente entre linguagens e plataformas (p.129).
- Semantica identica em todos os servicos e o ganho principal; sem ela, timeouts divergentes amplificam falha em cascata (p.129).
- Circuit breaker serve para permitir recuperacao: falhar rapido libera o servico para voltar (p.129).
- O fallback e decisao de produto, tomada antes do incidente: o exemplo do livro e renderizar a agenda completa do evento em vez da agenda pessoal (p.129).
- Detalhe de retry, timeout, backoff e fallback: `error-handling-patterns/reference/resilience.md`.

## 12. Observabilidade: RED/golden signals com classe de erro

- Meça rate, errors e duration (RED) e cubra os Four Golden Signals (p.170).
- Segmente erro por classe: 5xx e falha do servico, 4xx e comportamento de cliente (p.170).
- Serie de 403/401 e sinal de ataque ou credencial vazada, nao ruido de cliente (p.171).
- Alerta sem contexto de janela gera falso positivo: queda de trafego em feriado ou fim de semana nao e incidente (p.171).
- Ajuste o alerta com contexto de atividade (logins ativos, dia util) antes de ligar o pager (p.171).
- Metrica sem segmento esconde o estagio culpado: agregado de erro nao diz se o problema e borda, servico ou cliente (p.170, p.171).

Leitura do sinal (p.170, p.171):

| Sinal | Interpretacao | Acao | Fonte |
| --- | --- | --- | --- |
| aumento de 5xx | falha do servico | investigar servico e dependencia | p.170 |
| aumento de 4xx | comportamento de cliente | documentar contrato, checar SDK | p.170 |
| serie de 401/403 | ataque ou credencial vazada | revisar authz e rotacao | p.171 |
| queda de trafego | possivel feriado/fim de semana | so alertar com contexto de atividade | p.171 |

## 13. Propagacao de headers e validacao de release

- Todo servico que termina um request e cria outro deve copiar os headers de tracing/observabilidade para o request downstream (p.172).
- Sem isso o trace se rompe no primeiro hop (p.172).
- Deve existir politica explicita sobre repassar ou nao headers de autenticacao: repassar permite que um servico se passe por outro servico ou usuario (p.172).
- Na validacao de canary/release, o cliente de teste envia `Cache-Control: no-cache, no-store` (p.172).
- Sem bypass de cache, o rollout parece saudavel porque a resposta veio do cache (p.172).
- Politica de header e decisao registrada, nao detalhe de implementacao (p.172).

Validacao de release, na ordem (p.172):

1. cliente de teste envia `Cache-Control: no-cache, no-store`;
2. o trace do request de teste aparece completo no coletor;
3. os headers de auth que descem o stream estao conforme a politica escrita;
4. so depois disso o rollout avanca.

## 14. Anti-padroes

- **Minor que exige acao do cliente**: versao quebrada com numero compativel (p.50).
- **Remover versao sem janela**: consumidor descobre em producao (p.159).
- **Renumerar field no proto**: corrupcao silenciosa em runtime (p.52).
- **Spec gerada de spec**: um campo no OAS renumera o proto (p.56).
- **Spec verde, comportamento divergente**: schema valida, semantica nao testada (p.47, p.68).
- **CDC para publico externo**: nenhum consumidor representa a API inteira (p.71).
- **Gateway por moda**: funcoes enterprise sem requisito (p.88).
- **Rate limit como load shedding**: politica por request usada para conter saturacao (p.189).
- **Fail open silencioso**: protecao que desaparece exatamente na falha (p.110, p.159).
- **Header de auth repassado sem politica**: impersonacao entre servicos (p.172).
- **Alerta sem janela**: pager disparando em feriado (p.171).

## 15. Checklist de operacao

- [ ] Cada versao tem estado de lifecycle declarado (p.159).
- [ ] Major exposto ao consumidor por URL ou header, com um unico mecanismo (p.160).
- [ ] Janela de deprecacao definida para a proxima mudanca major (p.159).
- [ ] Diff de spec no CI falha o build em breaking change (p.51, p.160).
- [ ] Contrato com exemplos, testado; schema e contrato sao testes distintos (p.68).
- [ ] Metodologia de contrato escolhida pelo publico (producer x CDC) (p.71, p.73).
- [ ] Ingress e o mais simples que atende aos requisitos conhecidos (p.88).
- [ ] Rate limiting por origem e load shedding por saturacao existem como politicas separadas (p.189).
- [ ] Fail open/closed esta declarado para cada componente de borda (p.159).
- [ ] Algoritmo de rate limiting nomeado e justificado (p.190).
- [ ] Chamada interna tem limite e monitoracao de erro (p.189).
- [ ] Resiliencia vive na camada comum, com fallback de produto definido (p.129).
- [ ] Alertas usam RED/golden signals com classe de erro e janela (p.170, p.171).
- [ ] Headers de tracing propagados; politica de header de auth escrita (p.172).
- [ ] Smoke de release envia `Cache-Control: no-cache, no-store` (p.172).

## Fontes

- **Mastering API Architecture — Design, Operate, and Evolve API-Based Systems** (James Gough, Daniel Bryant, Matthew Auburn), cap. 1 "Design, Build, and Specify APIs" (p.47, p.50, p.51, p.52, p.56, p.58), cap. 2 "Testing APIs" (p.68, p.71, p.73), cap. 3 "API Gateways: Ingress Traffic Management" (p.88, p.110, p.116), cap. 4 "Service Mesh: Service-to-Service Traffic Management" (p.129), cap. 5 "Deploying and Releasing APIs" (p.159, p.160, p.170, p.171, p.172), cap. 6 "Operational Security" (p.189, p.190), cap. 8 "Redesigning Applications to API-Driven Architectures" (p.227).
