---
name: visual-design-system
description: Diretrizes de engenharia visual, design systems, consistência estética e acessibilidade WCAG para desenvolvimento frontend. Use ao criar telas, landing pages, dashboards, componentes web, formulários ou estruturar tokens de design — inclui deny-list para evitar o visual genérico de UI gerada por IA.
---

# Visual Design System & Frontend Engineering

Esta skill estabelece os padrões e critérios de aceitação visual para código frontend produzido pelos agentes. O alvo é uma interface que pareça **produto real**, não template gerado.

## 1. Direção Antes do Código

Nenhuma tela nova começa pelo markup. Declare primeiro, em 4 linhas no plano:

- **Adjetivos:** 3 que a interface deve transmitir (ex.: densa, sóbria, editorial, lúdica).
- **Referência:** 1 produto real (site ou app) cujo acabamento serve de norte. Sem referência, ancore na estética do domínio do usuário (ferramenta fiscal ≠ SaaS de IA).
- **Densidade:** compacta (dashboard/ferramenta) ou arejada (landing/marketing).
- **Modo:** claro, escuro ou ambos.

Sem direção declarada, o resultado é o default do modelo — que é justamente o visual genérico. Se o usuário não deu referência e a tela é de marketing, pergunte uma vez antes de gerar.

### 1.1 Modo Retrofit (tela existente)

Ajustar o que existe é diferente de criar: o custo do erro é maior porque a tela já está em uso. Cinco passos, nesta ordem:

1. **Auditar antes de tocar:** inventarie tokens, tipografia, espaçamento e superfícies já presentes; marque só o que viola a deny-list (§2).
2. **Nomear a direção implícita:** a tela já tem uma direção, mesmo que ruim. Declare se vai assumi-la e corrigir ou substituí-la — e por quê.
3. **Atacar por impacto:** tipografia → densidade/espaço → cor → superfícies/bordas → motion. Pare quando o escopo acabar.
4. **Raio de mudança:** ou aplica a linguagem em **todas** as superfícies tocadas, ou não toca. Redesign parcial que mistura dois estilos é pior que o original.
5. **Não reescreva o que funciona:** se 70% está bom, corrija os 30%. Preserve markup, classes e lógica que já atendem — ajuste de UI não é pretexto para refatorar código adjacente.

Valide com o §9 inteiro, incluindo o teste do print lado a lado (antes/depois).

## 2. Deny-List: o que denuncia UI de IA

Não use os padrões abaixo sem justificativa explícita na direção (§1):

| Padrão | Por quê | Em vez disso |
|---|---|---|
| Gradiente roxo→azul, glow, `blur-3xl` atrás do hero | Assinatura visual de template | Cor de marca sólida com intenção; no máximo 1 gradiente justificado |
| `Inter`/`system-ui` como fonte "de design" e todos os títulos em `font-bold tracking-tight` | Ausência de escolha tipográfica | Uma família com personalidade para display; tracking negativo só em tamanhos grandes |
| Card soup: todo bloco em `rounded-2xl border shadow-lg p-6` com ícone em quadradinho `bg-primary/10` | Sem hierarquia: tudo tem o mesmo peso | Uma só linguagem de superfície (borda fina **ou** flat); hierarquia por espaço e tipografia |
| Hero clichê: badge "✨ Novo", "Supercharge your X", dois CTAs genéricos, 3 features "Fast/Secure/Scalable" | Copy e layout de molde | Copy e números concretos do domínio; um CTA primário |
| Glassmorphism, `backdrop-blur` com borda `white/10` no dark | Efeito decorativo sem função | Superfícies opacas com contraste real |
| Tudo `py-24 gap-6`, 100% simétrico, sempre 3 colunas iguais | Ritmo uniforme denuncia geração em série | Ritmo vertical variado + ao menos 1 assimetria intencional |
| `text-gray-500` sobre `gray-100`, paleta apagada | Baixo contraste e cinza puro | Neutros tintados na direção da marca e contraste AA conferido |
| "Seamlessly", "Effortlessly", "Elevate", "Unlock", "Empower", "Supercharge" | Vocabulário de marketing genérico | Vocabulário do domínio do produto |
| Só o happy path: nenhum empty/error state, skeleton de 3 barras cinzas | Interface não sobrevive a dado real | Estados reais por componente (§6) |
| `hover:scale-105`, `animate-pulse` decorativo, tudo animado | Ruído visual e custo de performance | Motion só com propósito (§7) |

## 3. Sistema de Tokens & Cor

Não use valores hexadecimais soltos no markup. Use tokens com significado semântico:

- `primary`: ação principal, identidade do produto.
- `surface` / `background`: camadas de profundidade (canvas, cards, popovers).
- `muted` / `subtle`: bordas, divisores e fundos secundários.
- `text`: `text-primary`, `text-muted`, `text-inverse`.
- `destructive` / `warning` / `success`: feedback de estado estritamente padronizado.

