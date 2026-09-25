# Agent ReAct — referência

## Ciclo bom (exemplo)

**Goal:** achar por que `syncSkills` não cria symlink no Cursor.

```text
Thought: Hipótese (não comprovada) — destino ~/.cursor/skills-cursor está protegido e o sync pula. Validar: Grep isProtectedSkillsDir.
Action: Grep isProtectedSkillsDir em cmd/agent-sync/
Observation: path_test.go marca skills-cursor como protegido; SkillsDir do Cursor é ~/.cursor/skills.
Update: Verificado que skills-cursor é protegido. Hipótese “apply pula Cursor” ainda pendente → validar com agent-sync -status / log do apply.
```

## Hipótese com validação (bom vs ruim)

**Bom (agente valida):**
> Hipótese: o binário em `~/.local/bin` é antigo e não tem alvo Cursor. Vou checar com `agent-sync -status` e a data do binário.

**Bom (usuário valida):**
> Hipótese: o deploy em produção ainda aponta para a imagem antiga. Não tenho acesso ao cluster daí. Para validar, rode: `kubectl get deploy … -o jsonpath='{.spec.template.spec.containers[0].image}'` e cole a saída.

**Ruim:**
> O problema é o binário antigo no PATH — por isso o Cursor não sincroniza. (afirmado como fato, sem Action nem pedido ao user)

## Ciclo ruim (anti-padrões)

| Anti-padrão | Por quê |
| --- | --- |
| Thought de 20 linhas + 5 tools de uma vez | Mistura hipóteses; Observation fica ambígua |
| Repetir `Read` do mesmo arquivo “para confirmar” | Loop; se já leu, cite `path:line` |
| “Provavelmente é X” sem Action nem pedido ao user | Hipótese vendida como fato; falta Observation |
| Cadeia A→B→C sem validar A | Empilhamento de suposições |
| Implementar com base só em hipótese | Corrige o problema errado |
| Reabrir hipótese descartada sem fato novo | Desperdício de budget |
| Continuar após 3 ciclos sem progresso | Escalada cega; deve parar e perguntar |

## Quando paralelizar Actions

OK em paralelo só se forem **independentes** (ex.: `git status` + `git log` + `git diff` no início de um commit).

Não paralelize se B depende do resultado de A (ex.: achar arquivo → ler arquivo).

## Checkpoint mínimo (alinhar com STATE.md)

```text
Subtask: …
Proven: …
Refuted: …
Blocked: …
Next action: …
```

Grave no `STATE.md` / `memory-mcp` quando a tarefa já for multi-etapa ou o budget estiver alto — não a cada ciclo curto.
