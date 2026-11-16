# ADR: Skill única por domínio com detalhe em `reference/`

- **Status**: Aceito
- **Data**: 2026-11-16
- **Decisor**: Matheus Dutra
- **Tags**: skills, taxonomia, progressive-disclosure, anti-overengineering

---

## Contexto & Drivers

O repositório chegou a **57 skills**. A poda foi executada em três ondas, cada uma um commit
independente:

| Onda | Commit | Resultado |
|---|---|---|
| 1 | `e3c029e` | 57 → 45 |
| 2 | `1644013` | 45 → 37 (backend única de linguagem) |
| 3 | `8b69c02` | 37 → 32 (architecture única) |

Estado após esta ADR: **34 skills** (32 + `prompt-engineering` e `llm-eval`), das quais **25**
têm diretório `reference/`.

O problema que motivou a poda não era quantidade, era **granularidade errada**:

1. **Uma skill por framework/linguagem gerava stubs.** `php-pro`, `symfony`, `phpunit-symfony`,
   `doctrine` eram quatro gatilhos competindo pelo mesmo pedido ("trabalhe neste código PHP").
2. **Colisão de gatilho é mensurável e já é validada.** `internal/skills/lint.go:74` define
   `collisionThreshold = 0.25` de Jaccard entre `description`; acima disso o lint emite warning de
   misrouting. Quanto mais skills do mesmo domínio, mais provável a colisão.
3. **Conteúdo de referência não precisa de gatilho.** O detalhe técnico (padrões, checklists,
   trechos de código) não compete por ativação; ele é carregado sob demanda pelo `SKILL.md`.
   Manter isso em skill separada pagava o custo de gatilho sem entregar valor de roteamento.
4. **O corpo do repo já usava `reference/` singular.** Das 32 skills de então, **20** citavam
   `reference/` e apenas **2** citavam `references/`; o template (`templates/skill.md.tpl`) listava
   `references/` plural. A maioria já era o padrão.

## Opções Consideradas

- **Opção 1 (Escolhida): uma skill por domínio, detalhe em `reference/` singular.**
  Prós: um gatilho por domínio; sem colisão interna; progressive disclosure preservada; SKILL.md
  enxuto com o detalhe fora do caminho crítico.
  Contras: `SKILL.md` e `reference/` crescem (ex.: `backend`); exige disciplina de não citar
  arquivo inexistente.

- **Opção 2: manter uma skill por framework/linguagem (modelo anterior, 57 skills).**
  Descartada: cada skill era um stub; quatro gatilhos para o mesmo pedido; colisão de
  `description` acima do threshold do lint.

- **Opção 3: adotar `references/` plural para alinhar com o template.**
  Descartada: o corpo do repo já era singular em 20 das 32 skills. Padronizar no que era maioria
  custa menos do que migrar 20 skills para satisfazer um template.

## Decisão

1. **Um domínio = uma skill.** Quando duas skills competem pelo mesmo pedido, a menor é absorvida
   como `reference/<topico>.md` da maior, e a menor é removida.
2. **Subdiretório de detalhe é `reference/` (singular).** `references/` plural está proibido.
3. **`assets/` e `scripts/` só existem se o arquivo existir.** É proibido citar caminho que não
   está no repositório — o caso real que motivou a regra: 45 links mortos (20 em `references/*.md`
   e 25 em `assets/`+`scripts/`) herdados de skills vendorizadas, removidos no commit `efc24ce`.
4. **`description` é gatilho, não resumo.** Precisa distinguir a skill das vizinhas
   (Jaccard < 0.25) e conter padrão de gatilho aceito pelo lint
   (`use when|quando|para |<verbo de ação>`, `internal/skills/lint.go:33`).
5. **`SKILL.md` < 500 linhas**; o que passar disso vai para `reference/`.
6. **Ao absorver uma skill, remover as citações ao nome antigo** em títulos, seções e instruções
   de roteamento.

## Consequências & Trade-offs

**Positivas**

- 34 skills com um gatilho por domínio; `agent-sync skills lint` → 34 skills, 0 errors, 0 warnings.
- Zero links mortos para `reference(s)/*.md` e `assets/`/`scripts/` (verificado após `efc24ce`).
- Progressive disclosure preservada: o detalhe só entra no contexto quando o `SKILL.md` o aponta.
- Menos superfície para o modelo errar o roteamento.

**Negativas / Riscos**

- `reference/` grande pode não ser lido: o `SKILL.md` precisa apontar cada arquivo com uma linha
  do que ele contém, senão o detalhe fica inalcançável na prática.
- Absorção deixa rastro: `skills/backend/reference/{go,ts,php}.md` ainda têm o nome da skill antiga
  como H1 e como cabeçalho de seção (`# golang-pro` em `go.md:1`, `# typescript-pro` em `ts.md:1`,
  `# php-pro` em `php.md:1`, `## De symfony` em `php.md:61`, `## De doctrine` em `php.md:146`,
  `## De phpunit-symfony` em `php.md:231`, `## De javascript-pro` em `ts.md:51`,
  `## De nodejs-backend-patterns` em `ts.md:101`, `## De go-concurrency-patterns` em `go.md:174`).
  Pior: `php.md:76-77,86,144,229,246` instrui "use `php-pro`" / "use `doctrine`" / "ver
  `phpunit-symfony`" — skills que não existem mais.
- O template continua ensinando o padrão antigo: `templates/skill.md.tpl:33` lista `references/`.

**Mitigação**

- `templates/skill.md.tpl` corrigido para `reference/` nesta mesma decisão.
- A dívida do `backend/reference/` está listada abaixo com `file:line` para correção dirigida.
- Lint no CI é o gate: name, description com gatilho, colisão e leitura de `SKILL.md`.

## Critérios de Aceite / Smoke

- [x] `agent-sync skills lint` → 34 skills, 0 errors, 0 warnings.
- [x] Zero links `reference(s)/*.md` apontando para arquivo inexistente.
- [x] Zero links `assets/` e `scripts/` apontando para arquivo inexistente.
- [x] `templates/skill.md.tpl` usa `reference/` singular.
- [ ] Nenhuma skill cita skill inexistente em "Related Skills" — **aberto**: 8 nomes
      (`agentic-browser-testing`, `playwright-automation`, `qa-metrics`, `qa-project-context`,
      `release`, `risk-based-testing`, `test-planning`, `test-reliability`).
- [ ] Nomes antigos removidos de `skills/backend/reference/{go,ts,php}.md` — **aberto** (ver
      Consequências).
- [ ] Decisão registrada como `D-N` em `.agent-sync/session-state.json` via
      `agent-sync state write` + `state render` — **aberto**.

## Dívida conhecida (rastreada, não bloqueia a decisão)

| Arquivo | Linhas | Defeito |
|---|---|---|
| `skills/backend/reference/go.md` | 1, 174 | H1 e seção com nome de skill absorvida |
| `skills/backend/reference/ts.md` | 1, 51, 101 | idem |
| `skills/backend/reference/php.md` | 1, 61, 76-77, 86, 144, 146, 229, 231, 246, 315 | idem + instruções de roteamento para skill inexistente |
| `skills/*/SKILL.md` ("Related Skills") | — | 8 skills citadas que não existem no repo |

Não confundir com falso positivo: `skills/docs-research/SKILL.md:47-50` cita `symfony` e `doctrine`
como **URLs de documentação** (`symfony.com/doc`, `doctrine-project.org`), não como skills.
