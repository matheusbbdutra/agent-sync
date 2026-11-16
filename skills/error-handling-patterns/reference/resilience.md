# Resilience (retry, timeout, circuit breaker e degradacao)

> Politica, nao implementacao. Codigo de circuit breaker, backoff e fallback ja existe em `implementation-playbook.md` (Patterns 1-3); aqui ficam as decisoes que o codigo precisa respeitar.
> Onde a resiliencia vive (gateway/mesh x codigo) esta em `api-design-principles/reference/api-operations.md` §11.
> Citacoes usam o marcador `p.N` do digest de origem.

## 1. Premissas: trate as falacias de distribuicao como fatos do design

- O livro agrupa as oito falacias de computacao distribuida em p.134 (Mastering API Architecture). A regra acionavel nao e recita-las, e converter cada uma em premissa testavel (p.134).
- Latencia: meca media e p95/p99 antes de aprovar fluxo com chamada remota; 10 chamadas x 100 ms adicionam 1 s ao fluxo (Fundamentals, p.137).
- Latencia de cauda, nao media, define a viabilidade do requisito de resposta (p.137).
- Banda: payload grande em chamada frequente e acoplamento disfarcado — 2.000 req/s de um contrato de 500 kb viram 1 Gb/s (p.138).
- Topologia: comece sincrono; assincronismo entra so com requisito concreto (descompasso de escala, tolerancia a falha do consumidor, buffer de pico) (p.277).
- Assincronismo traz custo proprio: sincronizacao de dados, deadlock, race condition e depuracao dificil (p.277).
- Consequencia: "a rede e rapida" e "o servico responde" nao sao premissas validas; cada uma precisa de numero medido no proprio ambiente (p.137).

## 2. Retry: so quando a operacao e segura para repetir

- Antes de configurar retry, decida a semantica de 5xx por operacao: o efeito foi aplicado ou nao? (Mastering API, p.45)
- O caso do livro e o teste: em sistema de pagamento, um 500 significa que o pagamento passou? Sem resposta documentada, retry cego duplica efeito (p.45).
- Operacao nao idempotente sem chave de idempotencia nao recebe retry automatico; o retry vira decisao do cliente com idempotency key (p.45; mecanica em `api-design-principles/reference/rest-best-practices.md`).
- Operacao idempotente (leitura, PUT, DELETE) tolera repeticao, mas o retry automatico continua condicionado a semantica de 5xx documentada (p.45).
- Erro 4xx nao e retentavel: repetir comportamento de cliente so multiplica chamada invalida (p.170).
- Retry na borda e no mesh, nao em cada servico: comportamento por linguagem diverge e amplifica falha em cascata (p.129).
- Retry sem limite superior de tentativas transforma incidente de dependencia em incidente de plataforma (p.129).

Tabela de decisao por resposta (p.45, p.170):

| Resposta | Operacao idempotente | Operacao nao idempotente | Fonte |
| --- | --- | --- | --- |
| 5xx com efeito nao aplicado, documentado | retry com limite | retry com idempotency key | p.45 |
| 5xx com efeito aplicado, documentado | nao retentar; tratar como sucesso parcial | nao retentar | p.45 |
| 4xx | nao retentar | nao retentar | p.170 |
| timeout | retry com limite (efeito incerto) | nao retentar; reconciliar estado | p.45 |

## 3. Timeout: orcamento do fluxo, nao numero isolado

- Timeout e orcamento: multiplique a latencia p95 de cada hop pelo numero de chamadas encadeadas do fluxo antes de escolher o valor (p.137).
- O timeout do chamador nao pode exceder o tempo restante do fluxo; caso contrario, quem espera ja desistiu quando a resposta chega (derivado de p.137).
- Hops encadeados com timeout uniforme somam; definir por hop sem olhar o total reproduz o calculo de p.137 ao contrario.
- Timeout maior que o do cliente e inutil: a resposta nao chega a ninguem (derivado de p.137).
- Timeout nao substitui circuit breaker: um protege o chamador da espera, o outro protege o chamado da carga (§4) (p.129).

Derive o timeout nesta ordem (p.137):

1. liste os hops do fluxo critico;
2. meca p95 (e p99) de cada hop no ambiente real;
3. some os p95 para obter o orcamento total do fluxo;
4. distribua o orcamento por hop, com folga para o hop mais lento;
5. recuse o design se a soma estourar o requisito de resposta do usuario.

## 4. Circuit breaker: abrir para permitir recuperacao

