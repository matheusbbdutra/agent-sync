# Headers de ciclo de vida da API (Sunset, Deprecation, traceparent)

> Referencia carregada por `../SKILL.md` e por `reference/api-operations.md` §2 (lifecycle e janela de deprecacao).
> Nao duplica: estados Planned/Beta/Live/Deprecated/Retired, gate de compatibilidade no CI e semver ficam em `reference/api-operations.md` §1-§5; propagacao de headers de tracing na validacao de release fica em §13 do mesmo arquivo.
> O que esta aqui e a **mecanica de fio** dos tres headers: nome, formato de campo e semantica, sempre a partir da spec. Inferencia do autor marcada como **[inferencia]**.

## 1. Tres headers, tres perguntas

| Header | Pergunta que responde | Spec | Status | Formato do valor |
| --- | --- | --- | --- | --- |
| `Deprecation` | quando deixa de ser recomendado | RFC 9745 | Standards Track, marco/2025 | Structured Field Date: `@` + segundos Unix |
| `Sunset` | quando deixa de responder | RFC 8594 | Informational, maio/2019 | HTTP-date |
| `traceparent` | em que trace/span este request esta | W3C Trace Context | Recommendation / Level 2 | `version-trace-id-parent-id-trace-flags` |

Duas confusoes que custam caro em revisao de contrato:

- `Deprecation` e `Sunset` **nao usam o mesmo formato de data**. Um e epoch com `@`, o outro e HTTP-date textual.
- `traceparent` nao e header de ciclo de vida; e observabilidade. Esta aqui porque e o que transforma "achamos que ninguem usa a v1" em medicao (§7).

## 2. Sunset (RFC 8594)

Abstract da spec: "This specification defines the Sunset HTTP response header field, which indicates that a URI is likely to become unresponsive at a specified point in the future. It also defines a sunset link relation type that allows linking to resources providing information about an upcoming resource or service sunset."

Semantica (§3):

- "The Sunset header field contains a single timestamp that advertises the point in time when the resource is expected to become unresponsive."
- "The Sunset value is an HTTP-date timestamp, as defined in Section 7.1.1.1 of [RFC7231], and SHOULD be a timestamp in the future."
- "It is safest to consider timestamps in the past mean the present time, meaning that the resource is expected to become unavailable at any time."
- ABNF: `Sunset = HTTP-date`.

Consequencias praticas:

- **`SHOULD` no futuro, nao `MUST`**: data no passado e legal e deve ser lida como "pode cair agora". Nao trate data passada como header invalido.
- **`likely` nao e SLA**: o RFC anuncia expectativa, nao compromete disponibilidade ate a data nem garante queda depois dela.
- **Informational**: e especificacao de header, nao norma de obrigatoriedade. Um cliente que ignora `Sunset` nao viola nada.
- Formato: HTTP-date no subconjunto IMF-fixdate — tamanho fixo, zona unica, sempre `GMT` (RFC 7231 §7.1.1.1). Forma: `Sunset: Sun, 06 Nov 1994 08:49:37 GMT`.
- O RFC descreve **o header**, nao o comportamento depois da data. Qual status code a rota devolve apos o sunset (410, 404, redirect) e decisao do provedor, que precisa estar documentada em outro lugar. **[inferencia]**

## 3. Deprecation (RFC 9745)

Abstract: "The Deprecation HTTP response header field is used to signal to consumers of a resource (identified by a URI) that the resource will be or has been deprecated."

Semantica (§2.1), texto da spec:

- "describes the deprecation of the resource identified with the response it occurred within (see Section 6.4.2 of [HTTP])."
- "It conveys the deprecation date, which may be in the future (the resource in context will be deprecated at that date) or in the past (the resource in context was deprecated at that date)."
- "`Deprecation` is an Item Structured Header Field; its value MUST be a Date as per Section 3.3.7 of [RFC9651]."
- O documento "uses 'Structured Field Values for HTTP' [RFC9651] to specify syntax and parsing of date values".

Exemplo da propria spec (recurso deprecado em sexta, 30 de junho de 2023, 23:59:59 UTC):

```http
Deprecation: @1688169599
```

Leitura correta:

- `@` + segundos desde epoch. **Nao** e HTTP-date; `Deprecation: Fri, 30 Jun 2023 23:59:59 GMT` esta errado.
- Data no futuro = aviso antecipado. Data no passado = ja deprecado.
- **Deprecation nao muda comportamento**: o recurso continua respondendo. O header so sinaliza que a migracao deve comecar.
- Comparacao entre as duas specs: `Deprecation` responde "quando deixa de ser recomendado"; `Sunset` responde "quando deixa de responder". A RFC 9745 dedica §4 a `Sunset` justamente para marcar a diferenca.
- Como os dois convivem: `Deprecation` <= `Sunset`. Deprecation depois do sunset e contradicao de contrato. **[inferencia]**

## 4. Link relations: onde mora o "por que" e o "como migrar"

Os dois headers dizem **quando**, nao **o que fazer**. O complemento e `Link`:

