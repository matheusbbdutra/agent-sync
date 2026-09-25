# ADR: Especialização de Engenharia Visual e Design System no Frontend (UI/UX Pro Max)

- **Status**: Aceito (promovido por D-64 em 2026-09-23; skill `visual-design-system` + agente `frontend-ui-designer` wirados nas 5 CLIs — Claude/Codex/OpenCode/Cursor/Antigravity)
- **Data**: 2026-09-23
- **Decisor**: Matheus Dutra
- **Tags**: frontend, ui-ux, design-system, tailwind, accessibility, visual-engineering

---

## Contexto

Historicamente, o `agent-sync` concentrou seus agentes e skills no núcleo de backend (PHP/Symfony, Go, PostgreSQL, segurança, testes e arquitetura). No entanto:

1. **Demanda Real por Qualidade Visual e Frontend:** Ao desenvolver aplicações completas ou módulos de interface, agentes de propósito geral tendem a gerar interfaces genéricas, sem hierarquia tipográfica consistente, sem estados de interação (hover, focus, loading, skeleton, empty states) e com problemas de acessibilidade (WCAG).
2. **Lições de Projetos como UI UX Pro Max:** A alta adesão da comunidade a projetos como `nextlevelbuilder/ui-ux-pro-max-skill` decorre da carência de diretrizes estéticas rigorosas em modelos de linguagem. Agentes precisam de regras concretas de design tokens, contraste, espaçamento matemático (4px/8px grid) e feedback de UX.
3. **Escopo Cirúrgico:** Não se trata de inflar o repositório com centenas de componentes cosméticos, mas de fornecer uma **skill canônica e um subagente de engenharia visual** focados em produzir interfaces de alto nível sem alucinações de estilo.

---

## Decisão

Instituir no `agent-sync` uma vertente canônica de **Engenharia Visual e Design System**:

### 1. Criação da Skill Autoral `visual-design-system`
- Adicionar em `skills/visual-design-system/SKILL.md` (sob o domínio `frontend-design` no manifesto).
- **Conteúdo das Diretrizes Técnicas:**
  - **Sistema de Tokens:** Escalas tipográficas estruturadas, paleta semântica (Primary, Neutral, Surface, Destructive) e uso consistente de Tailwind CSS / CSS Variables.
  - **Estados de Componentes Obrigatórios:** Todo componente de interface deve prever explicitamente: Default, Hover, Active, Disabled, Loading e Error.
  - **Acessibilidade e Micro-interações:** Contraste mínimo WCAG AA (4.5:1), navegação por teclado e foco visível (`focus-visible:ring-2`).

### 2. Subagente Especialista `frontend-ui-designer` (Read-only / Auditor)
- Criar a especificação do subagente canônico em `agents/frontend-ui-designer.md`.
- Atuação: revisar código de componentes frontend, inspecionar consistência visual, apontar quebras de design token e sugerir correções de acessibilidade e refinamento estético antes do commit.

### 3. Matriz de Cobertura Cross-CLI (5xN)

| CLI | Estado | Mecanismo de Integração |
|---|---|---|
| **Claude Code** | ✅ Coberto | Subagente `frontend-ui-designer` gerado em `~/.claude/agents/` e skill symlinkada. |
| **OpenCode v2** | ✅ Coberto | Subagente registrado com permissão granular em `~/.config/opencode/agents/`. |
| **Codex** | ✅ Coberto | Subagente gerado em `~/.codex/agents/` com modo `sandbox_mode=read-only`. |
| **Antigravity** | ✅ Coberto | Subagente canônico gerado nas definições de agents do Antigravity CLI. |
| **Cursor** | ✅ Coberto | Subagente gerado em `~/.cursor/agents/frontend-ui-designer.md` (`readonly: true`). |

---

## Consequências

**Positivas:**
- Elevação substancial no acabamento estético e ergonomia das telas geradas pelas CLIs.
- Prevenção de retrabalho com refatorações de CSS desorganizado ou classes inline arbitrárias.
- Complementa a esteira de desenvolvimento completa do repositório (do banco ao design visual).

**Negativas / Trade-offs:**
- Requer atenção para que o agente respeite o stack frontend do projeto do usuário (ex.: Tailwind vs. CSS Modules vs. Styled Components) sem impor frameworks não utilizados.
