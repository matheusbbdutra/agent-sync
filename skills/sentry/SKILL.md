---
name: sentry
description: "Use when investigating errors or performance issues reported in Sentry. Depuração e rastreamento de erros no Sentry: triagem de issues, breadcrumbs e transações."
---

# Sentry — Depuração de Erros e Performance

Você investiga problemas a partir do **Sentry** e chega à causa raiz no código, com evidência. Nunca trate só o sintoma.

## Use this skill when

- Investigar um **issue/event** do Sentry (exceção, crash, regressão).
- Entender agrupamento (por que virou 1 ou N issues) e ruído de alertas.
- Correlacionar erro de produção com **release/commit** e código.
- Analisar **performance** (transactions/spans, p95, queries lentas).
- Configurar o SDK (DSN, release, environment, sample rates, PII).

## Do not use this skill when

- O erro está reproduzível e claro localmente sem telemetria (depure direto).
- A tarefa é só monitoramento/alertas sem um incidente concreto.
- É uma vulnerabilidade → use `security-auditor`.

## Instructions

1. Colete **contexto do evento**: projeto, ambiente, release, primeira/última ocorrência, nº de eventos e usuários afetados.
2. Leia o **stack trace** (filtre ruído com `trace-strip`); identifique o frame da aplicação.
3. Revise **breadcrumbs** (ações que precederam) e **tags/context** (user, url, request, device).
4. **Localize no código do release** (`path:line`) — confirme o commit.
5. Formule **hipóteses** (input/estado que disparou) e valide cada uma.
6. Identifique a **causa raiz**, proponha a correção mínima e um teste que evite reincidência.
7. Confirme que o erro cessa no release seguinte; considere alerta de regressão.

## Conceitos-chave

- **Organization / Project / Environment:** escopo; sempre confirme o ambiente (prod ≠ staging).
- **Issue × Event:** *issue* é o agrupamento; *event* é uma ocorrência com dados próprios.
- **Release / Dist:** liga eventos a uma versão/distribuição; use para achar **suspect commits** e regressões.
- **Breadcrumbs:** trilha de ações antes do erro (HTTP, logs, navegação).
- **Tags × Context:** tags são indexadas/filtráveis; context é estrutura por evento.
- **Fingerprint:** regra de agrupamento; mexer nela pode unir/dividir issues.
- **Transaction / Span:** tracing de performance; identifique o span lento no caminho.

## Triagem de um issue

- **Impacto:** frequência, usuários únicos, ambientes afetados, se é regressão (novo na release X).
- **Erro × causa:** a mensagem nem sempre é a causa; siga o frame mais interno **do seu código**.
- **Dados do request:** método, rota, status, corpo (com PII mascarada).
- **Hipótese → evidência:** cada hipótese precisa de evidência (log, código, reprodução).

## Source maps, símbolos e grouping

- **JS/TS:** sem **source maps** o trace aparece minificado — garanta o upload (`sentry-cli sourcemaps upload` / plugin de build) por release.
- **Go/native:** envie **debug symbols** quando aplicável.
- **Grouping:** mensagens com valores dinâmicos (IDs, datas) podem fragmentar em vários issues → use fingerprinting/`scope.setFingerprint`. O oposto (erros distintos no mesmo issue) indica fingerprint genérico.
- **Merge/Split/Ignore:** agrupe corretamente; ignore apenas ruído real (com regra e prazo).

## Performance / Tracing

- Compare **p95/p99** do span suspeito; procure N+1, chamadas externas, serialização.
- Use **trace** de um evento lento para achar o span culpado; correlacione com o código.
- Meça, não especule: colete baseline antes/depois da correção.

## Configuração do SDK (armadilhas comuns)

- **DSN** via variável de ambiente — nunca hardcoded/commitado.
- **release** e **environment** sempre definidos (senão perde correlação).
- **Sample rates:** `traces_sample_rate`/`profiles_sample_rate` conforme volume; cuidado com custo.
- **PII:** mantenha `send_default_pii` desligado por padrão; configure scrubbing.
- **Integrações de framework:** Symfony (`SentryBundle`), Laravel, Express/Node, browser JS — verifique a config do framework.
- **Erros engolidos:** não capture e ignore sem relançar; evite duplicar eventos (capturar no mesmo nível).

## Ferramentas

- **Sentry MCP** (opcional): `https://mcp.sentry.dev/mcp/{org}/{project}` (OAuth) — permite buscar issues/eventos e traços diretamente. Ver `mcp-advisor`.
- **API do Sentry / UI:** para consultas pontuais e links canônicos.
- **`trace-strip`**: filtre frames de biblioteca ao analisar o trace.
- **`ast-outline`**: localize a função no código do release antes de ler o trecho.

## Guardrails

- **Nunca** exiba ou cole PII/segredos (e-mails, tokens, dados de usuário): referencie a posição, mascare.
- Investigação é **read-only** por padrão; alteração de comportamento só com correção intencional e testada.
- Cite a evidência (`evento`, `path:line`, `release`); diferencie fato verificado de hipótese.
