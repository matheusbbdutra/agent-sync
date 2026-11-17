# Prototype fidelity

A prototype exists to answer **one question**. If you cannot say which question, the right fidelity is
"none" — go talk to the user instead.

Book basis (cited by chapter, not page): Buxton, *Sketching User Experiences* — the point of a sketch is
to think, not to present; Snyder, *Paper Prototyping* — paper tests screen structure before any pixel
exists; McElroy, *Prototyping for Designers* — each fidelity level buys one kind of answer and costs one
kind of effort.

## The ladder

| Level | Cost | Answers | Cannot answer | Tool (verified license) |
|---|---|---|---|---|
| **Paper / whiteboard** | minutes | does the flow make sense? where does the eye go? | spacing, type, color | pen, or Excalidraw (MIT) |
| **Low-fi wireframe** | tens of minutes | hierarchy, density, navigation, empty/error states | any visual judgment | Excalidraw, tldraw (custom SDK license), Figma |
| **Sketched hi-fi** | hours | does it read as a product? is the direction right? | real motion, real data | code, or hand-drawn render via Rough.js (MIT) |
| **Interactive prototype** | days | can a person finish the task without help? | performance, production edge cases | the real UI with faked data |
| **Production** | weeks | everything | — | — |

Rules that survive the ladder:

- **Climb one rung at a time.** Jumping from paper to polished code skips the step where direction is
  decided and makes every later change expensive.
- **Never polish below the question.** Pixel-perfect spacing on a wireframe whose job is "does the flow
  work" is wasted money and, worse, invites feedback on the wrong thing.
- **Throwaway by default.** A prototype that must be maintained has stopped being a prototype.
- **Fake data must be boring.** Real-looking data hides label and empty-state problems.

## Why hand-drawn works for prototypes

A hand-drawn surface signals "not finished" and lowers the cost of criticism. That is the whole point:
stakeholders critique the idea, not the kerning.

Two MIT tools cover it:

- **Excalidraw** — whiteboard with a hand-drawn rendering mode; the `.excalidraw` file is yours, JSON,
  and diffable.
- **Rough.js** — renders shapes to SVG/canvas with a sketchy appearance; use it when you want the
  hand-drawn look inside a page you already build.

Do not ship the sketchy look in production: at the point where the user must trust the product, texture
competes with content.

## Screen prototyping order that avoids rework

1. **Flow first** — the sequence of screens, one box per screen, arrows only where the user decides.
2. **Structure per screen** — one primary action, one scan path, three zones (top: context, middle:
   content, bottom: action).
3. **States** — empty, loading, error, success, partial. A screen without its empty and error state is
   half a screen and will be built wrong.
4. **Density** — decide whether the screen breathes or packs before choosing components. This is where
   most "looks generated" screens come from: everything at the same airiness.
5. **Type and color** — only now, from the token system (§3 of the skill).
6. **Motion** — last, and only where it communicates a state change.

## Mobile-specific rules

- **Thumb zone**: primary action in the lower third; destructive or rare actions out of it.
- **One hand**: nothing that requires a reach to the top corner for a frequent action.
- **Fixed bottom bar** must not cover content: reserve its height in the scroll container.
- **Safe areas**: respect the notch and the home indicator; `env(safe-area-inset-*)` or the platform
  equivalent — a prototype without it will look broken on the target device.

## When not to prototype

- The pattern is settled and identical to an existing screen → build it with the existing components.
- The decision is about copy, not structure → write the copy and read it aloud.
- The question is performance → prototype the load, not the screen.
- The stakeholder already decided and wants validation → the prototype will not survive contact; the
  conversation must come first.

## Checklist

- [ ] The prototype answers one named question.
- [ ] The fidelity is the cheapest that can answer it.
- [ ] Hand-drawn/tool file is checked in or linked; it is not a screenshot.
- [ ] Every screen has empty, loading and error states.
- [ ] Density was decided before components were chosen.
- [ ] Mobile: thumb zone, safe areas, bottom bar height reserved.
- [ ] It is explicit what is faked, and nobody will mistake it for the finished product.
