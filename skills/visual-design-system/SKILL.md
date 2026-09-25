---
name: visual-design-system
description: Diretrizes de engenharia visual, design systems, consistência estética e acessibilidade WCAG para desenvolvimento frontend. Use ao criar interfaces, componentes web, formulários ou estruturar tokens de design.
---

# Visual Design System & Frontend Engineering

Esta skill estabelece os padrões e critérios de aceitação visual para código frontend produzido pelos agentes.

## 1. Sistema de Tokens & Escalas

### Cores Semânticas
Não use valores hexadecimais soltos no markup. Use tokens com significado semântico:
- `primary`: Ação principal, identidade do produto.
- `surface` / `background`: Camadas de profundidade do layout (canvas, cards, popovers).
- `muted` / `subtle`: Bordas, divisores e fundos secundários.
- `text`: Hierarquia clara (`text-primary`, `text-muted`, `text-inverse`).
- `destructive` / `warning` / `success`: Feedback de estado estritamente padronizado.

### Grade de Espaçamento
- Adote múltiplos de **4px / 8px** (ex.: Tailwind `gap-1`, `gap-2`, `gap-4`, `p-6`).
- Mantenha consistência entre containers vizinhos.

### Escala Tipográfica
- `text-xs` (12px): Metadados, tags secundárias, timestamps.
- `text-sm` (14px): Textos de interface, tabelas, botões e formulários.
- `text-base` (16px): Corpo de texto principal.
- `text-lg` / `text-xl` (18-20px): Títulos de seções ou cards.
- `text-2xl` / `text-3xl`: Títulos de páginas e dashboards.

---

## 2. Ciclo de Vida Completo de Componentes

Nenhum componente interativo pode ter apenas o estado "ideal" (*happy path*). Toda implementação deve prever:

1. **Default:** Estado de repouso legível.
2. **Hover:** Indicação visual de interatividade (mudança sutil de tonalidade ou elevação).
3. **Focus-visible:** Foco por teclado nítido (ex.: `focus-visible:ring-2 focus-visible:ring-offset-2`).
4. **Active:** Feedback de clique (micro-transição).
5. **Disabled:** Visualmente atenuado com `cursor-not-allowed` e `opacity-50`, bloqueando eventos.
6. **Loading:** Spinner ou Skeleton sem quebrar a geometria do container (evitar saltos de layout / Layout Shifts).
7. **Empty State:** Ilustração ou texto explicativo amigável com botão de ação quando não houver dados.

---

## 3. Checklist de Acessibilidade (WCAG 2.1 AA)

- **Contraste Mínimo:** 4.5:1 para texto normal; 3:1 para textos grandes e elementos gráficos de UI.
- **Teclado:** Todos os controles interativos devem ser alcançáveis via `Tab` e acionáveis via `Enter` ou `Space`.
- **Formulários:** Inputs devem ter sempre `<label>` explícito associado (`htmlFor` / `id`) e mensagens de erro descritivas com `aria-describedby`.
- **Botões de Ícone:** Qualquer botão composto apenas por ícone deve conter `aria-label` descritivo.
