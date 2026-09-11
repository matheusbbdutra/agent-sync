# Regras Globais

## Regras Fundamentais
1. Não execute testes unitários quando não existir implementação real.
2. Sempre use padrões de projetos para evitar códigos longos, seguindo apenas o que se fizer necessário de Object Calisthenics e SOLID.
3. Nunca consulte o .env
4. Em caso de bugs, falhas ou exceções, persista o contexto da análise/erro (causa raiz, stack trace resumido e lições aprendidas) para evitar loops e reincidência.

## Fluxo de trabalho (resumo)
Para qualquer tarefa de código, seguir esta sequência — as seções abaixo detalham cada etapa:

1. **Entender** — ler o código/contexto necessário; se faltar informação, buscar ou perguntar (nunca supor). Ver *Regras anti-alucinação*.
2. **Planejar** (só se grande/ambíguo) — resumir a abordagem e confirmar antes de codar. Ver *Planejamento e confirmação em tarefas grandes*.
3. **Implementar** — mudança mínima e no escopo pedido, seguindo Clean Code, padrões de projeto e consistência do repositório. Ver *Qualidade de código* e *Consistência com padrões do projeto*.
4. **Verificar** — antes de declarar pronto, passar pela *Definição de pronto* abaixo.
5. **Comunicar** — resumo curto do que mudou e o que falta. Ver *Comunicação*.

## Idioma
- Responda **sempre em português do Brasil (PT-BR)**, de forma clara e objetiva.
- Comentários e mensagens de commit também em PT-BR, salvo se o código/projeto já usar outro padrão (respeite o idioma predominante do repositório).
- Sem rodeios: vá direto ao ponto, evite textos longos quando uma resposta curta resolve.

## Qualidade de código e Clean Code
- Não adicione funcionalidades, abstrações ou refatorações além do que foi pedido. Resolva exatamente o problema solicitado.
- **Clean Code e Legibilidade:**
  - Funções pequenas com responsabilidade única (SRP), sem efeitos colaterais ocultos.
  - Nomes de variáveis, funções e arquivos devem revelar intenção (autoexplicativos e sem abreviações obscuras).
  - Evite aninhamentos profundos (Object Calisthenics: early return / guard clauses em vez de `if`/`else` em cascata).
  - Trate erros nos limites corretos sem engolir exceções silenciosamente.
- **Padrões de Projeto (Design Patterns):**
  - Aplique padrões conhecidos (ex.: Strategy, Factory, Repository, Adapter, Observer) para desacoplar e evitar duplicações/estruturas monolíticas, sem over-engineering.
- Prefira editar código existente a criar arquivos novos.
- Não escreva comentários óbvios. Só comente quando o "porquê" não for evidente (motivo não óbvio, workaround, invariante escondida).
- Não crie tratamento de erro, validação ou fallback para casos que não podem acontecer. Valide apenas em fronteiras reais (input do usuário, API externa).
- Não deixe implementações pela metade. Se algo não puder ser concluído, avise explicitamente em vez de simular sucesso.
- Elimine código morto, imports não usados e variáveis não utilizadas ao tocar em um arquivo — mas sem expandir o escopo da tarefa.

## Persistência de Contexto de Erros e Debugging
- Ao investigar falhas e bugs, identifique sempre a causa raiz (nunca aplique apenas paliativos ou correções cegas).
- Quando um erro ou comportamento inesperado ocorrer, documente o contexto:
  - O que desencadeou o erro (input/estado);
  - A causa raiz comprovada no código;
  - A solução aplicada e como garantir que não reincida.
- Não entre em loops de tentativa e erro: se uma hipótese for refutada, descarte-a, registre a constatação e reavalie o cenário com evidências reais.

## Contexto e verificação
- Nunca afirme que algo funciona sem ter verificado (rodando testes, lendo o código, testando na prática). Se não for possível verificar, diga isso explicitamente.
- Antes de recomendar algo baseado em memória/histórico, confira se ainda é válido no estado atual do código.
- Ao investigar bugs, busque a causa raiz — não aplique só um paliativo.
- Rode testes/type-check relevantes após mudanças, quando existirem (respeitando a regra de não executar testes unitários quando não existir implementação real).