- Objetivo declarado: abrir o circuito para falhar rapido e permitir que o servico se recupere (Mastering API, p.129).
- Dispare em falha repetida de chamada critica, nao em erro isolado (p.129).
- Enquanto aberto, a resposta e degradada por decisao de produto (§6), nunca erro generico quando havia alternativa barata (p.129).
- Nao combine circuit breaker com retry sem limite: retry infinito impede a recuperacao que o circuito tenta garantir (p.129).
- Implemente na camada comum (gateway/mesh) para ter semantica identica em todos os servicos (p.129).
- Codigo de referencia do mecanismo (estados closed/open/half-open) ja esta em `implementation-playbook.md`; nao reimplemente por servico (p.129).

Estados e o que cada um exige (mecanismo em `implementation-playbook.md`, Pattern 1):

| Estado | Comportamento | O que precisa estar definido | Fonte |
| --- | --- | --- | --- |
| fechado | chama o servico normalmente | limiar de falha | p.129 |
| aberto | falha rapido, sem chamar | resposta degradada por cenario | p.129 |
| meio-aberto | testa recuperacao | numero de sucessos para fechar | p.129 |

- A transicao precisa de um criterio objetivo (limiar de falha, numero de sucessos); circuito sem criterio abre e fecha por ruido (p.129).

## 5. Falha em cascata: os tres vetores

- **Retry amplificado**: N chamadores retentando contra dependencia lenta multiplicam carga por N (p.129).
- **Timeout divergente**: servicos com timeouts diferentes mantem a dependencia sob carga por mais tempo do que o necessario (p.129).
- **Chamada interna sem limite**: loop entre servicos derruba a plataforma de dentro para fora — "friendly fire DoS" (p.189).
- Contramedida para os tres: limite e monitoracao de erro tambem nas chamadas internas, nao so na borda (p.189).
- Ausencia de protecao nao e neutra: muitas aplicacoes e gateways nao implementam limite por default e o modo de falha fica indefinido (p.189).

## 6. Fallback e degradacao: decisao de produto, tomada antes do pico

- O fallback e decisao de produto: o exemplo do livro e renderizar a agenda completa do evento em vez da agenda pessoal (p.129).
- Escreva a resposta degradada por cenario (dependencia fora, dado indisponivel, cota estourada) antes do incidente, nao durante (p.129).
- Declare se o componente falha aberto (libera) ou fechado (bloqueia); API financeira tende a fail closed (p.159).
- Componente de seguranca que falha aberto em silencio protege no teste e nao protege na falha (p.110).
- Fallback tambem precisa de limite: aplicar o mesmo principio de limite interno de p.189 evita que a degradacao vire o segundo incidente (derivado de p.189).
- Degradacao precisa ser observavel como estado; se o fallback nao aparece na metrica, o time nao sabe que esta nele (§8) (p.170).

## 7. Aplicacao com modelo: falha, fallback e degradacao

- Gateway/router de modelo existe para degradar: rotear para modelo alternativo, retentar apos espera curta ou tratar a falha com elegancia (AI Engineering, p.479).
- Router deve ser rapido e barato; um router tao caro quanto o modelo nao e camada de degradacao (p.477).
- Adicione a camada de gateway quando a dor aparecer (rate limit de provedor, multiplicacao de modelos), nao no primeiro dia (p.470).
- Enumere os modos de falha antes de escolher a degradacao: tool invalida, parametro invalido, valor errado, goal nao atingido, reflection error (o agente declara concluido sem estar), ausencia de tool para o dominio (p.318, p.319).
- Cada modo de falha pede resposta diferente: retry resolve falha transitoria, nao resolve goal nao atingido (p.318).
- Logue toda chamada de tool com entrada e saida; sem isso nao se sabe qual modo de falha dominou (p.320).
- Nao trate "funcionou" como metrica: meca a frequencia de cada modo de falha (p.318).

### 7.1 Degradacao de guardrail e de validacao

- Mantenha guardrails na entrada **e** na saida mesmo em modo degradado: remover os dois amplia o risco exatamente na pior hora (p.271).
- Acompanhe violation rate junto com false refusal rate: degradacao de guardrail move os dois numeros, e so um deles aparece no dashboard (p.268).
- Um guardrail que recusa tudo tem violation rate zero e sistema inutil — metrica unica esconde esse estado (p.268).
- O par de numeros so faz sentido lido junto: violation rate cai e false refusal sobe quando o guardrail fica mais agressivo (p.268).
- Degradacao de guardrail precisa de prazo: desligar sem data de retorno transforma excecao em configuracao permanente (derivado de p.271).

