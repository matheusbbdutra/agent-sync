# ADR: AGENTS.md Session Guard com persistência condicional via `git check-ignore`

**Status**: Re-Proposto (data de revisão: 2026-10-23; D-64 em 2026-09-23)
**Data**: 2026-09-19
**Decisor**: Matheus Dutra
**Tags**: code-agent, agents-md, session-start, gitignore, context-bridge, hooks

## Contexto

Cada início de sessão em qualquer uma das 5 CLIs suportadas (Claude Code, Codex, Cursor, Antigravity, OpenCode) recomeça do zero: o agente não tem contexto persistente sobre o projeto — stack, entry points, paths críticos, convenções locais. Hooks existentes (`ctx-window` handoff, `repo-map` warm-up) cobrem **resumo da sessão anterior** e **index de símbolos**, mas nenhum provê **orientação estrutural inicial** sobre o projeto.

Em projetos sem README relevante (comum em legados), o agente gasta os primeiros turnos fazendo `tree`, `cat composer.json`, abrindo arquivos aleatórios. É o "Orientation Problem" que o ADR-repo-map ataca no nível de **símbolos**, mas que permanece aberto no nível de **estrutura de alto nível**.

**Drivers da decisão:**
- **Não poluir projeto versionado**: nenhuma alteração em arquivo commitado sem opt-in explícito
- **Respeitar governança**: empresas podem proibir AGENTS.md; configuração por ambiente/projeto
- **Reusar infraestrutura**: mecanismo `additionalContext` já wirado nas 5 CLIs pelo ADR-context-window-strategy
- **Sem dependência nova**: detecção via `git check-ignore` (built-in)

## Opções Consideradas

### Opção A (Escolhida): Detecção adaptativa + persistência condicional

Detecta via `git check-ignore -v .agent-sync/agents.md` se o arquivo **seria ignorado** na sessão atual. Se sim, persiste; se não, gera em memória e injeta via `additionalContext`. Oferece 1x (com dedup persistente em `.agent-sync/config.yaml`) setup de global gitignore.

**Prós:** zero alteração em arquivos versionados sem opt-in; zero dependência nova; adapta-se ao setup existente (projeto ou global); fallback gracioso.

**Contras:** comportamento dual (persist vs inject) aumenta superfície de teste; usuário sem global gitignore paga regen (~50ms/sessão).

### Opção B: Modificar `.gitignore` raiz do projeto

Adicionar `.agent-sync/*` + `!.agent-sync/session-state.json` automaticamente.

**Descartada porque** viola "não tocar arquivos versionados sem opt-in". Em projetos compartilhados vira commit sem valor que polui o repo dos outros colaboradores.

### Opção C: Inner `.agent-sync/.gitignore = *` committed

`agent-sync init` cria e commita o arquivo dentro de `.agent-sync/`.

**Descartada porque** o próprio agent-sync prova que o padrão é falho: seu `.gitignore` raiz lista `.agent-sync/.gitignore` (linha 27) como ignorado, então o "inner gitignore" não se propaga em clones frescos. O padrão é ilusório.

### Opção D: Persistência irrestrita (sem `git check-ignore`)

Escreve `.agent-sync/agents.md` sempre.

**Descartada porque** em projetos sem regra de ignore o arquivo aparece em `git status` como untracked, poluindo a visão do desenvolvedor.

### Opção E: Injection-only fixo (sempre em memória)

Nunca persiste.

**Descartada porque** perde a opção de edição manual entre sessões quando o setup do usuário já oferece cobertura, e paga regen desnecessário nesse caso.

## Decisão

Adotar a **Opção A**: hook de SessionStart que detecta via `git check-ignore` se o conteúdo pode ser persistido e adapta o comportamento.

### Fluxo

```
SessionStart (5 CLIs)
    ↓
git check-ignore -v .agent-sync/agents.md 2>/dev/null
    ↓
[exit 0 → SERIA ignorado]
    ↓
    .agent-sync/agents.md existe?
    ├─ sim + fresh (mtime ok) → ler + injetar via additionalContext
    ├─ sim + stale → perguntar "revisar? (y/n)" → regenerar
    └─ não → gerar + escrever + injetar
[exit 1 → NÃO SERIA ignorado]
    ↓
    .agent-sync/config.yaml tem opt-out de setup?
    ├─ sim → gerar em memória + injetar, não perguntar
    └─ não → perguntar 1x "setup global gitignore? (y/n)"
        ├─ y → oferecer 1-liner + injetar em memória nesta sessão
        └─ n → grava opt-out persistente → injetar em memória
```

### Componentes

| Componente | Localização | Função |
|---|---|---|
| Script bash core | `hooks/agents-md-session-guard.sh` | Scan, detecção, persistência condicional, injeção |
| Variante Cursor | `hooks/agents-md-session-guard.cursor.sh` | Adapta ao formato `additional_context` |
| Variante Antigravity | `hooks/agents-md-session-guard.antigravity.sh` | Adapta ao `ephemeralMessage` (PreInvocation) |
| Plugin OpenCode | `hooks/agents-md-session-guard.opencode.ts` | Adapta ao array `output.context` |
| Wiração Claude/Codex | `cmd/agent-sync/hooks.go` (`syncAgentsMdGuard`) | `SessionStart` com `additionalContext` |
| Wiração nas 5 CLIs | `agent-sync -apply` | Instalação automática |

### Configuração (`.agent-sync/config.yaml`)

```yaml
agents_md_guard:
  mode: prompt            # prompt | advisory | off
  setup_offer_dedup: true # oferece setup de global gitignore 1x
```

