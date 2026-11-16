# Platform limits & conventions

The authority behind the `lint-post.mjs` checks, plus the norms a script can't
judge. Load this when you need the reasoning behind a flag, the media/aspect
rules, or the per-platform link and hashtag conventions. The machine-readable
copy the linter reads is [`platform-limits.json`](platform-limits.json) — keep
the two in sync.

## Constraint table

| Platform  | Char limit                       | Link norm                            | Hashtag norm                       | Media norm                                                   |
| --------- | -------------------------------- | ------------------------------------ | ---------------------------------- | ------------------------------------------------------------ |
| LinkedIn  | 3,000 (truncates ~210 / 3 lines) | First comment or below the fold      | ≤ 3, end, CamelCase                | 1.91:1–1:1 image; document posts 1:1.294; alt ≤ 300 chars    |
| X         | 280 (URLs = 23, emoji = 2)       | End of post, max 1                   | 0–1, inline                        | 16:9 or 1:1; up to 4 images; alt ≤ 1,000 chars               |
| Instagram | 2,200 caption (truncates ~125)   | No body links — bio or first comment | 3–5, end or first comment          | 1:1, 4:5, or 1.91:1; Reels 9:16; alt text supported — use it |
| Facebook  | 63,206 (truncates ~250)          | Inline OK; link card preferred       | 0–2                                | 1.91:1 link card; 1:1 / 4:5 feed image                       |
| Threads   | 500                              | Inline OK, 1 per post                | ≤ 1 topic tag                      | 1:1 or 4:5; up to 20 media items                             |
| Bluesky   | 300                              | Inline native                        | Weak signal; ≤ 2                   | 1:1 / 4:3; up to 4 images; alt text strongly normed          |
| Mastodon  | 500 default (URLs = 23)          | Inline native                        | 2–4, discovery-critical, CamelCase | Alt text strongly normed; flag its absence                   |
| Reddit    | título 300; corpo 10,000          | Inline OK, contexto antes do link    | 0 — hashtag não funciona aqui      | Link post ou galeria; título carrega o hook                  |

## Character counting

Count the way each platform does, not the way a text editor does — this is what
the linter automates:

- **X** counts every URL as 23 characters and most emoji as 2.
- **Mastodon** counts all links as 23 characters.
- Everyone else counts raw length.

A post can be under the hard limit and still bury its hook below the
visible-truncation point (LinkedIn ~210 / 3 lines, Facebook ~250, Instagram
~125). Legal length and a surviving hook are two separate checks.

## Link placement, by platform

- **LinkedIn:** move body links to the first comment or at least below the fold — a naked URL mid-hook reads badly. Show both placements.
- **X:** one link, at the end. Never two in one post.
- **Instagram:** body links are dead text — rewrite to "link in bio" or a first-comment note.
- **Facebook:** inline is fine; a clean link-card preview beats a raw URL.
- **Bluesky / Mastodon:** inline links are native and fine.

## Hashtags, by platform

Enforce CamelCase for multi-word tags — it is both legibility and screenreader
correctness (`#SocialMedia`, not `#socialmedia`).

- **Instagram:** 3–5 relevant tags at the end or in the first comment.
- **LinkedIn:** ≤ 3 at the end.
- **X:** 0–1, inline.
- **Threads:** at most one topic tag.
- **Bluesky:** weak signal — warn if the draft leans on them.
- **Mastodon:** discovery-critical — warn if there are none.

## Media & accessibility

- Aspect ratio suits the placement (see the table).
- Alt text exists and actually describes the image — "image" or a keyword-stuffed sentence is a `fix`.
- Text-heavy images mirror their text in the post body or alt text, or screenreader users get nothing.
- Emoji are not used as bullet points (each is read aloud) and not so frequent they break the reading flow.

## Reddit (fonte: código do próprio Reddit)

- **Título** limitado a **300** caracteres (`reddit-archive/reddit`, `r2/r2/lib/validator/validator.py:603`,
  `max_length = 300`). O título é o hook inteiro: não existe "corpo acima do fold".
- **Corpo** de self post declarado com `max_length=10000` no mesmo arquivo (linha 655). Trate como piso
  conservador e confirme no compositor do sub antes de colar texto longo.
- **Hashtag não é cultura** da plataforma: `hashtags.max = 0`.
- **Link** é esperado e não penalizado como no LinkedIn — o problema aqui é link nu; o corpo carrega
  o argumento e o link vem com contexto.
- **Flair** é exigência por subreddit, não global: a publicação falha silenciosamente sem ela em
  vários subs. Confira a regra do sub antes de compor.
- **Regras do sub valem mais que a convenção da plataforma.** Leia as regras antes de publicar, não
  depois do bloqueio.

## Verdict discipline

| Verdict | Meaning                                      | What it must include                                                  |
| ------- | -------------------------------------------- | --------------------------------------------------------------------- |
| `pass`  | Meets the constraint                         | Nothing beyond the check name                                         |
| `warn`  | Will publish, but will underperform or annoy | One-clause reason + suggested improvement                             |
| `fix`   | Don't post as-is                             | The corrected text inline — a `fix` without it is an unfinished check |

If a check doesn't apply (no media, no links), say `n/a` rather than dropping the
row — a silently skipped check reads as a pass.