| Relation | Spec | Aponta para |
| --- | --- | --- |
| `deprecation` | RFC 9745 §3 | documentacao da deprecacao e de como lidar com ela |
| `sunset` | RFC 8594 | politica de sunset do recurso ou do servico |

- Formato do `Link` e da RFC 8288 (Web Linking): URI entre `<>`, seguida de parametros `;`-separados, com `rel` obrigatorio.
- RFC 9745 §3 e explicito sobre o proposito: informar "where to find more information about deprecation of the context", e isso vale "before the actual deprecation to make a deprecation policy discoverable or after deprecation when there may be documentation about the deprecation and how to manage it".
- Exemplo de forma combinada (estrutura conforme as duas specs, valores ilustrativos):

```http
Deprecation: @1688169599
Sunset: Sun, 06 Nov 1994 08:49:37 GMT
Link: <https://developer.example.com/deprecation>; rel="deprecation"; type="text/html"
```

- Header sem link e aviso sem instrucao: o consumidor sabe que vai quebrar e nao sabe o que fazer. Publicar o link e parte do aviso, nao enfeite. **[inferencia]**

## 5. traceparent (W3C Trace Context)

O header tem quatro campos, nesta ordem: `version`, `trace-id`, `parent-id`, `trace-flags`. ABNF da spec (version 00):

```abnf
version-format = trace-id "-" parent-id "-" trace-flags
trace-id      = 32HEXDIGLC   ; 16 bytes. All zeroes forbidden
parent-id     = 16HEXDIGLC   ; 8 bytes. All zeroes forbidden
trace-flags   = 2HEXDIGLC    ; 8 bit flags
```

Regras que a spec enuncia:

- `version` e 1 byte em dois caracteres hex; **`ff` e invalido**; a spec define version `00`.
- `trace-id` e `parent-id` **todos zero sao proibidos** — nao sao "id vazio valido".
- Hex e **lowercase** (`HEXDIGLC`). Uppercase e invalido.
- `trace-flags` tem um bit em uso: o `sampled`.
- Valor valido, do exemplo da spec: `00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`.
- Tamanho: para version 00, 32 + 16 + 2 caracteres hex mais 3 hifens = **55 caracteres**. **[inferencia]** aritmetica sobre a ABNF acima, nao frase da spec — o tamanho e fixo so nessa versao.
- Pass-through **nao** deve interpretar a versao: a spec orienta que servicos de passagem "should not analyze the version" e "should expect that headers may have larger size limits in the future and only disallow prohibitively large headers". Parsear versao futura como erro quebra o trace de quem so repassa.

`tracestate` e o segundo header, que carrega o que nao cabe no `traceparent`:

- valor e uma `list` de `list-members` separados por virgula; cada member e `chave=valor`;
- "There can be a maximum of 32 `list-members`";
- ao exceder, "the right-most `list-member` should be removed";
- espacos e tabs em volta do member sao ignorados; members vazios sao permitidos.

## 6. O que propagar entre servicos

Texto normativo da Trace Context, que separa os dois casos:

- Com header recebido: "A vendor receiving a `traceparent` request header MUST send it to outgoing requests. It MAY mutate the value of this header before passing it to outgoing requests."
- E o encadeamento com o `tracestate`: "If the value of the `traceparent` field wasn't changed before propagation, `tracestate` MUST NOT be modified as well" — quem so repassa nao mexe no `tracestate`.
- Sem header recebido: "A vendor receiving a request without a `traceparent` header SHOULD generate `traceparent` headers for outbound requests, effectively starting a new trace." A spec observa que gerar mesmo sem amostrar e o esperado, para propagar a decisao de sampling para baixo.

Regra operacional (**[inferencia]** a partir dos trechos acima):

1. `trace-id` identifica o trace inteiro: **nunca** e regenerado no meio do caminho. Regenerar parte a arvore em duas.
2. `parent-id` identifica o span do chamador. O span criado no servico passa a ser o novo `parent-id` no hop seguinte.
3. `trace-flags` propaga a decisao de sampling para baixo; reescrever para "sampled" sem amostrar produz trace pela metade.
4. `tracestate` so muda por quem tem entrada para adicionar, e respeitando o limite de 32.
5. Header nao atravessa fila, topico ou job em background sozinho: o contexto precisa ser copiado para o envelope da mensagem, senao o trace morre no produtor. **[inferencia]**

O que isso resolve em contrato de API: com `traceparent` propagado, "quem ainda chama `/v1/attendees`" deixa de ser pergunta para o time de produto e passa a ser consulta no coletor de traces. Ver tambem `api-operations.md` §13 (propagacao de headers e validacao de release), que cobre o lado do rollout.

## 7. Como os tres entram na politica de deprecacao

Sequencia, com o header que carrega cada passo:

| Passo | Header | O que muda para o consumidor |
| --- | --- | --- |
| anuncio | `Deprecation: @<futuro>` + `Link rel="deprecation"` | nada funciona diferente; a migracao comeca |
| medicao | `traceparent` + log por rota/versao | nada; o provedor descobre quem ainda chama |
| aviso final | `Sunset: <data>` | nada funciona diferente ate a data |
| retirada | — (a janela fechou) | resposta muda ou deixa de existir |

