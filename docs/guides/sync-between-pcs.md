# Sync between two PCs (Turso Cloud)

Procedimento para compartilhar memória entre múltiplos PCs via
Turso Cloud + `memory-sync`.

## Inicialização em cada PC

```bash
memory-sync -init
```

Cria `~/.config/agent-sync/config.json` (0600, fora do repo, nunca sobrescrito)
e imprime seu path. Configure:

- `turso.url` → `libsql://your-database.turso.io`
- `turso.token` → token deste PC

## `config.json` para projetos com `origin`/`upstream` remote

```json
{
  "turso": {
    "url": "libsql://your-database.turso.io",
    "token": "your-token"
  }
}
```

ID do projeto derivado automaticamente do remote.

## `config.json` para projetos sem remote

Mapear cada path local para um ID estável:

```json
{
  "turso": { "url": "libsql://your-database.turso.io", "token": "your-token" },
  "projects": {
    "/home/you/projects/app": "main-app"
  }
}
```

## Wrapper `agent-sync-session`

```bash
agent-sync-session ~/Documents/agent-sync codex
# ou claude / antigravity / opencode / cursor
```

Comportamento:
- Exige checkout limpo (`git pull --ff-only` interno).
- Atualiza as 5 instalações de CLI quando repo muda.
- Baixa memórias antes de iniciar o CLI.
- No exit: sobe memórias persistentes + push dos commits feitos na sessão.
- **Não** commita mudanças locais pendentes.

## Uso manual

```bash
memory-sync -phase start                  # antes da sessão
memory-sync -phase end                   # depois
# Em conflito:
memory-sync -phase resolve-local -conflict <id>
memory-sync -phase resolve-remote -conflict <id>
```

## Detalhes técnicos

- DB local: `~/.cache/agent-sync/memory.db`. Memórias scratch são
  excluídas do sync.
- MCP grava PC + path local + project ID. Search por `project_dir`
  resolve o mesmo projeto em paths diferentes.
- Conexão Turso Cloud precisa validação contra a própria conta.