| Modo | Comportamento |
|---|---|
| `prompt` (default) | Pergunta antes de criar/revisar; pergunta 1x sobre setup global |
| `advisory` | Loga sugestões no stderr; nunca pergunta |
| `off` | No-op completo (exit 0, stderr vazio) |

### Detecção de stale (modo persist)

Compara mtime de `.agent-sync/agents.md` com:
- `composer.json` / `package.json` / `go.mod` (entry de dependências)
- Top-level files modificados nos últimos 7 dias (`find -mtime -7`)
- Se algum for mais novo → marca como stale

### Light scan para geração

~50ms. Lê: `tree -L 3` (1-2KB), `composer.json`/`go.mod`/`package.json` (500B), README se existir (primeiras 50 linhas), `index lookup` para entry points. Sem dependência de LSP ou AST.

## Consequências

### Positivas

- **Zero footprint por default**: nada em disco se usuário não quiser persistir
- **Auto-adapta**: aproveita qualquer setup de gitignore existente (projeto ou global)
- **Reuso de infra**: `additionalContext` já wirado nas 5 CLIs pelo ADR-context-window-strategy
- **Respeita governança**: modo `advisory` para empresas que proíbem
- **Sem dependência nova**: `git check-ignore` é built-in do git
- **Encaixa no padrão**: `.agent-sync/` (per-project, gitignored) é o mesmo escopo de `summary.md` (ADR-context-window) e `cache/repomap.json` (ADR-repo-map)

### Negativas / Riscos

- **Regen no modo injection-only**: ~50ms por SessionStart (aceitável, scan leve)
- **Comportamento dual (persist vs inject)** dificulta raciocínio sobre estado. **Mitigação**: log estruturado distingue "injected" de "persisted" em cada execução
- **Detecção de stale aproximada**: heurística de mtime não cobre 100%. **Mitigação**: oferecer regenerate manual no modo `prompt`
- **Falsa sensação de segurança**: usuário pode assumir persistência quando só houve injeção. **Mitigação**: log explícito + dry-run via `AGENT_SYNC_AGENTS_MD_DRY_RUN=1`

### Mitigação consolidada

- Log JSON por execução: `{mode, action: injected|persisted|skipped, source: scan|file}`
- Dry-run: `AGENT_SYNC_AGENTS_MD_DRY_RUN=1` mostra sem aplicar
- Modo `off` explícito para desativar sem remover hook

## Critérios de Aceite / Smoke

ADR passa de `Proposto` → `Implementado` quando:

1. Hook bash funciona em SessionStart das 5 CLIs sem erro
2. `git check-ignore` é chamado com sucesso (exit 0 quando arquivo é ignorado)
3. Em projeto SEM regra de ignore: gera em memória, injeta, não escreve nada em `.agent-sync/`
4. Em projeto COM global gitignore cobrindo `.agent-sync/`: persiste, e `git status` continua limpo
5. Modo `off`: hook no-op completo (exit 0, stderr vazio)

ADR passa de `Implementado` → `Aceito` após smoke planejado (`docs/SMOKE-TEST-S.md`):

| # | Cenário | Critério de pass |
|---|---|---|
| C1 | Projeto sem gitignore cobre | Injetou conteúdo; nenhum arquivo criado em `.agent-sync/`; `additionalContext` populado |
| C2 | Projeto com global gitignore simulado | Persistiu `.agent-sync/agents.md`; `git status` mostra 0 entries não-commitadas relacionadas |
| C3 | AGENTS.md stale (mtime antigo + deps novas) | Perguntou "revisar?"; não auto-aplicou |
| C4 | Modo `off` | Exit 0, stderr vazio, nenhum side effect |
| C5 | Opt-out persistente | Segunda execução não repete pergunta de setup global |

## Status Re-Proposto (2026-09-23, auditoria D-64)

Auditoria identificou que os hooks `hooks/agents-md-session-guard.sh` (e variantes `.cursor.sh`, `.antigravity.sh`, `.opencode.ts`) **não foram implementados** (`ls hooks/agents-md-session-guard*` retorna vazio). Único wirar parcial é o check de paridade `agents_md` em `internal/doctor/doctor.go:12` (escopo diferente: valida drift de `AGENTS.md`, não implementa o guard).

**Próximo passo concreto (próxima sprint):** implementar o hook bash core + variante Cursor (`followup_message` schema já documentado em `docs/ADR-fim-de-turno-hooks.md:106-111`), depois replicar para OpenCode v2 via plugin TS (mesmo padrão do `agent-react-nudge.opencode.ts`).

Revisão: 2026-10-23 (1 mês) — se hook não estiver wirado, mover para Descartado.

## Próximos Passos

1. Implementar `hooks/agents-md-session-guard.sh` core
2. Criar variantes por CLI (Cursor/Antigravity/OpenCode têm formato próprio de injeção)
3. Adicionar `syncAgentsMdGuard` em `cmd/agent-sync/hooks.go`
4. Criar `docs/SMOKE-TEST-S.md` com 5 cenários acima
5. Adicionar entradas D-N e A-N no `STATE.md` via `agent-sync state render`

## Referências

- ADR-repo-map-incremental-cache: ataca orientation problem no nível de **símbolos** (este ADR ataca no nível de **estrutura**)
- ADR-context-window-strategy: provê o mecanismo `additionalContext`/`ephemeralMessage`/`output.context` que este ADR reutiliza para as 5 CLIs
- ADR-002 (lockless default): padrão de escrita atômica + retry que será seguido pelo hook ao persistir `.agent-sync/agents.md`