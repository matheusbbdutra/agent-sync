---
name: symfony
description: Especialista no framework Symfony (PHP 8+) cobrindo HTTP Kernel, front controllers finos, roteamento por atributos, injeção de dependência/autowiring, configuração por ambiente, Messenger, Event Dispatcher, Form/Validator, segurança e comandos de console. Use ao criar ou refatorar aplicações Symfony, serviços, controllers ou integrações.
---

# Symfony (PHP 8+)

Você é um engenheiro especialista em Symfony. Aplique as convenções do framework e mantenha o domínio independente da infraestrutura.

## Use this skill when

- Criar/estruturar um projeto Symfony ou um bundle.
- Implementar controllers, rotas, serviços, mensagens assíncronas ou comandos.
- Configurar injeção de dependência, ambiente, segredos ou segurança.
- Refatorar controllers gordos ou lógica de framework vazando no domínio.

## Do not use this skill when

- O projeto não usa Symfony (PHP puro → use `php-pro`).
- A tarefa é só modelagem de banco (→ use `doctrine`).
- É apenas um script CLI isolado sem framework.

## Instructions

1. Confirme a versão do Symfony e o layout (`src/`, `config/`, `templates/`, `bin/console`).
2. Prefira atributos PHP (`#[Route]`, `#[AsCommand]`, `#[AsEventListener]`) às versões em YAML/XML.
3. Mantenha controllers finos: orquestram, não contêm regra de negócio.
4. Injete serviços e interfaces, nunca o container.
5. Verifique com testes de kernel/functional (ver `phpunit-symfony`).

## Estrutura e ciclo de vida

- **Front controller:** `public/index.php` instancia o `Kernel` e trata `Request` → `Response`.
- **Kernel:** `src/Kernel.php` registra bundles e configura o container; não coloque lógica de negócio.
- **Bundles:** agrupe por domínio/feature em apps grandes; evite um bundle para cada coisa.
- **Organização:** prefira `src/<Contexto>/{Domain,Application,Infrastructure}` a camadas globais.

## Controllers e rotas

- Controller = adaptador HTTP. Receba `Request`/DTO mapeado, delegue ao caso de uso, devolva `Response`.
- Use `#[Route('/orders/{id}', methods: ['GET'])]` e `#[IsGranted]` para autorização.
- Valide entrada com `#[MapRequestPayload]`/Form + Validator, não com `if` no controller.
- Nunca injete `EntityManager`/repositórios de ORM no controller: chame um serviço de aplicação.
- Retorne códigos corretos (200/201/204/400/404/409) e use Serializer para representação.

## Injeção de dependência

- Autowiring + autoconfiguration; declare serviços em `config/services.yaml`.
- Programe contra interfaces; ligue interface→implementação em `services.yaml` (`bind`/alias).
- Serviços são stateless e compartilhados: cuidado com estado mutável.
- Use `#[Autowire]`, service locators para coleções e `#[TaggedIterator]` para plugins.
- Configure tempo de vida (shared) apenas quando necessário; evite singletons stateful.

## Configuração e ambiente

- `config/packages/*` por bundle; `config/packages/<env>/*` para overrides por ambiente.
- Use variáveis de ambiente (`%env(...)%`) e **segredos do Symfony** (`bin/console secrets:*`) para dados sensíveis — nunca commite segredos.
- Flex: `composer require` instala receitas; revise o que a receita altera.

## Mensageria e eventos

- **Messenger:** `#[AsMessageHandler]`; rotas sync/async em `config/packages/messenger.yaml`.
- Despache comandos/eventos de domínio; handlers devem ser idempotentes.
- **EventDispatcher:** `#[AsEventListener]` para efeitos transversais; não use eventos para substituir chamadas diretas do fluxo principal.
- Configure retry/DLQ e monitore transporte em produção.

## Form, Validator e Console

- Forms para entrada de UI; DTOs + Validator para APIs.
- Constraints em atributos; grupos de validação por contexto.
- `#[AsCommand(name: 'app:x')]`: comandos finos delegando a serviços; retorne exit codes corretos.

## Segurança

- `security.yaml`: firewalls, providers, access_control.
- Autenticação por authenticators; autorização por **voters** (`#[IsGranted('EDIT', subject: $entity)]`).
- Nunca confie em input do cliente; valide e escape em contextos de render.

## Qualidade

- Erros: use exceções específicas e o `ErrorListener` do framework; não capture e ignore.
- Performance: evite queries em loop (N+1 do Doctrine), cache HTTP/Doctrine quando fizer sentido, lazy services.
- Siga as convenções do projeto (rector, php-cs-fixer, PHPStan/Psalm) antes de inventar padrão novo.

## Referências do repositório

- Use `doctrine` para ORM/persistência e `phpunit-symfony` para testes.
