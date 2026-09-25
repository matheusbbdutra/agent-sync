# ADR: Runtime TS dos plugins OpenCode v2 — contrato de pré-requisitos

**Status**: Aceito
**Data**: 2026-09-21
**Decisor**: Matheus Dutra
**Tags**: ai-agent, opencode, runtime, typescript, scripts, apply

## Contexto

O `agent-sync -target opencode` wirava os plugins `.ts` para `~/.config/opencode/plugins/` corretamente desde D-26 (2026-09-20), mas o **runtime TS** necessário para esses plugins carregarem ficava silenciosamente quebrado em PCs novos:

- Falta de Node >= 20.11 (alguns plugins usam `import.meta.dirname`)
- Falta do binário OpenCode v2 (Bun-compiled)
- Falta do pacote `@opencode/plugin` em algum `node_modules` que o loader v2 ache

Resultado: `.ts` wirados mas plugin não carrega. Documentado em `.agent-sync/investigation-plugin-loader.md` (2026-09-20): "Logs do OpenCode mostram `Cannot find package '@opencode/plugin'` intermitentemente". Sem mensagem de erro visível ao usuário do `agent-sync`.

## Decisão

O contrato de runtime TS vira responsabilidade explícita, distribuído em 3 camadas:

1. **`scripts/setup-opencode.sh`** (commit `b4f584f`) — fonte de verdade do bootstrap. Valida os 3 pré-requisitos em modo `--check` (CI/diagnóstico) ou instala idempotentemente. Mesmo padrão de `scripts/setup-go.sh`, sem sudo.

2. **Pré-check no `applyToTarget`** (commit `caf57e8`) — antes de wirar plugins v2, emite warning amarelo com comando corretivo apontando para `scripts/setup-opencode.sh`. **Não bloqueia o wirar** (preserva D-26: best-effort, setup é responsabilidade do usuário).

3. **Esta ADR** — formaliza que wirar plugins `.ts` é separado do runtime TS que os executa. Documenta o contrato e a reversão trivial.

## Pré-requisitos (validados empiricamente em `@opencode/plugin@2.0.11`)

| Pré-requisito | Mínimo | Como validar | Sem isso |
|---|---|---|---|
| Node | >= 20.11 | `node --version` | Plugins com `import.meta.dirname` falham |
| Binário OpenCode | >= v2.0.0 | `opencode --version` | Loader v2 não existe; wirar vira silencioso |
| `@opencode/plugin` | resolúvel | `find ~/.config/opencode/node_modules/@opencode/plugin` | Loader v2 lança `Cannot find package` |

**Não é pré-requisito**: `tsx` global. O binário OpenCode v2 embute `Bun.Transpiler` (`@opencode/plugin/dist/source.bun.js:loader`) — TS é transpilado pelo runtime Bun do OpenCode, não por `tsx` externo.

## Matriz de comportamento

| Cenário | Wirar plugins | Warning emitido | Comportamento |
|---|---|---|---|
| OpenCode v1 | só `.opencode.ts` (legados) | não | Plugins legados wiram normalmente |
| OpenCode v2 + tudo OK | todos os `.opencode.v2.ts` | não | Tudo funciona |
| OpenCode v2 + falta `@opencode/plugin` | todos wiram | sim (1 warning) | Plugins wirados mas não carregam |
| OpenCode v2 + Node < 20.11 | todos wiram | sim (1 warning) | Plugins wirados; runtime pode falhar |
| OpenCode v2 + sem binário | nenhum wirado | sim (1 warning) | opencodeMajorVersion cai para 1, wirar vira v1-only |

## Reversão

- **Sem `scripts/setup-opencode.sh`**: `git revert b4f584f` (mantém ADR + apply pré-check).
- **Sem pré-check no apply**: `git revert caf57e8` (mantém script standalone).
- **Sem ambos**: `git revert b4f584f caf57e8` — volta ao comportamento pré-A-33 (wirar silencioso sem aviso).
- **Workaround manual** (sem script): `npm i -g @opencode/plugin` (Node 20+ já vem com mise/asdf na maioria dos setups).

## Anti-over-engineering (AGENTS.md §3)

- **Por que não subcommand `agent-sync doctor`?** Escopo. O pré-check já existe no `-apply`. Subcommand novo vira manutenção dupla. O script standalone cobre o caso "quero só diagnosticar".
- **Por que `npm i -g` em vez de instalar local?** O loader v2 (`source.node.js:50-58`) consulta `localSource` a partir do `parentURL` do plugin. Plugin em `~/.config/opencode/plugins/` resolve `@opencode/plugin` melhor a partir de `~/.config/opencode/node_modules/` (sem precisar de permissões globais). Script tenta local primeiro, global como fallback.
- **Por que não instalar o binário OpenCode automaticamente?** Fora de escopo deliberado. mise/asdf resolvem versão pinning; misturar isso no script de setup TS acoplaria concerns.

## Smoke

- ✅ `bash scripts/setup-opencode.sh --check` (PC OK): exit 0, 3 ✅.
- ✅ `HOME=tmp AGENT_SYNC_OPENCODE_VERSION=2 agent-sync -target opencode` (sem `@opencode/plugin`): warning visível com hint, wirar prossegue.
- ✅ `go test ./cmd/agent-sync/`: 4 testes novos verdes, 0 regressão.

## Pendências separadas

- Smoke planejado real (smoke planejado cross-PC com PC quebrado real) fica para entrega subsequente.
- Adicionar `scripts/setup-opencode.sh` ao `Makefile` target `setup` (junto com `setup-go.sh`).
