---
name: agent-delegate
description: Critérios para decidir se e para qual CLI/modelo delegar uma tarefa (Claude Code, Codex, Antigravity/agy, OpenCode), usando o modo não-interativo de cada um e a memória compartilhada (`memory` MCP). Use quando o usuário pedir para "mandar isso pra outro agente/modelo", ao avaliar se uma tarefa é barata/mecânica o suficiente para rodar num modelo mais econômico, ou ao decidir se vale delegar em vez de executar você mesmo.
---

# Delegação entre agentes/CLIs

Responda em PT-BR, objetivo (CLAUDE.md global). Esta skill não cria uma orquestração automática — é um checklist para decidir manualmente se/para onde delegar. A infra de suporte é a memória compartilhada (`memory-mcp`, ver `tools/cmd/memory-mcp`) mais os modos não-interativos que cada CLI já expõe:

- Claude Code: `claude -p "<prompt>"`
- Antigravity (agy): `agy --print "<prompt>"` (ou `-p`)
- OpenCode: `opencode run "<mensagem>"`

Qualquer CLI pode chamar qualquer outro via `Bash`/subprocess — é simétrico em teoria (agy → opencode, opencode → claude, claude → agy, etc.), não é uma via de mão única "Claude delega pro barato". **Na prática, Claude Code → agy tem uma limitação conhecida** (ver seção abaixo) — as demais direções não apresentaram esse problema nos testes feitos.

## Limitação conhecida: Claude Code não consegue orquestrar `agy` em modo headless

Testado em 2026-09-12: delegar do Claude Code para `agy --print` esbarra no classificador de segurança do próprio harness do Claude Code ("Create Unsafe Agents"), não em uma limitação do `agy` ou do `memory-mcp`.

- `agy --print` sem nenhuma flag especial falha sozinho: modo headless não consegue aprovar permissões de ferramenta (MCP, escrita de arquivo, comando) interativamente, e nega tudo por padrão.
- `--dangerously-skip-permissions` e `--mode accept-edits` são recusados pelo classificador do Claude Code antes mesmo de chegar ao `agy` — é uma concessão de capacidade ampla demais para um agente autônomo, e isso é bloqueado independente de confirmação do usuário no chat.
- Escopar a permissão no `settings.json` do agy (`mcp(memory/*)`, `write_file(/caminho/especifico/*)`, `read_file(/caminho/especifico/*)`) **funcionou uma vez** (permissão restrita a um diretório específico, não `*` global).
- Mas: (a) qualquer permissão de `command(...)` com argumento livre (ex.: `command(python3 *)`, necessário pro agy verificar o próprio script rodando) continua sendo recusada, e com razão — é execução arbitrária de comando; e (b) em tentativas repetidas na mesma sessão, o classificador passou a bloquear até a variante já validada como seguro, sugerindo que ele também pondera o padrão de insistência, não só o conteúdo de cada comando isoladamente.

**Conclusão prática:** não tente automatizar Claude Code → agy via `Bash`/subprocess sem que o usuário aprove manualmente as permissões do agy primeiro (rodando `agy` interativo uma vez). Não insista tentando variações de flags/regras de permissão numa mesma sessão — isso é reconhecidamente um padrão que o próprio classificador escala para bloqueio mais amplo. Se o usuário quiser essa comparação, oriente-o a rodar `agy` interativamente e aprovar os prompts uma vez; depois disso, `agy --print` deve funcionar sem intervenção.

**Direções que funcionaram sem problema:** Claude Code → OpenCode (`opencode run`), testado de ponta a ponta com sucesso (leitura de `memory-mcp` + execução + verificação real do resultado).

## Antes de delegar: consultar a memória compartilhada

Sempre que for delegar, primeiro consulte o `memory-mcp` (`search_memory`/`list_memories`) pelo contexto/decisões já existentes relevantes à tarefa, e inclua isso no prompt que você vai passar. Delegar sem esse contexto joga fora o ganho principal: o agente-alvo (especialmente um modelo mais barato) reconstrói do zero algo que já foi decidido, com risco de contradizer uma decisão registrada.

## Critérios para decidir se delega

1. **Risco/reversibilidade da tarefa.** Mecânica e fácil de verificar (boilerplate, busca, resumo, formatação, teste repetitivo) → pode ir para um modelo mais barato. Decisão de arquitetura, segurança, ou algo difícil de auditar depois → mantenha no modelo/CLI que já está conduzindo a sessão.
2. **Custo de verificação.** Só delegue se checar o resultado for mais barato do que você mesmo ter feito a tarefa. Se validar a saída exige o mesmo cuidado de tê-la escrito, não há ganho.
3. **Capacidade necessária.** Alguns CLIs têm ferramentas que outros não têm (ex.: automação de browser só num deles). Isso restringe o alvo possível, independente de custo.
4. **Contexto disponível.** Só delegue se a memória compartilhada já tiver o suficiente para o agente-alvo não precisar "descobrir" algo caro sozinho — senão a delegação não economiza nada.
5. **Escolha explícita, não heurística automática.** Quem inicia a delegação (você ou o usuário) escolhe o CLI/modelo-alvo na hora, usando os critérios acima como checklist mental. Não tente automatizar "qual modelo pra qual tarefa" com regras codificadas — é over-engineering para uso pessoal e esconde a decisão de quem deveria fazê-la.

## Apagar memória: só o que foi marcado como descartável

`delete_memory` só remove memórias gravadas com `scratch: true` no `store_memory` — memórias permanentes (decisões, feedback, contexto de projeto) são recusadas por design, mesmo que pareçam irrelevantes depois. Se algo realmente precisar sair, isso é uma ação manual deliberada (editar o `.db` direto), não uma chamada de ferramenta que qualquer agente delegado poderia disparar. Ao gravar algo que é só teste/rascunho/experimento de uma delegação, marque `scratch: true` desde o início para poder limpar depois sem fricção.

## Proveniência e confiança

Ao gravar uma memória vinda de uma tarefa delegada, registre o campo `agent` corretamente no `memory-mcp` (`store_memory`). Uma memória gravada por um modelo mais barato pede mais cautela ao ser reutilizada depois — não é motivo para não gravar, é motivo para, ao reler, considerar se vale reverificar antes de tratar como fato.

## Fluxo de uma delegação

1. Consultar `memory-mcp` pelo contexto relevante (`search_memory`).
2. Montar o prompt já embutindo esse contexto (não assumir que o agente-alvo vai buscar sozinho).
3. Rodar via `Bash` o modo não-interativo do CLI/modelo escolhido.
4. Revisar o resultado antes de aceitar como pronto — principalmente se veio de modelo mais barato e a tarefa não é puramente mecânica.
5. Se o resultado gerar uma decisão/aprendizado que vale persistir, gravar de volta no `memory-mcp` com a proveniência correta.

## Quando NÃO delegar

- A tarefa é pequena o suficiente que delegar (montar prompt, rodar subprocesso, revisar saída) custa mais do que fazer direto.
- A tarefa envolve decisão irreversível ou sensível (segredos, ações destrutivas, mudanças de arquitetura) — mantenha no fluxo normal de confirmação com o usuário, não terceirize a decisão em si para outro modelo.
- Não há memória/contexto suficiente registrado e buscá-lo/produzi-lo já seria caro — nesse caso, resolva a lacuna de contexto primeiro (você mesmo), antes de sequer cogitar delegar.
