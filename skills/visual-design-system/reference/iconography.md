# Iconography

Sources for the numbers below: IBM Carbon icon usage (`carbondesignsystem.com/elements/icons/usage/`)
and the Tabler Icons specification (24×24 grid, 2 px stroke, MIT). Everything else here is craft
practice, not a vendor rule.

## One family, or none

- **One icon family per product.** Mixing families shows immediately: different corner radius,
  different stroke, different optical weight.
- Permissive families, all safe to embed: Tabler (MIT), Heroicons (MIT), Feather (MIT), Lucide (ISC).
- **Never emoji as a UI icon.** It renders differently per platform, cannot inherit color or size,
  and breaks the family. Emoji in a *label* is copy, not iconography.
- Emoji-free does not mean character-free: a monogram (two letters over a soft shape) is a legitimate
  avatar/fallback when no icon exists.

## Grid and stroke

| Rule | Value | Why |
|---|---|---|
| Drawing grid | **24 × 24** | multiples of 4 keep the family aligned at every scale |
| Stroke | **2 px** | consistent weight across the set; 1.5 px looks thin at 16 px |
| Sizes shipped | **16, 20, 24, 32** | Carbon's artboard set; pick one per context and stay there |
| Color | `currentColor` | icon inherits text color — never hardcode a hex in the SVG |
| Fill | solid, monochrome | color is a state signal, not decoration |

```svg
<svg width="24" height="24" viewBox="0 0 24 24" fill="none"
     stroke="currentColor" stroke-width="2"
     stroke-linecap="round" stroke-linejoin="round">
```

## Alignment

- Beside text, the icon is **centered with the text block**, not aligned to the baseline. Baseline
  alignment is the single most common reason an icon row looks "off by 1px".
- **Optical size is not geometric size.** At the same 24 px box, a circle reads smaller than a square;
  a triangle reads smaller than both. Nudge the shape, not the box.
- Icon + label is the default. **Icon alone only when** the meaning is universal in the domain and the
  control has an accessible name (`aria-label`, or visible tooltip on hover/focus).
- Touch/click target **≥ 44 px**, even when the glyph is 16 px. The padding belongs to the button, not
  to the SVG.

## Color and contrast

- Icons carry information, so they obey text contrast: **4.5:1** against the background (Carbon).
- When an icon is paired with text, **use the text color**. A lighter icon next to darker text reads as
  disabled.
- Color must never be the only carrier of meaning: pair with shape, sign or label (WCAG 1.4.1).

## Sizes per context

| Context | Size | Note |
|---|---|---|
| Dense table / inline with 14 px text | 16 | the smallest that survives a 2 px stroke |
| Form field, list row | 20 | default for touch and pointer both |
| Toolbar, tab bar, nav | 24 | the family's native grid |
| Empty state, feature card | 32 | beyond this, it is illustration, not icon |

## Drawing an icon that does not exist

1. Draw on the 24 × 24 grid, keep the 2 px stroke, snap endpoints to the grid.
2. Compare side by side with 3 existing icons **at 16 px** — that is where inconsistency shows.
3. Keep one idea per icon. If it needs two shapes overlapping to be understood, the label is missing.
4. Export as SVG with `currentColor`, no inline `width`/`height` in the path, `viewBox` intact.
5. Add it to the same folder as the family, named by intent (`filter-active`, not `icon-23`).

## Checklist

- [ ] A single family; no emoji standing in for a UI icon.
- [ ] 24×24 grid and 2 px stroke in the source; `currentColor`, no hardcoded fill.
- [ ] One size per context, from the 16/20/24/32 set.
- [ ] Icon centered with its text block, not baseline-aligned.
- [ ] Icon-only controls have an accessible name and a ≥ 44 px target.
- [ ] Contrast ≥ 4.5:1 when the icon carries information.
- [ ] New icons compared against the family at 16 px before merging.