Regras:

- **Uma cor de marca** com justificativa na direção. Não herde o `blue-600`/`violet-600` do framework.
- **Neutros tintados** (OKLCH ou HSL com chroma baixo na matiz da marca), não `gray-*` puro.
- **Dark mode não é inversão**: superfícies mais claras elevam, não mais escuras; sombra perde função e vira borda.
- Contraste conferido no par real usado, não no token isolado.

## 4. Tipografia

- Escala modular: razão **1.2** para UI densa, **1.333** para marketing. Não invente 8 tamanhos arbitrários.
- `measure` de **45–75ch** para corpo de texto (`max-w-prose` / `65ch`).
- Line-height por papel: display 1.0–1.15, corpo 1.5–1.7, UI 1.2–1.4.
- Tracking negativo apenas em tamanhos display (≥24px); positivo apenas em caixa-alta pequena (labels).
- **Duas famílias no máximo**, com contraste real: display com personalidade + texto neutro e legível. Pesos: no máximo 2 por tela.
- Fonte via `font-display: swap` + fallback métrico próximo; nunca depender de CDN bloqueante.

## 5. Espaço, Layout e Densidade

- Grade de **4px/8px** (`gap-1`, `gap-2`, `gap-4`, `p-6`) e consistência entre containers vizinhos.
- Ritmo vertical **variado** por seção (ex.: 48/80/120px no desktop; 32/48/64px no mobile). Nunca o mesmo `py-*` em tudo.
- Duas larguras de referência: **conteúdo** (65ch) e **aplicação** (1200–1440px). Dashboard usa gutter menor e densidade maior que landing.
- Assimetria intencional em pelo menos um bloco de marketing (2/3 + 1/3), não 3 colunas iguais por padrão.
- **390px é requisito de aceitação**, não "depois": sem scroll horizontal e sem texto truncado.

## 6. Ciclo de Vida Completo de Componentes

Nenhum componente interativo pode ter apenas o estado "ideal" (*happy path*). Toda implementação deve prever:

1. **Default:** estado de repouso legível.
2. **Hover:** indicação sutil de interatividade (tonalidade ou elevação).
3. **Focus-visible:** foco por teclado nítido (ex.: `focus-visible:ring-2 focus-visible:ring-offset-2`).
4. **Active:** feedback de clique (micro-transição).
5. **Disabled:** atenuado com `cursor-not-allowed` e `opacity-50`, bloqueando eventos.
6. **Loading:** spinner ou skeleton sem quebrar a geometria do container (sem Layout Shift).
7. **Empty state:** texto explicativo do domínio + ação quando não houver dados.
8. **Error:** mensagem específica e recuperável, nunca só "Algo deu errado".

## 7. Motion

- Durações: **120ms** (micro), **180ms** (hover), **240ms** (entrada), **320ms** (transição de tela). Acima de 400ms só se coreografado.
- `ease-out` para entrada, `ease-in` para saída; `linear` não se usa em UI.
- Anime só `transform`, `opacity` e `filter` — nunca `width`, `height`, `top` ou `left`.
- `prefers-reduced-motion` obrigatório sempre que houver animação.
- Animação decorativa em loop é proibida.

## 8. Checklist de Acessibilidade (WCAG 2.1 AA)

- **Contraste:** 4.5:1 para texto normal; 3:1 para textos grandes e elementos gráficos de UI.
- **Teclado:** todos os controles alcançáveis via `Tab` e acionáveis via `Enter`/`Space`; foco nunca obscurecido por elemento fixo.
- **Formulários:** `<label>` explícito associado (`htmlFor`/`id`) e erro descritivo com `aria-describedby`.
- **Botões de ícone:** sempre com `aria-label` descritivo.
- **Alvo de toque:** mínimo 44x44px.

## 9. Aceitação

Uma tela só está pronta quando **todos** os itens forem verdadeiros:

- [ ] Direção (§1) declarada e seguida.
- [ ] Nenhum item da deny-list (§2) sem justificativa escrita.
- [ ] Tokens semânticos; zero hex solto no markup.
- [ ] Todos os estados do componente (§6) implementados.
- [ ] WCAG AA verificado (§8).
- [ ] 1440px e 390px sem quebra nem scroll horizontal.
- [ ] `prefers-reduced-motion` respeitado.
- [ ] **Teste do print:** comparando o screenshot com 1 produto real da mesma categoria, existe ao menos uma decisão visual (tipografia, cor, densidade ou composição) que um template pronto não teria. Se não existir, refaça.
- [ ] **Em tela existente:** o raio de mudança (§1.1) foi respeitado — sem mistura de dois estilos na mesma tela.
