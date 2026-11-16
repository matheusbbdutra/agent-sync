# Screen archetypes (optional catalog)

Catalog to decide **the hierarchy of each tab**, not to assemble the whole app from it.
The five archetypes below correspond to different mental models: overview, past,
inventory, comparison and creation. Choose by what the tab answers, not by what already exists in the code.

**Symptom of wrong use:** every tab with the same skeleton (sometimes an uppercase overline, sometimes a
giant number, sometimes a 1px hairline, sometimes all three). That is the editorial PDF syndrome of §2 —
§5 of SKILL.md already requires a varied rhythm; here the variation is structural.

| Archetype | Structure | Watch out |
| --- | --- | --- |
| **Overview** | High-contrast anchor (hero) with a scope switcher + rhythm/progress indicator + 2–4 metrics in an asymmetric grid + interactive chart + short list of what is coming | Turning into a dashboard of identical KPIs. The anchor must answer "como estou" [how am I doing], not "quanto tem" [how much is there] |
| **Feed / history** | Compact summary of the period at the top + search + pill filters + list grouped by date | An infinite list with no empty state and no grouping is a disguised table |
| **Object management** | Segmented tabs at the top + cards that represent the real object + progress/limit bar + direct action on the card | A card with no visual representation of the object (only text and a generic icon) loses the archetype |
| **Analytics** | Proportion (donut/comparative bar) next to the key metric + list comparing actual vs. target | Never color alone: a proportion needs a label and a value; a donut above 5 slices becomes noise |
| **Creation / form** | Type selector at the top + central stage for the main datum + fields in tactile blocks with a label that changes per type | A fixed label in a polymorphic form is the classic error ("Data" [Date] when it should be "Recebimento" [Receipt]) |

**Detail is not a tab.** Item detail is a layer (bottom sheet, drawer, child route) opened from
a feed, object or chart. A detail tab without an object has no anchor and breaks the mental model.

## Application rules

- **One tab, one archetype.** Two tabs with the same archetype are candidates for merging.
- **The anchor changes, the navigation skeleton does not:** bar, header and tokens remain; what varies
  is the composition of the content. Shell consistency and content variation coexist.
- **Each archetype inherits the full lifecycle of §6** (loading, empty, error, success) when the tab
  depends on data. An archetype without states is a mock-up.
- **Describe hierarchy, not component.** "Hero com o saldo do período" [Hero with the period balance] leaves room for the brand;
  "a `rounded-2xl shadow-lg` card with a 40px number" is the template that §2 bans.
- **Mobile:** one archetype per viewport. A vertical information ladder, never two columns of
  content on the phone.
