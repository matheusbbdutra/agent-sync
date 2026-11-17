---
name: visual-design-system
description: Visual engineering guidelines, design systems, aesthetic consistency, UX writing and WCAG accessibility for web and mobile apps (Android/Compose/XML and iOS). Use when creating screens, landing pages, dashboards, components, forms, native apps or structuring design tokens — includes a deny-list to avoid the generic look of AI-generated UI.
---

# Visual Design System & Frontend Engineering

This skill establishes the visual standards and acceptance criteria for frontend code produced by agents. The target is an interface that looks like a **real product**, not a generated template.

## 1. Direction Before Code

No new screen starts with the markup. Declare first, in 4 lines in the plan:

- **Adjectives:** 3 that the interface must convey (e.g.: dense, sober, editorial, playful).
- **Reference:** 1 real product (site or app) whose finish serves as the north star. Without a reference, anchor on the aesthetics of the user's domain (tax tool ≠ AI SaaS).
- **Density:** compact (dashboard/tool) or airy (landing/marketing).
- **Mode:** light, dark or both.

Without a declared direction, the result is the model's default — which is precisely the generic look. If the user gave no reference and the screen is marketing, ask once before generating.

**Precedence:** the design system, `DESIGN.md` and the project tokens beat this skill; this skill beats a generic external skill. If the user's brief fixes an aesthetic that violates the deny-list, the brief wins — record the exception and proceed.

**Personal finance domain** (wallet, cards, budget, installments, cash flow): also read [reference/consumer-fintech.md](reference/consumer-fintech.md) — it fixes the money model (efetivado/previsto/reservado/livre [settled/forecast/reserved/free]), the value formatting and the domain dataviz rules. It does not replace any section of this skill.

**Screen structure:** [reference/product-archetypes.md](reference/product-archetypes.md) brings the optional catalog of archetypes (overview, feed, object management, analytics, creation) to decide the hierarchy of each tab. Optional because prescribing the same skeleton on every screen is precisely the §2 syndrome.

**Icons, prototyping and authorship:** [reference/iconography.md](reference/iconography.md) fixes the icon family, the 24×24 / 2 px grid, sizes 16/20/24/32, contrast and touch target. [reference/prototype-fidelity.md](reference/prototype-fidelity.md) is the fidelity ladder — paper, wireframe, hand-drawn, interactive — with what each level can and cannot answer. [reference/human-craft.md](reference/human-craft.md) is the positive side of §2: how a screen looks authored instead of generated, and the "would a template have this?" test used in §11.

### 1.1 Retrofit Mode (existing screen)

Adjusting what already exists is different from creating: the cost of error is higher because the screen is already in use. Five steps, in this order:

1. **Audit before touching:** inventory the tokens, typography, spacing and surfaces already present; mark only what violates the deny-list (§2).
2. **Name the implicit direction:** the screen already has a direction, even a bad one. Declare whether you will take it on and correct it or replace it — and why. **Preserve by default; replace the existing direction only with an explicit redesign request.**
3. **Attack by impact:** typography → density/space → color → surfaces/borders → motion. Stop when the scope ends.
4. **Blast radius:** either apply the language to **all** touched surfaces, or do not touch it. A partial redesign that mixes two styles is worse than the original.
5. **Do not rewrite what works:** if 70% is good, fix the 30%. Preserve markup, classes and logic that already deliver — a UI adjustment is no pretext to refactor adjacent code.

Validate with §11 in full, including the side-by-side print test (before/after).

## 2. Deny-List: what betrays AI UI

Do not use the patterns below without an explicit justification in the direction (§1):

| Pattern | Why | Instead |
|---|---|---|
| Purple→blue gradient, glow, `blur-3xl` behind the hero | A template's visual signature | An intentional solid brand color; at most 1 justified gradient |
| `Inter`/`system-ui` as the "de design" [design] font and every heading in `font-bold tracking-tight` | Absence of a typographic choice | One family with personality for display; negative tracking only at large sizes |
| Card soup: every block in `rounded-2xl border shadow-lg p-6` with an icon in a little square `bg-primary/10` | No hierarchy: everything has the same weight | A single surface language (thin border **or** flat); hierarchy through space and typography |
| Cliché hero: badge "✨ Novo" [New], "Supercharge your X", two generic CTAs, 3 "Fast/Secure/Scalable" features | Template copy and layout | Concrete copy and numbers from the domain; one primary CTA |
| Glassmorphism, `backdrop-blur` with a `white/10` border in dark | Decorative effect without function | Opaque surfaces with real contrast |
| Everything `py-24 gap-6`, 100% symmetric, always 3 equal columns | A uniform rhythm betrays mass generation | A varied vertical rhythm + at least 1 intentional asymmetry |
| `text-gray-500` over `gray-100`, washed-out palette | Low contrast and pure gray | Neutrals tinted toward the brand and AA contrast verified |
| "Seamlessly", "Effortlessly", "Elevate", "Unlock", "Empower", "Supercharge" | Generic marketing vocabulary | The product domain's vocabulary |
| Only the happy path: no empty/error state, a skeleton of 3 gray bars | The interface does not survive real data | Real states per component (§6) |
| `hover:scale-105`, decorative `animate-pulse`, everything animated | Visual noise and performance cost | Motion only with purpose (§7) |
| The "anti-IA" [anti-AI] aesthetic (cream + serif + terracotta, broadsheet with hairlines, near-black + acid green) | It is also a default; the agent merely swaps one cluster for another | A choice anchored in the domain; at most 1 strong aesthetic gesture |
| Emoji as a UI icon | It amateurizes and breaks consistency across platforms | A single icon library (Lucide, Material Symbols, SF Symbols) |