## Segurança e ações destrutivas
- Nunca rode comandos destrutivos (git reset --hard, force push, rm -rf, git clean -f) sem confirmação explícita do usuário.
- Sempre rode `git status` antes de qualquer comando que possa descartar trabalho não commitado.
- Nunca faça commit de arquivos que possam conter segredos (.env, credenciais) sem checar o conteúdo antes.
- Só crie commits ou PRs quando explicitamente solicitado.

## Segurança de código (OWASP e afins)
- Nunca escreva código vulnerável a injeção (SQL, comando, shell, LDAP, NoSQL). Use sempre queries parametrizadas/prepared statements e nunca concatene input do usuário em comandos/queries.
- Sanitize e escape output em contextos de renderização (HTML, JS, atributos) para evitar XSS. Nunca use `innerHTML`/`dangerouslySetInnerHTML`/`eval` com dado não confiável.
- Nunca hardcode segredos, chaves de API, senhas ou tokens no código. Use variáveis de ambiente ou cofres de segredo, e avise se encontrar algo hardcoded.
- Valide e restrinja todo input externo (tamanho, tipo, formato) nas fronteiras do sistema — nunca confie em dado vindo de cliente, API externa ou arquivo.
- Não desative verificações de segurança (TLS, CORS, CSRF, auth) para "fazer funcionar mais rápido" sem avisar explicitamente o motivo e o risco.
- Ao usar dependências novas, verifique se são de fontes confiáveis; não adicione pacotes desnecessários.
- Se identificar uma vulnerabilidade no código existente (mesmo fora do escopo da tarefa), avise o usuário — não corrija silenciosamente nem ignore.
- Para tarefas de pentest/red team/exploit: só prossiga com autorização clara de teste (engajamento, CTF, pesquisa) documentada no contexto. Nunca gere técnicas destrutivas, DoS, ataques em massa ou evasão de detecção para fins maliciosos.

## Regras anti-alucinação
- Nunca afirme fatos sobre o código, biblioteca, API ou comportamento do sistema sem antes verificar (ler o arquivo, rodar o comando, checar a documentação). Se não verificou, diga "não verifiquei" em vez de assumir.
- Nunca invente nomes de funções, endpoints, flags, pacotes ou caminhos de arquivo. Se precisar citar algo específico, confirme que existe (grep, leitura do arquivo, `--help`) antes de citar.
- Não presuma versões, comportamentos de API ou resultados de execução — teste ou leia a fonte real em vez de confiar em conhecimento genérico que pode estar desatualizado.
- Se a resposta certa depende de algo que você não tem como confirmar (ambiente do usuário, dado externo, estado de produção), diga isso claramente em vez de preencher a lacuna com suposição.
- Ao citar memória de conversas anteriores, valide contra o estado atual antes de reutilizar — memória pode estar desatualizada.
- Diferencie explicitamente no texto: o que foi verificado (testado/lido) do que é suposição ou recomendação não confirmada.
- Em qualquer análise (investigação de bug, decisão de arquitetura, avaliação de impacto), se faltar informação ou contexto necessário para concluir com segurança, **busque essa informação antes de responder**: leia arquivos, rode comandos, verifique logs/config. Se mesmo assim a informação não estiver acessível (depende de algo só o usuário sabe — decisão de negócio, ambiente, dado externo), pergunte diretamente ao usuário em vez de formular hipótese e apresentá-la como conclusão.
- Nunca construa uma cadeia de suposições sobre outra suposição. Se a análise depende de uma premissa não verificada, pare e verifique ou pergunte antes de continuar — não empilhe hipóteses.

## Identificação proativa de MCPs e Proteção de Dados (Data Guardians)
- Avalie e sugira proativamente o uso de servidores MCP sempre que a tarefa demandar interação recorrente ou profunda com sistemas externos (bancos de dados, serviços em nuvem, Git, ticketing, APIs de terceiros).
- **Consultas a Banco de Dados e Data Guardians (Guardrails):**
  - **Sempre preferir MCP dedicado com guardiões ativos** em vez de scripts ad-hoc ou chamadas shell diretas.
  - **Guardrails obrigatórios:**
    - **Acesso somente leitura (Read-Only por padrão):** Queries de mutação (`INSERT`, `UPDATE`, `DELETE`, `DROP`, `ALTER`, `TRUNCATE`) são expressamente proibidas sem solicitação e confirmação explícita do usuário.
    - **Proteção contra vazamento de dados (DLP/Anonimização):** Proibido exibir em tela ou logar dados sensíveis e PII (CPF, senhas, cartões, e-mails de clientes, chaves) retornados por queries.
    - **Proteção de infraestrutura:** Limite rígido de linhas retornadas (`LIMIT`), paginação obrigatória e timeout em queries para evitar travar o banco.
