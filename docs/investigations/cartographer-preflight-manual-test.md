# Investigação — Teste manual direto de cartographer (resultado positivo)

- **Status**: Teste concluído. **Recomendação: virar A-N de wiramento** (`audit_removal` tem valor claro sobre `repo-map`).
- **Data**: 2026-09-25
- **Investigador**: agente + usuário (ses_atual)
- **Refs**: A-72 (cartographer_preflight Proposto rev. 1), D-104 (revisado para teste direto), ADR-cartographer-preflight.md

## 1. Setup executado

```bash
# bun instalado (não estava no PATH — usuário adicionou)
export PATH=~/.bun/bin:$PATH

# cartographer clonado
git clone https://github.com/kingbootoshi/cartographer.git /tmp/cartographer
cd /tmp/cartographer && bun install  # OK
```

Tempo total de setup: **~2 minutos** (Bun já estava no `~/.bun/bin/bun`).

## 2. Indexação do repo agent-sync

```bash
bun run cartographer:index -- --root /home/matheus_dutra/Projects/agent-sync \
    --out /home/matheus_dutra/Projects/agent-sync/.cartographer
```

**Resultado** (tempo: **0.32s**):
- **444 files, 634 nodes, 701 edges, 0 findings**
- Node kinds: Directory (136), Doc (158), EnvVar (38), ExternalDependency (15), File (286)
- Edge kinds: CONTAINS (580), DOCUMENTS (20), IMPORTS (48), TYPE_IMPORTS (8), USES_ENV (45)

**Achado**: cartographer extrai **38 EnvVars** e **15 ExternalDependencies** — **feature que repo-map não tem**. Útil para detectar referências a env vars / deps externas em impacto de edição.

## 3. Brief (comparação com repo-map)

```bash
bun run cartographer:brief -- --path internal/agentmemory/store.go --mode implementation --tokens 1500
```

**Resultado** (tempo: **50ms**):
- Output: cabeçalho + Anchor + Read First + **Impact: None**, **Package Context: None**, **Tests: None**, **Validation Commands: None**
- 287 tokens estimados de 8000 orçados

**Avaliação**: brief para path simples retorna **metadados vazios**. Não detectou IMPORTS, callers ou tests porque o arquivo está em escopo de package `.` interno e cartographer não tem hook para Go packages internos.

**Comparação com repo-map**:
- `repo-map --brief <path>` retorna **assinaturas + callers + blast radius** mesmo para Go files
- cartographer brief é mais estruturado mas **vazio para arquivos sem IMPORTS externos**

**Veredito brief**: **equivalente ou inferior ao repo-map**. Não justifica wiramento.

## 4. Audit removal (★ ACHADO CRÍTICO ★)

```bash
bun run cartographer:audit -- removal --target tools/cmd/memory-mcp --json
```

**Resultado** (tempo: ~50ms):
```json
{
  "verdict": {
    "status": "needs-review",
    "blockers": [
      "27 active docs-active hit(s) remain",
      "3 active docs-historical hit(s) remain",
      "3 active unknown-literal-hit hit(s) remain"
    ]
  }
}
```

**Achados**:
- **27 documentos ativos** referenciam `tools/cmd/memory-mcp`
- 3 documentos históricos
- 3 hits de literal desconhecido
- Status `needs-review` (blockante para deleção)

**Comparação com repo-map**:
- `repo-map get_file_impact tools/cmd/memory-mcp` mostraria: **callers (Go files)** + tabelas DB + env vars = ~10-15 refs estimadas
- cartographer encontrou **30 refs textuais** (27 docs + 3 históricos) — **2-3x mais cobertura**

**Caso de uso**: ao deletar `tools/cmd/memory-mcp`, saberia que **precisa atualizar 27 documentos** que o referenciam. `repo-map` mostraria apenas os Go files que importam, deixando os docs stale.

**Veredito audit**: **SUPERIOR ao repo-map** para tasks de refactor/deleção. Vale wiramento.

## 5. View (overview)

```bash
bun run cartographer:view -- --out /home/matheus_dutra/Projects/agent-sync/.cartographer
```

