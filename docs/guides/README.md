# Guides

Documentos auxiliares referenciados pelo README principal. ADR-*.md
(Decisões de design) ficam em `../`; guias são tutoriais/referências
operacionais sem conotação de ADR.

## Índice

- [`memory-cross-cli.md`](memory-cross-cli.md) — `memory-mcp` + nudge
  hook e como cada CLI consome.
- [`context-window-details.md`](context-window-details.md) — paper de
  referência e detalhes da sliding window.
- [`sync-between-pcs.md`](sync-between-pcs.md) — `memory-sync` config
  entre múltiplos PCs via Turso Cloud.
- [`architecture-flow.md`](architecture-flow.md) — fluxos end-to-end
  atuais (build → sync → runtime → estado canônico).
- [`ideal-flow.md`](ideal-flow.md) — fluxos target (3 camadas
  canônicas injetadas no SessionStart das 5 CLIs).
