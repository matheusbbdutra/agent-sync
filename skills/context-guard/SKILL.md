---
name: context-guard
description: OBRIGATÓRIA em tarefas multi-etapa (3+ passos) e sessões longas. Guardião da janela de contexto que monitora degradação (drift, contradições, esquecimento no meio, lost in the middle), reancora as regras críticas após compaction e mantém checkpoint/handoff em STATE.md. SEMPRE carregue ao iniciar tarefa multi-etapa, ao chegar a ~40 passos, ao perceber drift/contradição/loop, após qualquer compaction ou quando a qualidade cair.
---

# Context Guard

> **Obrigatória por padrão:** se a tarefa tem 3+ passos ou a sessão está longa, esta skill deve ser carregada e o `STATE.md` mantido. Não é opcional.

Sessões longas **degradam em silêncio**: o agente começa a ignorar regras que seguia, contradiz decisões antigas e "esquece" o que está no **meio** da janela de contexto (*lost in the middle*). Compaction então descarta instruções sem avisar. Esta skill combate isso por **auto-disciplina comportamental**.

## Use this skill when

- Tarefa multi-etapa ou sessão que já passou de ~40 passos/ferramentas.
- O agente contradiz decisão anterior ou o estilo/nomenclatura começa a variar.
- Logo após um evento de **compaction**/resumo de contexto.
- Escopo crescendo sem limite ou qualidade da resposta caindo.

## Do not use this skill when

- Tarefa curta e direta (poucos passos), sem risco de drift.
- O problema é só reduzir tokens de uma leitura pontual → use `token-saving-toolkit`.

## Instructions

1. Estime a "zona" atual pelo volume e pelos sinais de drift.
2. Na zona amarela, faça checkpoint e **recite** as regras ativas.
3. Na zona vermelha, pare, verifique as regras e faça handoff para uma sessão nova.
4. Após compaction, **releia** as regras e revalide a próxima ação antes de executar.
5. Reduza contexto com as ferramentas de baixo token (abaixo).

## Zonas de saúde

| Zona | Sinal | Ação |
| --- | --- | --- |
| 🟢 Verde | início, poucos passos | operação normal |
| 🟡 Amarela | ~40 passos OU 1º sinal de drift | checkpoint + recitar + reduzir leituras exploratórias |
| 🔴 Vermelha | ~60 passos OU contradição/drift claro | parar, verificar regras, handoff, sugerir sessão nova |

> Contagens de passos são heurísticas: priorize os **sinais de drift** sobre o número.

## Sinais de drift (qualquer um → reancorar)

- Contradiz decisão tomada antes nesta sessão.
- Nomes/estilo/convenções divergem do início.
- Lê arquivo e "lembra" um conteúdo diferente do atual.
- Repete tentativa já refutada (loop).
- Ignora uma regra que seguia há pouco.
- Escopo cresce sem controle.

## Ancoragem anti-compaction

Quando houver compaction (perda súbita de contexto anterior):

1. **RELEIA** as regras/`global-rules` — não confie na memória.
2. **RECITE** em 1–2 linhas as 3–5 regras mais críticas ativas.
3. **VERIFIQUE** se a próxima ação respeita essas regras antes de executar.
4. Para qualquer decisão anterior, **RELEIA a fonte** (`STATE.md`, código, commit) — não suponha.
5. Reponha os fatos-chave no **fim** do raciocínio (recência), não no meio.

## Checkpoint e handoff (STATE.md)

- A cada mudança relevante de estado, mantenha `STATE.md` no projeto (use `templates/STATE.md` como base).
- Formato do checkpoint: **estado atual**, **decisões tomadas**, **arquivos tocados**, **próximos passos**, **bloqueios**.
- Ao atingir a zona vermelha: escreva o `STATE.md` e proponha **sessão nova** retomando por ele.
- `STATE.md` é a fonte de verdade para retomar — não confie no histórico da conversa.
- **Nunca copie segredos/credenciais pro `STATE.md`** (chaves de API, tokens, senhas) mesmo que apareçam na conversa — descreva o problema sem colar o valor. Diferente do `docs-cache` (que tem filtro automático de redação), a escrita do `STATE.md` é feita por você diretamente; a disciplina aqui é sua, não há guardrail de harness interceptando.

## Higiene de contexto (use estas ferramentas)

- `ast-outline <arquivo>` — estrutura em vez do arquivo inteiro.
- `trace-strip` — poda stack traces de frameworks/vendor.
- `docs-fetch -outline|-grep|-max` e `docs-mcp` — trechos de docs, não páginas inteiras.
- `db-guardian` — evita resultado gigante (`LIMIT`, colunas explícitas).
- Ao investigar, **cite o trecho** (`path:line`) em vez de colar blocos.

## Lembrete automático (hook)

- Se aparecer uma mensagem começando com `[agent-sync]` cobrando `context-guard`/`STATE.md`, ela vem do hook `hooks/context-guard-nudge.sh` (ou do plugin equivalente do OpenCode), disparado a cada N chamadas de ferramenta — não é o usuário nem uma alucinação. Aja conforme pedido (carregue esta skill, atualize `STATE.md`); não precisa repetir o checklist inteiro em voz alta, só seguir.

## Checklist

- [ ] Sei em que zona estou (verde/amarela/vermelha)?
- [ ] As regras ativas estão recitadas (início e fim)?
- [ ] Decisões e estado estão em `STATE.md`?
- [ ] Estou usando outline/grep/retrieval em vez de despejar arquivos?
- [ ] Após compaction, reli as regras e revalidei a ação?
- [ ] Há contradição/drift? Se sim, parei para reancorar?