## 3. Token System & Color

Do not use loose hexadecimal values in the markup. Use tokens with semantic meaning:

- `primary`: main action, product identity.
- `surface` / `background`: depth layers (canvas, cards, popovers).
- `muted` / `subtle`: borders, dividers and secondary backgrounds.
- `text`: `text-primary`, `text-muted`, `text-inverse`.
- `destructive` / `warning` / `success`: strictly standardized state feedback.

Rules:

- **One brand color** with a justification in the direction. Do not inherit the framework's `blue-600`/`violet-600`.
- **Tinted neutrals** (OKLCH or HSL with low chroma on the brand hue), not pure `gray-*`.
- **Dark mode is not an inversion**: lighter surfaces elevate, not darker ones; shadow loses its function and becomes a border.
- Contrast verified on the real pair used, not on the isolated token.
- **Brand adaptation:** when swapping the palette, keep the *relationship*, not the hex — canvas in warm neutral,
  elevated neutral surface, deep brand anchor (not a bright color) for hero and CTA,
  and a vibrant accent used **sparingly**. A light accent (lime, coral, vivid amber) exists only
  as a background with dark ink on top; as text, border or focus ring it does not reach 3:1.
- **Pairs that must be measured in both themes** (the isolated token says nothing): primary and secondary text
  over canvas and over surface; semantic (`success`/`danger`/`warning`/`info`) over
  its own `-bg`; accent as background; border or anchor against the canvas. When creating the dark theme,
  redefine **also** the semantics, the `-bg` and the anchor — a token that crosses from light to dark
  becomes a light island or an invisible element.
- **Surface scale:** radius, shadow, border and canvas→surface delta come from
  [reference/surface-derivation.md](reference/surface-derivation.md) — values measured on GOV.UK,
  Carbon, Polaris and Radix. Between products the system varies; between screens only composition varies.

## 4. Typography

- Modular scale: ratio **1.2** for dense UI, **1.333** for marketing. Do not invent 8 arbitrary sizes.
- `measure` of **45–75ch** for body text (`max-w-prose` / `65ch`).
- Line-height per role: display 1.0–1.15, body 1.5–1.7, UI 1.2–1.4.
- Negative tracking only at display sizes (≥24px); positive only in small uppercase (labels).
- **Two families at most**, with real contrast: a display family with personality + neutral, legible text. Weights: at most 2 per screen.
- Font via `font-display: swap` + a close metric fallback; never depend on a blocking CDN.

## 5. Space, Layout and Density

- **4px/8px** grid (`gap-1`, `gap-2`, `gap-4`, `p-6`) and consistency between neighboring containers.
- **Varied** vertical rhythm per section (e.g.: 48/80/120px on desktop; 32/48/64px on mobile). Never the same `py-*` on everything.
- Two reference widths: **content** (65ch) and **application** (1200–1440px). A dashboard uses a smaller gutter and higher density than a landing page.
- Intentional asymmetry in at least one marketing block (2/3 + 1/3), not 3 equal columns by default.
- **390px is an acceptance requirement**, not "depois" [later]: no horizontal scroll and no truncated text.

## 6. Full Component Lifecycle

No interactive component may have only the "ideal" state (*happy path*). Every implementation must provide for:

1. **Default:** a legible resting state.
2. **Hover:** a subtle indication of interactivity (tone or elevation).
3. **Focus-visible:** crisp keyboard focus (e.g.: `focus-visible:ring-2 focus-visible:ring-offset-2`).
4. **Active:** click feedback (micro-transition).
5. **Disabled:** dimmed with `cursor-not-allowed` and `opacity-50`, blocking events.
6. **Loading:** spinner or skeleton without breaking the container geometry (no Layout Shift).
7. **Empty state:** explanatory text from the domain + an action when there is no data.
8. **Error:** a specific, recoverable message, never just "Algo deu errado" [Something went wrong].
9. **Success:** confirmation of what changed (toast/inline), not just "Sucesso" [Success].
10. **Living prototype:** in a demo or prototype, the filter filters, the tab switches panels, the card opens a drawer and
    the theme toggles. A button that does not respond is a failing grade, not a placeholder.