Decisoes que precisam estar escritas, nao assumidas:

- **Em quais respostas o header aparece**: so 2xx ou todas, incluindo erro. As duas escolhas sao defensaveis; a omissao nao e — consumidor que so ve o header em 200 vai concluir que ele nao existe.
- **`Deprecation` no primeiro dia da janela, `Sunset` quando a data ja e conhecida.** Inverter a ordem tira do consumidor a unica informacao acionavel (quanto tempo ele tem).
- **Quem monitora o uso**: sem `traceparent` propagado e log por rota, a decisao de retirar a versao e palpite com data. O gate de compatibilidade de `api-operations.md` §5 protege o schema, nao revela consumidor.
- **Sunset sem plano de resposta**: a data chega e a rota continua no ar "por seguranca". Isso ensina o consumidor que o header nao significa nada. **[inferencia]**
- **Header em versao deprecated x header em versao Live**: so a versao em deprecacao carrega `Deprecation`/`Sunset`; emitir na Live transforma aviso em ruido.

## 8. Checklist

- [ ] `Sunset` e HTTP-date IMF-fixdate, em GMT, com data no futuro ou no presente (data passada = "pode cair agora").
- [ ] `Deprecation` e Structured Field Date: `@` + segundos Unix, nunca HTTP-date.
- [ ] `Deprecation` <= `Sunset`; os dois nunca se contradizem.
- [ ] `Link` com `rel="deprecation"` (e `rel="sunset"` quando houver politica publica) apontando para doc real.
- [ ] Politica escrita define em quais respostas os headers aparecem.
- [ ] `traceparent` gerado quando ausente e propagado quando presente, sem regenerar `trace-id`.
- [ ] `tracestate` so modificado por quem adiciona entrada, respeitando o limite de 32 members.
- [ ] Hex lowercase; `trace-id`/`parent-id` nunca todos zero; version `ff` rejeitada.
- [ ] Contexto de trace copiado para envelope de mensagem em fluxo assincrono.
- [ ] Uso da versao deprecada medido por rota/versao antes de fixar a data de `Sunset`.

## Fontes

- **RFC 8594 — The Sunset HTTP Header Field** (E. Wilde, maio/2019, Informational): abstract, §3 semantica, `Sunset = HTTP-date`, `SHOULD` no futuro, leitura de data passada, link relation `sunset`. https://www.rfc-editor.org/rfc/rfc8594.html e https://www.rfc-editor.org/info/rfc8594/
- **RFC 9745 — The Deprecation HTTP Response Header Field** (S. Dalal, E. Wilde, marco/2025, Standards Track): abstract, §2.1 (descricao, data passada/futura, "Item Structured Header Field; its value MUST be a Date as per Section 3.3.7 of [RFC9651]"), exemplo `Deprecation: @1688169599`, §3 link relation `deprecation`, §4 `Sunset`. https://www.rfc-editor.org/rfc/rfc9745.html
- **RFC 7231 §7.1.1.1 — Date/Time Formats**: HTTP-date, IMF-fixdate como formato preferido, tamanho fixo e zona unica. https://www.rfc-editor.org/rfc/rfc7231.html (trecho lido em https://stackoverflow.com/questions/75235516/does-googles-http-server-return-a-header-with-the-wrong-date-time-format)
- **RFC 9651 — Structured Field Values for HTTP**: syntax de Date usada pela RFC 9745 §3.3.7. https://www.rfc-editor.org/rfc/rfc9651.html (referenciada pela RFC 9745; nao aberta integralmente)
- **RFC 8288 — Web Linking**: formato do `Link` e parametro `rel`. https://www.rfc-editor.org/rfc/rfc8288.html (referenciada; formato lido tambem em https://apiguide.dev/specifications/web-linking)
- **W3C Trace Context (Recommendation, 2021-11-23)**: campos do `traceparent`, ABNF de `version-format`, `trace-id`/`parent-id` 32/16 HEXDIGLC com zero proibido, `ff` invalido, exemplo `00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`, regra de pass-through, "MUST send it to outgoing requests", regra do `tracestate` inalterado, processing model sem header recebido. https://www.w3.org/TR/trace-context/
- **W3C Trace Context Level 2 (2024-03-28)**: `tracestate` como lista de no maximo 32 `list-members`, remocao do right-most, texto de geracao de `traceparent` para outbound. https://www.w3.org/TR/trace-context-2/
- **Trace Context — spec/30-processing-model.md (repo w3c/trace-context)**: passo a passo do caso "No traceparent Received". https://github.com/w3c/trace-context/blob/main/spec/30-processing-model.md
- **Trace Context — spec/20-http_request_header_format.md (repo w3c/trace-context)**: ABNF e campos do `traceparent`. https://github.com/w3c/trace-context/blob/main/spec/20-http_request_header_format.md
- **Nao confirmado**: numero de secao da Trace Context para `traceparent`/`tracestate` (citei por titulo de secao, nao por numero); texto literal que afirma o total de 55 caracteres (o valor foi derivado da ABNF); comportamento de status code apos `Sunset` (nenhuma das duas RFCs fixa isso).
