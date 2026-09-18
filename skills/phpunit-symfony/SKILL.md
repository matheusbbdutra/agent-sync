---
name: phpunit-symfony
description: "Testes automatizados no Symfony com PHPUnit (unitários, integração e WebTestCase)."
---

# PHPUnit no Symfony

Você é um engenheiro especialista em testes de aplicações Symfony. Escreva testes rápidos, isolados e determinísticos, que dão confiança real — não testes cerimoniais.

## Use this skill when

- Escrever/revisar testes unitários, de integração ou funcionais no Symfony.
- Configurar o ambiente de testes (banco, fixtures, variáveis de ambiente).
- Mockar serviços, HTTP externo, e-mail ou mensageria.
- Reduzir flakiness, tempo de suíte ou dependência de rede.

## Do not use this skill when

- O teste é de código PHP sem o framework (→ `php-pro`).
- O foco é TDD de fluxo geral (→ `tdd-orchestrator`).
- É teste E2E de navegador puro sem camada Symfony (→ `e2e-testing-patterns`).

## Instructions

1. Escolha o nível mínimo que prova o comportamento: unit → integration → functional.
2. Rode `bin/phpunit` para confirmar a configuração atual (`phpunit.xml.dist`).
3. Garanta isolamento (banco/estado) e determinismo (tempo, aleatoriedade, rede).
4. Escreva o teste que falha pelo motivo certo antes de implementar (TDD).
5. Meça o tempo; mantenha a suíte rápida e paralelizável.

## Níveis de teste

- **Unit (`PHPUnit\Framework\TestCase`):** classes puras, sem kernel. Rápidos.
- **Integration (`KernelTestCase`):** sobe o kernel, testa serviços/container/repositórios.
- **Functional (`WebTestCase`):** requisições HTTP completas via `client`.
- **E2E (Panther/Playwright):** navegador real; use com parcimônia.

## Configuração

- `symfony/test-pack` (PHPUnit + browser kit/dom crawler + maker).
- `phpunit.xml.dist`: `tests/` como suite, `bootstrap="tests/bootstrap.php"`, variáveis de ambiente (`APP_ENV=test`).
- Use `.env.test` com banco dedicado (`DATABASE_URL=..._test`).
- Nunca rode testes contra o banco de desenvolvimento/produção.

## Functional (WebTestCase)

- `$client = static::createClient(); $client->request('GET', '/orders')`.
- Asserções: `assertResponseIsSuccessful()`, `assertResponseStatusCodeSame(201)`, `assertSelectorTextContains(...)`, `assertJsonStringEqualsJsonString(...)`.
- `$client->loginUser($user)` para autenticação; teste também os caminhos **sem** permissão (403/401).
- Verifique a resposta e, quando relevante, o estado persistido.

## Integration (KernelTestCase)

- `self::getContainer()->get(Service::class)` para testar serviços reais.
- Substitua serviços por test doubles no container (`$container->set(...)`) quando necessário.
- Teste repositórios contra o banco real de teste (dão mais confiança que mocks de ORM).

## Banco de dados

- **Isolamento:** DAMA DoctrineTestBundle (rollback por teste) ou `zenstruck/foundry` (`ResetDatabase`).
- **Factories** (Foundry) > fixtures estáticas; crie só os dados do cenário.
- `RefreshDatabase` quando colisões de identidade, `Transaction`/rollback para velocidade.
- Nunca dependa da ordem dos testes nem de IDs fixos.

## Mocks e dublês

- `createMock`/`createStub` para colaboradores; evite mockar o que você não possui.
- **HttpClient:** `MockHttpClient` com respostas canônicas; nunca acesse a rede no teste.
- **Mailer:** `assertEmailCount`, `assertEmailAddressContains`.
- **Messenger:** transporte in-memory; `assertQueued`/`assertDispatched` e processe o transporte no teste.
- **Tempo:** `ClockMock`/`SymfonyClock` para congelar o relógio; nunca `sleep`.

## Boas práticas

- Um comportamento por teste, nome revelando a regra testada.
- Fixtures de dados explícitas e mínimas; helper/factory para evitar duplicação.
- Sem dependência de rede, tempo real, aleatoriedade ou estado global compartilhado.
- Rode em paralelo (`paratest`) quando a suíte crescer; mantenha testes independentes.
- Cobertura é consequência, não meta: cubra regras e casos de borda, não getters.

## CI

- `bin/phpunit --coverage-text` com limite mínimo de cobertura; cache do Composer/DB.
- Falhe o pipeline em teste vermelho; rode lint/análise estática (PHPStan/Psalm) como step separado.

## Referências do repositório

- Framework: `symfony`; persistência: `doctrine`; TDD geral: `tdd-orchestrator`.
