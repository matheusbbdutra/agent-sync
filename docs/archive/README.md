# `docs/archive/` — Material histórico

> **Convenção**: arquivos aqui foram movidos de `docs/` por uma das razões abaixo. **Não referenciar em código novo, ADRs novas ou sprints futuras** — usar a versão atual em `docs/` ou `docs/sprints/`.

## Estrutura

```
docs/archive/
├── adrs-descartadas/                     ADRs re-Proposto→Descartado por decisão posterior
│   └── ADR-code-graph-bounded-context.md subsumido por D-49/D-53/D-54/D-57; D-64 em 2026-09-23
└── sessoes/                              Documentos de trabalho efêmeros de sessões fechadas
    └── 2026-09-22-cruzamento-5-repos/    análise de 5 repos × agent-sync (ses_atual fechada 2026-09-22)
        ├── SESSION-2026-09-22-skill-lint-and-repos-analysis.md
        ├── repos-cruzamento.md
        ├── A-N-cruzamento-escopo.md
        ├── A-N-cruzamento-preaudits.md
        └── A-N-cruzamento-preaudits-2.md
```

## Política de promoção (anti-overengineering)

- **Não deletar** conteúdo do archive sem confirmação humana explícita (mesmo padrão de `docs/sprints/archive/`).
- **Não reintroduzir** arquivos sem reabrir a ADR/sessão com critério novo.
- **Não duplicar**: se trabalho do archive virou S-0.X ou A-N nova, o board é a fonte de verdade — não voltar ao doc de sessão.

## Como adicionar ao archive

1. Verificar refs ativos (`grep -rn '<arquivo>' --include='*.md' --include='*.go' --include='*.json'`).
2. `git mv` (preserva histórico).
3. Atualizar header do arquivo com nota "MOVIDO PARA ARCHIVE" + data.
4. Commitar granular com `docs(archive)` na mensagem.

## Histórico

- **2026-09-25** (D-101) — Criação inicial: 1 ADR descartada + 5 docs de sessão 2026-09-22.