# Plano: agente de análise de MR/PR

## Objetivo

Executar a mesma revisão técnica em um checkout local, em Pull Requests do GitHub, em Merge Requests do GitLab SaaS e em instâncias self-hosted antigas, incluindo GitLab 12, com publicação opcional e segura dos achados.

## Decisões de arquitetura

- O agente `code-reviewer` continua independente do provedor.
- Um contrato normalizado representa a mudança: provedor, projeto, identificador, base/head SHA, metadados e diffs por arquivo.
- `ChangeProvider` obtém a mudança; `ReviewPublisher` publica achados. A análise não depende de publicação.
- O modo padrão é somente leitura. `--dry-run` não faz chamadas de escrita.
- O Git local será acessado pelo executável `git`, preservando compatibilidade com o clone e a versão instalados.
- Tokens vêm de variáveis de ambiente ou credential helper e nunca são gravados em logs, prompts persistidos ou `STATE.md`.

## Adaptadores

### Local

1. Validar que o diretório é um checkout seguro.
2. Resolver base e head com `git rev-parse`/`git merge-base`.
3. Obter resumo e patch com `git diff --stat` e `git diff`.
4. Limitar tamanho e aplicar redaction antes de enviar ao agente.

### GitHub

1. Buscar metadados da PR e arquivos alterados pela API REST.
2. Normalizar patches e SHAs para o contrato comum.
3. Publicar comentários apenas quando explicitamente solicitado.
4. Usar uma chave idempotente por achado para evitar comentários duplicados.

### GitLab

1. Aceitar `base URL`, `project ID/path` e `merge request IID`.
2. Usar REST v4, sem GraphQL.
3. Para GitLab 12, preferir o endpoint legado de diffs disponível nessa linha (`merge_requests/:iid/diffs`); não depender de campos introduzidos em versões novas.
4. Publicar observações pela Notes API somente com opt-in; começar por comentário geral, deixando comentários posicionais para uma etapa posterior.
5. Permitir self-hosted com TLS verificado e timeout configurável.

## Fluxo do agente

1. Detectar origem ou receber `--provider` explicitamente.
2. Buscar a mudança pelo adaptador.
3. Executar redaction e limites de tamanho.
4. Montar prompt com contexto do repositório e checklist do `code-reviewer`.
5. Validar saída estruturada dos achados (`severity`, `file`, `line`, `problem`, `suggestion`).
6. Salvar relatório local e, se solicitado, publicar de forma idempotente.
7. Registrar métricas e erros sem payload sensível.

## Entregas incrementais

1. Contratos, modelo normalizado e provider local; testes com repositório temporário.
2. CLI de análise local com saída Markdown/JSON.
3. Adaptador GitHub em modo leitura e `dry-run`.
4. Adaptador GitLab REST com configuração de URL, incluindo compatibilidade v12.
5. Publicadores opcionais, idempotência e comentários posicionais.
6. Integração CI para GitHub Actions e GitLab CI, sem exigir escrita no repositório por padrão.

## Verificação e riscos

- Testar diffs vazios, arquivos binários, renomeações, patches truncados, branches divergentes e erro de autenticação.
- Nunca executar o código da MR durante a coleta; a primeira versão apenas lê Git e APIs.
- Validar a API real da instância GitLab 12 antes de ativar publicação.
- Cobrir redaction e garantir que tokens não apareçam em relatórios, logs ou comentários.
