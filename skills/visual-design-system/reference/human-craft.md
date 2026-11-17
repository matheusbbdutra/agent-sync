# Human craft — the positive side of the deny-list

§2 of the skill lists what betrays a generated UI. This file is the other half: what makes a screen look
**authored** by someone who has seen the product's users.

The acceptance test is already in §11 ("one visual decision a ready-made template would not have"). This
file is how you produce that decision instead of hoping for it.

Book basis (by topic, not page): Hicks, *The Icon Handbook* — drawing a family that belongs to a product;
Wathan & Schoger, *Refactoring UI* — the details that separate a designed screen from a styled one;
Bringhurst, *The Elements of Typographic Style* — typographic detail as a signal of care.

## Imperfection with a budget

Perfect symmetry and uniform spacing read as machine output. Deliberate irregularity reads as human —
**in small doses, with a reason**.

| Move | Budget | Example |
|---|---|---|
| One asymmetric element | 1 per screen | a hero image that bleeds off the right edge instead of centering |
| Hand-drawn annotation | 1 per flow, at most | an arrow pointing at the one thing that matters |
| Off-grid nudge | only where optics demand | optical alignment of a quote mark or an icon |
| Rotation | ≤ 2°, and never on text | a photo card tilted as if placed on a desk |
| Texture | opacity ≤ 5% | paper grain on a reading surface, not on the whole app |

The budget matters more than the moves. Two of these on one screen is style; five is noise and reads
*worse* than a clean template.

## Texture and material

- A single low-opacity grain layer over a large flat area adds depth that flat design cannot fake.
- Do not apply gradients as decoration — a full-bleed purple-to-blue gradient is the single most reliable
  "generated" tell.
- Real shadows come from one light source. Two shadows in conflicting directions are physically
  impossible and the eye catches it before the mind does.
- Borders, when used, should have one role: separate two surfaces of the same elevation. If they also
  decorate, remove them.

## Illustration and icon authorship

- **Prefer one authored asset over ten stock ones.** A single custom illustration, reused, builds identity.
- If nobody draws: derive abstract shapes from the domain (a card stack for installments, a ledger line
  for a statement) instead of generic "people with laptops".
- Custom icons belong to the family: same 24×24 grid, same 2 px stroke, same optical weight
  (`iconography.md`).
- A domain noun that the icon family does not have is an argument to draw one — that is precisely the
  detail a template cannot contain.

## Typographic detail

- **Real quotes**, not straight ones. Curly quotes, proper apostrophes, en dash for ranges, em dash for
  breaks.
- **Optical margins**: a heading that starts with "T" or "J" needs negative margin so it aligns visually
  with the body below.
- **Tracking by size**: large display type wants tighter tracking; small caps and all-caps labels want
  looser tracking plus a smaller size.
- **Hanging punctuation** in pull quotes and lists.
- Limit yourself to **one display face**; a second family is usually a second personality.
- Numbers in tables: tabular figures, right-aligned, same decimals.

## The "would a template have this?" test

Run this before accepting a screen — each "yes" is one point of authorship:

- [ ] There is an element that only this product's domain would justify (icon, shape, label, unit).
- [ ] One spacing decision is deliberately uneven because the content demanded it.
- [ ] The type has at least one detail a CSS reset would not give you (optical margin, hanging quote,
      tabular figures).
- [ ] There is exactly one textured or authored asset, not a set of generated ones.
- [ ] The empty state says something specific about this product, not "no data".
- [ ] Comparing with a competitor's screenshot, at least one element is not interchangeable.

Zero points → redo with one deliberate decision, then re-run. Do not add five at once.

## Fake humanity — worse than none

| Anti-pattern | Why it fails |
|---|---|
| Handwritten font for body copy | unreadable at length; reads as a theme, not a product |
| Global noise/grain over every surface | hides hierarchy instead of adding depth |
| "Organic blobs" behind every card | decoration with no informational role |
| Fake sticky notes / tape / paperclips | costume, not craft — the content stops being the protagonist |
| Random rotations on a grid of cards | looks accidental, not authored |
| Emoji as iconography | breaks platform consistency (`iconography.md`) |

The distinguishing question: **does the irregularity carry information or attention, or is it there to
look handmade?** Only the first is craft.
