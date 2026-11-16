# Operações assíncronas e concorrência otimista

Fonte: Microsoft `api-guidelines`, `azure/Guidelines.md` (CC BY 4.0). Reescrito; citação por seção.

Não duplica: paginação, versionamento genérico, idempotência e cache ficam em
`rest-best-practices.md`; semver, janela de deprecação e `Deprecation`/`Sunset` ficam em
`api-operations.md` e `api-lifecycle-headers.md`. Aqui só entra o que **não existe** na skill hoje:
`202 Accepted`, monitor de operação, chave de repetição do retry e concorrência por `ETag`.

## Quando uma operação vira assíncrona

Regra de corte declarada: implemente como operação longa quando o **p99 passa de 1 segundo** e o
cliente precisa acompanhar o andamento (`azure/Guidelines.md#Long-Running Operations & Jobs`).

Consequências:

- **Nunca** faça `PATCH` como operação longa; se precisa de semântica assíncrona de atualização,
  use ação `POST` (`#Long-Running Operations & Jobs`).
- Valide o máximo possível **na abertura** da operação, para o cliente receber o erro cedo em vez de
  descobri-lo no polling (`#Long-Running Operations & Jobs`).
- Ação síncrona concluída devolve `200-OK` (`#Performing an Action`).
- Não use "ação" para o que é CRUD padrão: criar, ler, atualizar e apagar têm verbo próprio
  (`#Performing an Action`).

## Contrato da operação longa

```http
POST /v1/exports            -> 202 Accepted
operation-location: https://api.example.com/v1/operations/8f3a
```

- `202` + header `operation-location` com URL **absoluta** do monitor de status
  (`#Long-Running Operations & Jobs`).
- O monitor é consultado com `GET`, e a resposta dele carrega os valores adicionais que o cliente
  precisa para continuar (`#Long-Running Operations & Jobs`).
- O cliente nunca deve precisar adivinhar o estado: ou a operação está no monitor, ou o recurso
  resultante já existe e é endereçável.

Checklist de desenho:

- [ ] A resposta de abertura informa **onde** acompanhar, não "tente de novo depois".
- [ ] Estados do monitor são finitos e documentados (em execução, concluída, falhou, cancelada).
- [ ] Falha tardia devolve erro diagnóstico no monitor **e** no recurso, não só um 500 genérico.
- [ ] Cliente que perde a URL da operação consegue reencontrá-la por recurso/idempotency key.
- [ ] Tempo de vida do monitor declarado (expurgo não pega o cliente de surpresa).

## Retry e chave de repetição

Para ação que precisa ser idempotente sob retry, o contrato é por header
(`#Repeatability of requests`):

```http
Repeatability-Request-ID: 6f0d3c2a-...
Repeatability-First-Sent: Tue, 18 Nov 2026 10:00:00 GMT
```

- A janela de repetição **MUST** ser de no mínimo **5 minutos** (`#Repeatability of requests`).
- Chave de repetição é diferente de `Idempotency-Key` de gateway: aqui a semântica é "este request
  é a repetição daquele", com carimbo de primeiro envio.
- Sem janela declarada, o cliente inventa timeout e o servidor trata como operação nova — o efeito
  duplicado aparece justamente no retry.

## Concorrência otimista com `ETag`

- Honre toda precondition recebida; se não puder verificar, devolva o erro de precondition
  (`#Conditional Requests`).
- `If-Match` protege atualização e exclusão de sobrescrever mudança alheia; `If-None-Match` serve
  para leitura condicional e criação (`#Conditional Requests`).
- Cada representação tem seu `ETag`: se o mesmo recurso é servido com codificações diferentes, os
  valores **não** podem ser iguais (`#Conditional Requests`).
- `ETag` é opaco para o cliente: não carregue versão semântica nele nem dependa do formato.

Fluxo mínimo:

```text
GET   /items/42          -> 200, ETag: "a1b2"
PUT   /items/42          If-Match: "a1b2"
                          -> 200 se casou; 412 se o recurso mudou
```

Regra de ouro: 412 é resposta legítima e esperada, não erro de servidor. Quem trata 412 como bug
está, na prática, fazendo last-write-wins sem declarar.

## Coleções paginadas como contrato

- A resposta de listagem é um **objeto** com um array no topo, não um array cru — assim cabe
  `nextLink` e metadados sem quebrar contrato (`#Collections`).
- `nextLink` é URL absoluta já com os parâmetros exigidos pela API; na última página o campo **não
  aparece** (`#Collections`).
- Documente explicitamente se itens podem ser **pulados ou duplicados** entre páginas; se a
  coleção não garante snapshot, diga (`#Collections`).
- Cada item carrega `id` e, quando há concorrência otimista, `etag`, para o cliente conseguir
  alterar sem um GET extra (`#Collections`).

## Versionamento como precondição

Se a API adota versão obrigatória por requisição, o padrão verificável é: parâmetro `api-version`
requerido em toda operação, valores em `YYYY-MM-DD` com sufixo `-preview` para preview, e erro
específico quando ele falta ou não é suportado (`#API Versioning`).

O valor para este repo é o formato do erro: código de erro próprio
(`MissingApiVersionParameter`, `UnsupportedApiVersionValue`) em vez de 400 genérico — o cliente
descobre o que fazer sem abrir ticket.

## Anti-padrões

| Anti-padrão | Efeito |
|---|---|
| 202 sem `operation-location` | cliente faz polling no endpoint errado |
| `PATCH` como operação longa | semântica de atualização parcial quebrada |
| Retry com chave nova a cada tentativa | duplicação garantida |
| `ETag` derivado de `updated_at` | colisão e 412 falso |
| `nextLink` relativo | cliente atrás de proxy não segue |
| Última página com `nextLink` vazio | loop de polling |