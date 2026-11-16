# Consumer Fintech — personal finance app

Read this reference when the screen is about personal finance: wallet, cards, budget,
installments, cash flow, invoices. It applies to web (HTML prototype) and native
(Compose/XML/iOS) — declare which before writing code, because navigation and insets differ (§9).

It does not repeat §2 (deny-list), §3 (tokens), §4 (typography) nor §8 (accessibility): they apply
in full here. What this file adds is the domain's **money model**, **formatting** and
**dataviz**.

## 1. Money model (non-negotiable)

Four states, and every number on the screen declares which one it belongs to:

| State | Meaning |
| --- | --- |
| **Efetivado** [settled] | already settled — it landed in or left the account |
| **Previsto** [forecast] | includes scheduled and future items of the cycle; not settled |
| **Reservado** [reserved] | earmarked money (goals, saving jars, emergency fund) |
| **Saldo livre** [free balance] | `Total − Reservado − Faturas abertas` — what you can spend today |

- The word "saldo" [balance] alone is prohibited. It is `saldo efetivado`, `previsto` or `livre`.
- The free balance formula is visible to the user (explanatory card), not only internalized in the code.
- Every derivation that the UI states must exist as a testable function, a single source of truth,
  with the same rule on the Home, on the accounts screen and on the statement. Three divergent implementations is the
  most expensive bug in this domain.
- Edge cases require an explicit answer before coding: negative free balance, an invoice closing
  today, an installment that falls after the cutoff, a reserve in another currency, a refund that arrives in the next cycle.
  Without a defined answer, show the value with an honest label ("pode variar até o fechamento" [may vary until closing]).

## 2. Microcopy per type

| Field | Expense | Income | Transfer |
| --- | --- | --- | --- |
| Date | Due date | Receipt | Date |
| Status | Paid / Pending | Received / To receive | Completed / Scheduled |
| Primary action | Pay | Confirm receipt | Complete |

- A card has no "vencimento" [due date]: it has **fechamento** (cutoff) and **vencimento** (payment). Do not mix them.
- An installment plan talks about **nº de parcelas restantes** [number of remaining installments] and **valor liberado a partir de \<mês\>** [amount released from \<month\>], not about "total".
- No marketing vocabulary: "Economize sem esforço" [Save effortlessly] is not microcopy for a finance app.

**Dictionary (cold → human).** The code and accessibility label stays technical; the
human layer is only the text that appears on the screen:

| Instead of | On screen |
| --- | --- |
| Free balance / Safe to spend | "Para gastar com tranquilidade" [To spend with peace of mind] |
| Expenses by category | "Onde seu dinheiro foi gasto" [Where your money went] |
| Cash flow projection | "Seu dinheiro nos próximos meses" [Your money in the coming months] |
| Pending items / Liabilities | "Próximos compromissos" [Upcoming commitments] |

- One screen, one name. `Saldo livre` is the canonical name (code, formula, `sr-only`); the human phrase
  replaces the visible text, never the accessibility label nor the identifier in the code.
- Do not create a synonym for the same number on different screens: if the Home says "Para gastar com
  tranquilidade" [To spend with peace of mind] and the Accounts screen says "Livre para usar" [Free to use], the user thinks they are two values.

## 3. Value formatting

- `font-variant-numeric: tabular-nums` on **every** monetary value, percentage and list date.
  Without it the column of numbers dances between rows.
- Currency via `Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })`. Never concatenate `"R$ "`.
- Aggregate/KPI: at most 2 decimals; transaction list: 2 fixed decimals (never 1 on one row and 2 on the other).
- Negative with an explicit minus sign **and** color/icon — never in accounting parentheses.
- **Date is not timestamp.** A transaction stores the instant with timezone and displays it in the user's time zone;
  calendar fields store a pure date. `toISOString().slice(0,10)` shifts the day for anyone
  west of UTC — the classic "lançamento no dia anterior" [transaction on the previous day] bug.
- **Privacy mode:** the eye button swaps the value for `••••••` keeping the block width
  (same character count or `min-width`, never `display:none`), without reflow on the cards.
  The hidden value must not go to the screen reader DOM nor to `aria-label`: mark the visible one
  with `aria-hidden` and announce "valor oculto" [hidden value] in an `sr-only`.
- **Vivid accent (lime, coral, amber)** exists only as a **background** with dark ink on top
  (`--accent` + `--accent-ink`, ~10:1). Never as text color, value, border or focus ring:
  lime over white sits at 1.2:1 and fails the 3:1 minimum of SC 1.4.11. A light accent in text
  is the most common palette error in generated UI.

## 4. Financial dataviz

- **A real zero axis.** In a balance/flow projection, positive columns rise above the zero line and
  negative ones descend below it, in `--out`. A horizontal bar filled to the right representing a negative
  balance is wrong — it is the most common error in the domain.
- **Spending pace:** a bar of current spend against the month's ceiling + a vertical
  "Hoje" [Today] marker positioned by `dia_atual / dias_do_mês`. Say in text what the bar shows
  ("gastou 68% do mês em 55% do tempo" [spent 68% of the month in 55% of the time]); a marker without a caption is decoration.
- **A clickable chart** updates a context panel with the cause of the number, not a value tooltip.
- **Color never alone** (WCAG 2.1 SC 1.4.1): inflow/outflow carry a sign, icon or label beyond
  green/red. It applies to projected deficit and to category.
- A doughnut/donut with at most 5 slices + "Outros" [Others]; order by value; a legend with value, not just color.
- Savings percentage always with the denominator explicit ("12% da receita do mês" [12% of the month's revenue]).

## 5. Mobile navigation and density

- A finance app is **Operate**: scannability, consistency and platform expectation above
  expression. Aesthetic gestures are left to the brand, not to the transaction flow.
- 3 to 5 main destinations with a short label. **A single floor of 11.5px** for bar label and
  metadata — 9px and 10.5px do not exist on the phone. One number only, not an "11–11.5" range.
- A category and institution icon is an SVG or a 2-letter monogram over a soft background. System
  emoji (🛒, ☕, 🏍) and Unicode glyph (⌂, ⇄) never: they vary by platform and break optical alignment.
- Content never behind the bottom bar: `padding-bottom` equal to the bar height + inset,
  or native safe area.
- Primary creation action ("novo lançamento" [new transaction]): in native use the Material 3 FAB/`extended FAB` or the
  iOS bar — recreating the bar for an elevated central "+" violates §9. In a web prototype it is allowed,
  provided it is not the same dock on every screen.
- Each screen has its visual anchor (wallet hero, card plastics, donut + goals). Repeating the
  same overline header + giant number + hairline on every tab is the editorial PDF syndrome
  that §2 bans — the anchor changes, the navigation skeleton does not.

## 7. Specific acceptance (beyond §11)

- [ ] Every number on the screen declares the state (efetivado / previsto / reservado / livre).
- [ ] The free balance formula exists in a single place in the code, and a test covers it.
- [ ] `tabular-nums` on values, percentages and list dates.
- [ ] A negative in a chart points down from a real zero axis.
- [ ] Green/red is never the only carrier of meaning.
- [ ] Privacy mode does not shift the layout and does not leak the value into the DOM.
- [ ] Transaction date tested in a time zone other than UTC.
- [ ] Financial dataviz checked in light and dark mode with the real tokens.