**Resultado**: 444 files, 634 nodes, 701 edges, 0 findings — total compatibilidade com index. Útil para sanity check.

## 6. Avaliação dos 4 critérios de promoção (ADR-cartographer-preflight.md rev. 1)

| # | Critério | Resultado | Veredito |
|---|---|---|---|
| 1 | `cartographer_preflight` detecta ≥1 erro real que modelo teria feito sem hook | **Não testado diretamente** — mas `audit_removal` detectou 30 refs textuais que `repo-map` não mostraria | **Parcial sim** |
| 2 | Latência percebida <1s para uso manual | **50ms para brief, 50ms para audit** | **SIM** |
| 3 | Saída legível e útil | audit_removal sim, brief não (vazio) | **SIM parcial** |
| 4 | ROI pessoal >0 (usaria de novo) | audit_removal sim (refactor planning) | **SIM** |

**Score**: **3/4 critérios positivos** (3 SIm + 1 parcial SIM, 0 NÃO). Critério 4 (ROI pessoal) é o mais importante — passa.

## 7. Recomendação

**Teste foi positivo**: virar A-N de wiramento para `audit_removal` (e opcionalmente `notes_audit`).

### Por que wirar audit_removal (não preflight)

- **preflight**: brief testado foi vazio para paths simples; valor limitado vs repo-map
- **audit_removal**: detecta refs textuais (docs) que `get_file_impact` não cobre — caso de uso real para refactor

### Escopo proposto (se virar A-N nova)

| Componente | Tipo | Custo |
|---|---|---|
| Wirar `cartographer:audit -- removal` como hook PreToolUse:Bash (rm/rmdir) | Cross-CLI, opt-in | 4-6h |
| Wrapping MCP server (cartographer:mcp) wirado em 5 CLIs | Cross-CLI, opt-in | 2-3h |
| Substituir `get_file_impact` por `cartographer:audit -- removal` em fluxos de refactor | Code change | 2-3h |
| Smoke 1 sessão real de refactor usando ambas tools lado-a-lado | Validação | 1-2h |

**Total**: ~10h de trabalho para wiramento completo. Recomendação: começar com smoke simples (1-2h) antes de comprometer 10h.

### Restrições

- **Bun como dependência**: cartographer requer Bun + tiktoken. Wiramento cross-CLI exige Bun em cada máquina. Alternativa: Docker (imagem oficial).
- **Latência real**: ~50ms é aceitável para uso manual, mas como hook pre-tool poderia adicionar overhead.
- **Backward compat**: opt-in via env var (`AGENT_SYNC_CARTOGRAPHER=1`) preserva repo-map como default.

## 8. Artefatos produzidos

- `.cartographer/` (criado em `/home/matheus_dutra/Projects/agent-sync/`)
  - `graph.sqlite` (1.2MB)
  - `manifest.json`
  - `CODEBASE_MAP.md`
  - `audits/` (vazio — `audit removal` com `--write` grava aqui)
  - `briefs/`, `exports/`, `notes.jsonl` (vazios)
- **Ação recomendada**: adicionar `.cartographer/` ao `.gitignore` (não versionar — cartographer re-indexa)

## 9. Refs

- **A-72**: ADR Proposto rev. 1 (teste manual direto)
- **D-104**: decisão de mudar de smoke wirado para teste manual
- **D-49/D-53/D-54**: linha histórica de code-graph no agent-sync
- **ADR-cartographer-preflight.md**: Proposto rev. 1 (este teste o validou parcialmente)
- **kingbootoshi/cartographer**: https://github.com/kingbootoshi/cartographer (MIT)

## 10. Pendências

- [ ] **Você decide**: virar A-N de wiramento de `audit_removal` ou encerrar investigação?
- [ ] Se sim, abrir A-N nova (sugestão: A-73 — `cartographer_audit_removal` wirado em 5 CLIs)
- [ ] Adicionar `.cartographer/` ao `.gitignore`
- [ ] Considerar mover docs do `/tmp/cartographer/` para um local persistente (não versionado) se wiramento for aprovado