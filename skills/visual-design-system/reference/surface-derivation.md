# Surface derivation (direction → number)

Complements §3 (tokens) and §5 (space/density). It is not a catalog of styles — it is the conversion of
"como isso deve parecer" [how that should look] into the four numbers the agent gets wrong by omission: **radius, shadow, border and
delta between surfaces**.

Sophistication is not more effect. It is fewer elements with more precision: a short radius scale, one
shadow per elevation, a single border language. That shows up measured below.

## 1. Reference measurement (it is not taste, it is production CSS)

Source: the public CSS bundle of each product, search for `border-radius`, `box-shadow`, `border`,
`font-size` and hex. Reproducible with `curl` + regex; no value below was estimated.

| | GOV.UK | IBM Carbon | Shopify Polaris | Radix Themes |
| --- | --- | --- | --- | --- |
| Profile | restrained/editorial | dense/technical | consumer/commerce | expressive/consumer |
| `border-radius` | **1 declaration in the whole CSS** (≈ zero) | 2px×10, 4px×8, 8px×3, 12px×2, 16px×4 | tokens 0/2/4/6/8/12px | 6 levels 3/4/6/8/12/16px, all ×`--radius-factor` |
| Shadow | 18 declarations, **all** with `#fd0` (focus ring); zero card shadow | 80 declarations, elevation resolved by a 1px border | 3 tiers: `--p-shadow-100/200/300` | 6 tiers; tier 2 has **5 layers** (ring + 4 offsets) |
| Border | structure via `border-bottom`, 41 declarations | 408 declarations, 50 in `1px solid` | 121 declarations, all by width token | 7 in total; the ring lives inside the `box-shadow` |
| Families | 1 (GDS Transport) | 2 (Plex Sans + Mono) | 2 (sans + mono) | 1 + mono |
| Font scale | 12–48px, base **19px**, 12 steps | 12–60px, 13 steps | by token, 1 literal value (15px) | by token |

Three consequences that go against instinct:

1. **The most sophisticated product does not have more radius.** GOV.UK has zero and is a craft
   reference. Carbon uses 2–4px and reserves 8–16 for overlay. Large radius is a sign of consumer, not of quality.
2. **Shadow is the most elastic axis** (0 → 3 → 6 tiers) and the easiest to get wrong: Radix compensates the
   radius excess with 5-layer shadows; Carbon forgoes shadow and uses a border.
3. **The level of expression shows up in the number of layers, not in the blur.** `0 10px 30px` with 4%
   alpha is a choice, not a rule — the measured goes from 1px/6% to 5 layers.

**Real case measured (APK `minhas-financas`, via `apktool`, 1036 colors, 40k files).** It serves as
evidence of drift, not as a model: 16 radius values in use (0/2/3/4/5/8/10/12/16/18/20/22/24/
28/36/56) against the limit of 3; elevation 142×`0dp`, 7×`1dp`, 2×`4dp`; border 18×`1dp`; three competing background
tokens (`#ffffff`, `#dedede`, `#e3e3e3`) with card `#f8fafd`; 20 distinct `textSize` in
use; `colorPrimary #1e88e5` (3.68:1 over white) and `colorAccent #0f9d58` (3.51:1); no
`values-night/colors.xml`, that is, dark with no token of its own. Result: consumer-profile radius,
restrained-profile elevation, no scale — the combination that §2 below prohibits.

## 2. Axes → values

Choose two axes before writing the first class. Do not invent a fifth level.

**The axes are independent, not a ladder.** You can have a consumer radius (16–24px) with dense
elevation (0) — it is the most common pattern in third-party Android apps. In that case the separation has to come
from the delta between surfaces (≥ 8%) or from a 1px border; high radius + zero shadow + delta below 5% is
the "caixa de wireframe arredondada" [rounded wireframe box], which combines the worst of both profiles.

| Axis | Restrained | Dense | Consumer | Expressive |
| --- | --- | --- | --- | --- |
| Radius (base / overlay) | 0 / 2 | 2 / 4 (8–16 only overlay) | 8 / 12–16 | 12 / 16–24, themeable |
| Radius levels in the product | 1–2 | 2 | 3 | up to 6, via variable |
| Shadow | none; elevation via border or background | 1 almost invisible tier (`0 1px 2px`, α ≤ .06) | 2–3 diffuse tiers, α ≤ .08 | 3 tiers, multilayer |
| Border | 1px `--line` as structure | 1px on everything that separates | only on input and divider | almost none; ring in the shadow |
| Canvas→surface delta | ≤ 3% luminance | 4–6% | 8–12% | 10%+ with color, not gray |
| Type scale | 1.25, base 18–19px | 1.2, base 14–16px | 1.25, base 16px | 1.333, display tracking −0.03em |
| Weights per screen | 2 | 2 | 2 | 2–3 |

Rules that follow from this:

- **At most 3 radius values per product** (base, small, overlay). Four is scale leakage;
  six requires that all of them come from a variable (the Radix case).
- **One shadow per elevation, and choose one mechanism**: either border, or shadow. The two together with equal
  weight are the "card soup" of §2.
- **Dark mode swaps shadow for border** (§3); therefore the shadow can never be the only carrier of
  hierarchy — the surface contrast has to carry it alone.

## 3. Two levels of variation (do not confuse)

- **Between products** the system varies: palette, radius, shadow, density, scale.
- **Between screens of the same product** the composition and the anchor vary (see
  [product-archetypes.md](product-archetypes.md)); **never** the palette, radius or shadow. A tab with a shadow
  different from the other is a product with four aesthetics.

## 4. Three worked examples

**Dense internal tool.** Radius 4px on everything, overlay 8px. One shadow `0 1px 2px rgba(0,0,0,.06)`.
1px `--line` border on every separation. Canvas `#f7f7f7`, surface `#ffffff` (delta ~4%). Scale 1.2,
base 14px, Mono only on id and technical value. List density: 40–44px row height.

**Welcoming consumer app.** Radius 16px on card, 20px on hero, 999px on pill (3 levels). Shadow
`0 10px 30px rgba(16,38,26,.045)` only on the hero; cards without shadow, separated by background. Canvas and
surface with 8–10% delta, both tinted on the brand hue. Scale 1.25, base 16px, two weights.

**Calm editorial.** Radius 0, zero card shadow, elevation via alternating background. 1px border only on a
horizontal divider. Canvas `#f3f2f1`, surface `#ffffff`. Scale 1.25, base 19px, measure of 65ch.

In all three, before delivering: measure the text pairs over canvas and over surface (§3) and check
that the chosen shadow is not the only signal of hierarchy.

## 5. Prohibited

- Delivering this reference's palette as reusable tokens. The values above are **ranges** and
  serve to calibrate; brand hex coming from here is the same error as "escolher a visão 7" [choosing option 7] in a catalog.
- Reusing another product's numbers just because the segment is similar. Carbon is dense because it is
  a cloud console; the other project's dense product may be dense for another reason.