## 7. Motion

- Durations: **120ms** (micro), **180ms** (hover), **240ms** (entrance), **320ms** (screen transition). Above 400ms only if choreographed.
- `ease-out` for entrance, `ease-in` for exit; `linear` is not used in UI.
- Animate only `transform`, `opacity` and `filter` — never `width`, `height`, `top` or `left`.
- `prefers-reduced-motion` mandatory whenever there is animation.
- Looping decorative animation is prohibited.

## 8. Accessibility Checklist (WCAG 2.1 AA / 2.2 AA)

- **Contrast:** 4.5:1 for normal text; 3:1 for large text and UI graphical elements.
- **Keyboard:** every control reachable via `Tab` and operable via `Enter`/`Space`; focus never obscured by a fixed element.
- **Forms:** explicit associated `<label>` (`htmlFor`/`id`) and a descriptive error with `aria-describedby`.
- **Icon buttons:** always with a descriptive `aria-label`.
- **Touch target:** minimum **24x24px** (WCAG 2.2 AA, 2.5.8); recommendation **44x44px** (WCAG 2.1 AAA, 2.5.5) on the web; **48dp** on Android and **44pt** on iOS.

## 9. Native Mobile (Android/iOS)

Applies to Compose, XML/Views and iOS. Compose-specific implementation belongs to its own skill; what stays here are the visual and platform criteria.

- **Platform first:** Android = Material 3 (Expressive when the project version supports it) + Material Symbols; iOS = HIG + SF Symbols. Never dress one platform in the other's uniform.
- **Native controls:** use `Scaffold`, `NavigationBar`, `ModalBottomSheet`, `TextField`, `LazyColumn` (Compose), `MaterialToolbar`, `BottomNavigationView`, `RecyclerView` (XML) or the iOS equivalents — do not recreate web buttons/divs.
- **Insets and safe areas:** status bar, navigation bar, cutout, keyboard/IME and edge-to-edge on Android 15+; no content under the notch or the keyboard.
- **Back:** predictive back on Android and edge-swipe on iOS; do not hijack the gesture.
- **Touch:** 48dp on Android, 44pt on iOS, with spacing between targets.
- **Typography:** `sp` on Android and Dynamic Type on iOS; test with font scale 1.0, 1.3 and 2.0.
- **Theme:** Material 3 color roles / iOS semantic colors; dynamic color with a static fallback; first-class dark mode.
- **Lists:** virtualize (`LazyColumn`/`RecyclerView`/`List`); no `Column` with 100 items.
- **States:** loading, empty, error, success, offline and permission denied.
- **Preview:** `@Preview` in Compose (light/dark, font scale, phone/tablet) or `tools:` in XML; capture the emulator on phone and tablet.
- **Acceptance:** without a real emulator screenshot, declare "não verificado" [not verified].

## 10. UX Writing

- **CTA = action + object:** "Salvar alterações" [Save changes], not "Enviar" [Send] or "Continuar" [Continue].
- **Error = what happened + how to fix it:** never just "Algo deu errado" [Something went wrong].
- **Empty = what it is + next step:** do not leave the screen dead.
- **Success = what changed:** "Convite enviado" [Invitation sent], not "Sucesso" [Success].
- **Consistency:** the "Publicar" [Publish] button produces the "Publicado" [Published] toast; the same vocabulary from the start to the end.
- **Voice:** sentence case, second person, the domain vocabulary; "Seamlessly/Elevate/Unlock/Supercharge" prohibited.

## 11. Acceptance

A screen is only ready when **all** the items are true:

- [ ] Direction (§1) declared and followed; precedence respected.
- [ ] No deny-list item (§2) without a written justification.
- [ ] Semantic tokens; zero loose hex in the markup.
- [ ] All component states (§6), including success, implemented.
- [ ] WCAG AA verified (§8) and the platform's touch targets checked.
- [ ] Web: 1440px and 390px without breaking or horizontal scroll. Mobile: phone/tablet, light/dark and font scale 1.0/1.3/2.0.
- [ ] `prefers-reduced-motion` / Reduce Motion respected.
- [ ] UX writing revised (§10): CTA, error, empty and success with the domain vocabulary.
- [ ] **Print test:** comparing the screenshot with 1 real product of the same category, there is at least one visual decision (typography, color, density or composition) that a ready-made template would not have. If there is none, redo it.
- [ ] **On an existing screen:** the blast radius (§1.1) was respected — no mixture of two styles on the same screen.
- [ ] **Evidence:** screenshot attached or "não verificado" [not verified] declared; without it, the screen is not ready.