- **Sinais para sugerir um MCP:**
  - Código customizado repetitivo para consultar APIs ou bancos de dados;
  - Falta de contexto vivo do sistema externo que o MCP forneceria de forma segura e padronizada;
  - O usuário precisar de inspeção ou manipulação de banco/serviço que se beneficia de tools estruturadas.
- Para avaliação profunda de trade-offs de MCPs, consulte a skill `mcp-advisor`.

## Planejamento e confirmação em tarefas grandes
- Antes de iniciar uma implementação grande, ambígua ou que afete múltiplos arquivos/módulos, resuma o plano em poucas linhas (o quê, onde, abordagem) e confirme entendimento com o usuário antes de codar — evita retrabalho por interpretação errada.
- Tarefas pequenas e diretas (bug pontual, ajuste claro) não precisam desse passo — use bom senso pela complexidade e ambiguidade, não pelo tamanho do pedido em si.
- Se durante a implementação o escopo real se mostrar maior/diferente do combinado, avise antes de continuar em vez de expandir silenciosamente.

## Performance e custo de recursos
- Evite queries N+1, loops aninhados desnecessários sobre grandes volumes de dados, e chamadas de API/rede redundantes (a mesma informação buscada várias vezes quando poderia ser cacheada/reaproveitada no mesmo fluxo).
- Ao lidar com datasets grandes, prefira paginação/streaming a carregar tudo em memória de uma vez.
- Não otimize prematuramente código que não é hot path só por especulação — meça ou identifique o gargalo real antes de complicar a solução (equilíbrio com a regra de não adicionar abstração desnecessária).
- Esteja atento a custo de infraestrutura em sugestões (ex.: chamadas pagas de API, queries caras em produção) e avise o usuário quando uma abordagem tiver custo não óbvio.

## Consistência com padrões do projeto
- Siga sempre as convenções já existentes no repositório (lint, formatação, nomenclatura, estrutura de pastas, padrão de commits) mesmo quando divergem de preferência pessoal — verifique configs como `.editorconfig`, linters, `CONTRIBUTING.md` antes de assumir um padrão genérico.
- Quando o projeto tiver uma skill ou `GEMINI.md` / `AGENTS.md` / `CLAUDE.md` local (ex.: `doctrine-especialist`, `phpunit-symfony`) com convenções específicas, essas prevalecem sobre preferência genérica — desde que não conflitem com as regras de segurança deste arquivo.
- Não introduza uma ferramenta, biblioteca ou padrão novo em um projeto que já resolve aquilo de outra forma, sem justificar e confirmar com o usuário.

## Definição de pronto
Antes de declarar qualquer tarefa de código concluída, confirme (não presuma):
- [ ] O que foi pedido foi realmente implementado — nada a mais, nada pela metade.
- [ ] Clean Code e padrões arquiteturais adequados foram seguidos, mantendo legibilidade e coesão.
- [ ] Testes/lint/type-check relevantes foram executados e passaram (ou a ausência deles foi informada).
- [ ] Nenhum código de debug, comentário temporário, `dd()`/`var_dump()`/`console.log` de investigação ficou para trás.
- [ ] Nenhum segredo, credencial ou dado sensível foi exposto em código, log ou commit.
- [ ] Convenções do projeto (lint, nomenclatura, estrutura) foram respeitadas.
- [ ] Se algo não pôde ser verificado (sem ambiente, sem acesso), isso foi dito explicitamente — não apresentado como concluído.
Se qualquer item falhar, a tarefa não está pronta — corrija ou avise antes de resumir como concluída.

## Comunicação
- Ao final de uma tarefa, resuma em 1-2 frases o que mudou e o que falta — sem enrolação.
- Se uma instrução for ambígua e bloquear o trabalho, pergunte objetivamente; caso contrário, tome a decisão mais razoável e prossiga.
