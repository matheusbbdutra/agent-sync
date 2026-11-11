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

### 1.1 Revisão 2026-10-27 — Camada Anti-Genérico ("cara de IA")

**Motivo:** a skill cobria tokens, estados e WCAG, mas nenhuma regra sobre o visual genérico que denuncia UI gerada por IA. O catálogo externo `ui-design` (`plugins/ui-design/skills/*`, ~1.4k linhas) é tecnicamente mais fundo, porém seu próprio quick-start prescreve `Inter` + `#2563eb` (`visual-design-foundations/SKILL.md:106,121`) — portar 1:1 reforçaria o problema em vez de resolvê-lo.

**Decisão:** expandir a skill canônica (52 → 112 linhas) com: §1 direção obrigatória antes do markup (adjetivos, referência real, densidade, modo); §2 deny-list de padrões de IA (gradiente roxo, card soup, glassmorphism, hero clichê, copy "Seamlessly/Elevate"); §3 cor com intenção (neutros tintados, uma cor de marca); §4 tipografia (escala modular, measure 45–75ch); §5 ritmo vertical variado e densidade por contexto; §7 motion (durações, `prefers-reduced-motion`); §9 aceitação com "teste do print" 1440/390.

**Wiring:** `frontend-developer` carrega a skill e declara direção antes de tela nova; `frontend-ui-designer` ganha princípio, 2 itens de checklist e diagnóstico de originalidade.

**Fora de escopo (não decidido):** skill nova `frontend-design-direction` com entrevista de direção (versão sem Stitch do `brand-landingpage`). Hoje o §1 cobre o caso; criar apenas se houver demanda recorrente de landing page.

**Validação:** `go run ./cmd/agent-sync skills lint` → 54 skills, 0 erros; `make apply` → 5 CLIs sincronizadas.

### 1.2 Revisão 2026-11-10 — Cobertura mobile, UX writing e precisão WCAG

**Motivo:** a skill era web-centric e a deny-list de primeira geração não cobria a estética "anti-IA" que substitui o gradiente roxo por creme + serifada + terracota. Além disso, o item 44x44 estava rotulado como AA, mas é AAA na WCAG 2.1 (2.5.5); o AA da WCAG 2.2 (2.5.8) é 24x24.

**Decisão:**
- §2: 12 itens, incluindo estética "anti-IA" e emoji como ícone.
- §6: success state.
- §8: 24x24 AA / 44x44 AAA na web; 48dp no Android / 44pt no iOS.
- §9: seção mobile nativo (Compose/XML/iOS), sem duplicar implementação; skill de Compose dedicada fica fora de escopo até confirmação do stack.
- §10: UX writing (CTA, erro, empty, success, consistência).
- §11: aceitação com font scale, screenshot/emulador e "não verificado".
- Precedência: design system/`DESIGN.md`/tokens do projeto > skill > skill externa.
- Retrofit: preservar por padrão; substituir direção existente só com pedido explícito.

**Fora de escopo (não decidido):** skill `android-compose-ui` dedicada. Criar apenas se o projeto confirmar Jetpack Compose (não XML) e houver demanda recorrente; `expo-native-ui` cobre React Native/Expo, não Kotlin.

**Validação:** `go run ./cmd/agent-sync skills lint` → 54 skills, 0 erros; screenshots não executados (sem app alvo).

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