## 8. Meça a degradacao, nao so a falha

- Desenhe as metricas a partir dos modos de falha que se quer pegar, nao do que a ferramenta expoe por default (p.487).
- Logue configuracao, prompt final, saida e tool calls com IDs e reconstrua trace por request com tempo e custo por passo (p.490).
- Mudanca silenciosa de modelo, de provedor ou de template e falha sem erro: sem log de configuracao ela nao aparece (p.492).
- Reporte latencia por percentil; a media esconde a cauda que define a experiencia (p.434).
- Conte goodput (requests que cumprem o SLO) em vez de throughput bruto; "dobramos o throughput" com SLO violado nao e melhoria (p.435).
- Para modelo, latencia total = TTFT + TPOT x tokens de saida; meca as duas partes separadas (p.433).
- Do lado da API, segmente erro por classe (5xx servico, 4xx cliente, serie de 401/403 ataque) e ajuste alerta por janela de atividade (Mastering API, p.170, p.171).

- Acompanhe MTTD (tempo ate detectar), MTTR (tempo ate recuperar) e CFR (taxa de falha do cliente): sao os numeros que dizem se a degradacao funcionou (p.486).
- MTTD alto com MTTR baixo significa problema de deteccao, nao de correcao (p.486, p.487).

## 9. Anti-padroes

- **Retry cego em operacao nao idempotente**: efeito duplicado (pagamento, transferencia) (p.45).
- **Retry sem limite**: impede a recuperacao que o circuit breaker tentaria (p.129).
- **Timeout uniforme por hop**: soma dos hops excede o orcamento do fluxo (p.137).
- **Circuit breaker por servico**: semantica divergente entre linguagens (p.129).
- **Fallback decidido no incidente**: erro generico onde havia alternativa barata (p.129).
- **Fail open silencioso**: protecao que desaparece na falha (p.110, p.159).
- **Degradacao sem metrica**: ninguem sabe que o sistema esta em modo degradado (p.170, p.487).
- **Media de latencia**: cauda longa invisivel (p.434).
- **Throughput sem SLO**: numero grande com experiencia pior (p.435).
- **"Funcionou" como teste**: modo de falha nao medido continua acontecendo (p.318).
- **Assincronismo por default**: custo de deadlock, race e depuracao sem requisito que pague (p.277).

## 10. Checklist

- [ ] A semantica de 5xx por operacao esta documentada (efeito aplicado ou nao) (p.45).
- [ ] Toda operacao com retry e idempotente ou tem idempotency key (p.45).
- [ ] Retry tem limite de tentativas e vive na camada comum (p.129).
- [ ] O timeout de cada hop respeita o orcamento total do fluxo (p.137).
- [ ] Circuit breaker abre em falha repetida e tem resposta degradada definida (p.129).
- [ ] Chamada interna tem limite e monitoracao de erro (p.189).
- [ ] Fail open/fail closed esta declarado por componente (p.159).
- [ ] O fallback e decisao escrita, com cota propria e estado observavel (p.129, p.189).
- [ ] Modos de falha enumerados e medidos por frequencia (p.318).
- [ ] Latencia reportada em percentil e goodput contra SLO (p.434, p.435).
- [ ] Configuracao e trace logados o suficiente para reconstruir o request (p.490, p.492).

## Fontes

- **Mastering API Architecture — Design, Operate, and Evolve API-Based Systems** (James Gough, Daniel Bryant, Matthew Auburn), cap. 1 "Error Handling" (p.45), cap. 3 "API Gateways" (p.110), cap. 4 "Reliability" e "The 8 Fallacies of Distributed Computing" (p.129, p.134), cap. 5 "Deploying and Releasing APIs" (p.159, p.170, p.171), cap. 6 "Operational Security" (p.189, p.190).
- **Fundamentals of Software Architecture — An Engineering Approach** (Mark Richards, Neal Ford), cap. 9 "Foundations" (p.137), cap. 17 "Microservices" (p.277).
- **AI Engineering — Building Applications with Foundation Models** (Chip Huyen), cap. 6 "Agent Failure Modes and Evaluation" (p.318, p.319, p.320), cap. 9 "Inference Performance Metrics" (p.433, p.434, p.435), cap. 10 "AI Engineering Architecture" (p.470, p.477, p.479) e "Monitoring and Observability" (p.487, p.490, p.492).
