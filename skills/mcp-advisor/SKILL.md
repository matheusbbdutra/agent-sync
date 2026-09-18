---
name: mcp-advisor
description: "Avalia necessidade de usar ou construir servidores MCP para integração segura de sistemas."
---

# Quando sugerir um MCP em vez de código customizado

Responda em PT-BR, objetivo (CLAUDE.md global). O objetivo aqui é **avaliar**, não empurrar MCP para tudo — muita integração simples e pontual continua sendo mais barata como código direto.

## Sinais de que vale um MCP

- A integração é **repetida** ao longo do projeto/sessões (mesma API sendo chamada em vários pontos), não uma chamada única e isolada.
- O sistema externo muda com frequência (dados ao vivo: produção, tickets, documentos, dashboards) e o agente precisaria "reconsultar" em conversas futuras — código customizado não persiste esse acesso entre sessões, um MCP sim.
- Já existe um MCP conhecido e mantido para esse sistema (ex.: banco de dados, Slack, GitHub, ferramentas de nuvem) — reinventar client custom é retrabalho e superfície extra de bugs.
- A tarefa precisa de ação autenticada e recorrente em um serviço de terceiros (não só um GET público ocasional).

## Sinais de que NÃO vale (ficar com código direto)

- É uma chamada de API única, dentro do escopo de uma feature específica do próprio produto do usuário (ex.: o app do cliente chamando a própria API de pagamento) — isso é código de produto, não tooling do agente.
- Não existe MCP para o sistema em questão e criar um do zero seria desproporcional ao valor (uma automação pontual não justifica manter um servidor MCP).
- O acesso é simples, estável e não precisa ser reusado entre conversas (ex.: ler um arquivo CSV local uma vez).

## Como conduzir a avaliação

1. Identifique o sistema externo envolvido e se é algo que reaparecerá em outras tarefas.
2. Confirme se já existe um MCP relevante — pergunte ao usuário quais MCPs ele já tem configurados (`ListAgents`/config não mostram isso diretamente; pergunte ou verifique `.claude/settings.json`/config MCP do projeto) antes de sugerir instalar um novo.
3. Se não houver MCP e a integração for recorrente, apresente a opção como sugestão curta — não decida sozinho por instalar/configurar nada sem aprovação (mudança de configuração conta como ação que precisa de confirmação, conforme CLAUDE.md).
4. Se for código de produto do próprio usuário (não tooling do agente), não sugira MCP — isso é escopo de aplicação, não de assistente.

## Formato da sugestão

Seja direto: nome do sistema, por que um MCP ajudaria mais que código custom, e pergunte se o usuário quer que você configure/verifique isso — não implemente a integração custom "de qualquer jeito" e depois mencione o MCP como nota de rodapé.
