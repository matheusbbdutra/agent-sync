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

1. Validar que o diretório é um checkout Git e aceitar `base` e `head` explícitos, por exemplo `upstream/branch-teste` e `origin/branch-teste`. Se faltarem referências ou houver ambiguidade entre fork e upstream, pedir a escolha ao usuário; não presumir que `origin` é a base.
2. Quando solicitado, atualizar apenas os remotos indicados com `git fetch`, sem `pull`, merge ou checkout. Registrar quais referências e SHAs foram usados; informar quando a comparação usar referências locais sem atualização. Não interpolar entradas em shell.
3. Validar que ambas as referências apontam para commits e resolver o ancestral comum com `git rev-parse`/`git merge-base`. Comparar o ancestral comum com `head`, preservando também o SHA de `base`, para analisar as mudanças propostas sem incluir commits exclusivos da base.
4. Obter resumo e patch com `git diff --stat` e `git diff`; ler trechos adjacentes, chamadas e testes existentes necessários para entender o comportamento, com limites de tamanho. Não executar código da branch analisada durante a coleta.
5. Aplicar redaction antes de enviar código ao agente; sinalizar arquivos, patches ou contexto ausentes em vez de declarar a análise completa.

### GitHub

1. Buscar metadados da PR e arquivos alterados pela API REST.
2. Normalizar patches e SHAs para o contrato comum.
3. Publicar comentários apenas quando explicitamente solicitado.
4. Usar uma chave idempotente por achado para evitar comentários duplicados.

### GitLab

1. Receber `project ID/path` e `merge request IID` por invocação. Configurar a instância em JSON, sem URL ou versão específicas de qualquer organização:

   ```json
   {
     "gitlab": {
       "base_url": "https://gitlab.example.com",
       "version": "auto",
       "token_env": "GITLAB_TOKEN",
       "timeout_seconds": 15
     }
   }
   ```

   `base_url` é a origem da instância; o adaptador monta `/api/v4`. `version` aceita `auto` ou uma versão informada pelo usuário, como `12`. O arquivo guarda apenas o nome da variável do token, nunca o token.
2. Usar o SDK Go do GitLab como cliente REST v4, isolado no adaptador. Ele já expõe `ListMergeRequestDiffs` (`/diffs`) e `GetMergeRequestChanges` (`/changes`); não é necessário HTTP manual apenas para acessar a rota antiga.
3. Em modo `auto`, tentar `/diffs` e usar `/changes` apenas quando a resposta indicar que a rota não existe. Com versão 12 configurada, usar `/changes` diretamente. Erros de autenticação, autorização, rede ou timeout devem ser retornados, sem fallback. Paginar `/diffs` até concluir; tratar `overflow` e diffs incompletos em `/changes` como resultado parcial, não como revisão completa.
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

## Critérios da revisão

1. Entender o fluxo afetado e o comportamento da base antes de classificar um achado. Para cada problema, distinguir defeito já existente, regressão introduzida pela mudança e risco plausível ainda não confirmado.
2. Priorizar correção funcional e segurança: entrada não confiável, autenticação/autorização, exposição de dados, integridade, disponibilidade e efeitos em fluxos dependentes. Avaliar condições reais de disparo e alcance do impacto; não inferir severidade apenas pela aparência do diff.
3. Só afirmar que a mudança quebra um fluxo quando houver evidência verificável: caminho de execução, condição de entrada, comportamento antes/depois e `arquivo:linha`. Para um risco potencial, descrever a hipótese e a verificação que falta. Reportar defeitos preexistentes separadamente, sem atribuí-los à mudança.
4. Informar se a revisão ficou parcial por diff truncado, contexto ausente, falha de atualização ou ausência de testes relevantes. Testes só são executados quando existe implementação real e o ambiente permite execução segura.

## Entregas incrementais

1. Contratos, modelo normalizado e provider local com referências explícitas, atualização opcional de remotos e testes com repositório temporário.
2. CLI de análise local com saída Markdown/JSON e critérios de evidência, impacto e segurança.
3. Adaptador GitHub em modo leitura e `dry-run`.
4. Adaptador GitLab REST com configuração de URL, incluindo compatibilidade v12.
5. Publicadores opcionais, idempotência e comentários posicionais.
6. Integração CI para GitHub Actions e GitLab CI, sem exigir escrita no repositório por padrão.

## Verificação e riscos

- Testar diffs vazios, arquivos binários, renomeações, patches truncados, branches divergentes e erro de autenticação.
- Nunca executar o código da MR durante a coleta; a primeira versão apenas lê Git e APIs.
- Validar a API real da instância GitLab 12 antes de ativar publicação.
- Cobrir redaction e garantir que tokens não apareçam em relatórios, logs ou comentários.
