# Platform rules for single posts

Per-platform rules for the `social-post` skill: how one post is shaped for each
platform's fold, length, link, and hashtag norms. Load the row for the target
platform before drafting, and check the draft against it before returning — don't
draft platform limits from memory.

## Constraint table

| Platform  | Length (hard)           | Fold / first visible | Links                             | Hashtags                | Formatting                          |
| --------- | ----------------------- | -------------------- | --------------------------------- | ----------------------- | ----------------------------------- |
| LinkedIn  | 3,000 chars             | ~200 chars / 3 lines | First comment (not body)          | 0–3, end, specific      | Plain text — markdown shows literal |
| X         | 280 chars               | whole post visible   | One, end of post                  | 0–1, inline             | Plain text                          |
| Instagram | 2,200 caption           | ~125 chars           | No body links — bio / 1st comment | 3–5, end or 1st comment | Plain text; line breaks matter      |
| Facebook  | 63,206 (truncates ~250) | ~250 chars           | Inline OK; link card preferred    | 0–2                     | Plain text                          |
| Threads   | 500 chars               | whole post           | Inline OK, 1 per post             | ≤1 topic tag            | Plain text                          |
| Bluesky   | 300 chars               | whole post           | Inline native                     | 0–2 (weak signal)       | Plain text                          |
| Mastodon  | 500 chars               | whole post           | Inline native                     | 2–4, CamelCase          | Plain text; CW when warranted       |

## Notes

- **Fold** is the make-or-break line: the hook must land a complete idea before the first-visible cutoff (LinkedIn ~200, Instagram ~125, Facebook ~250). Legal length and a surviving hook are two separate checks.
- **Links:** where a platform penalizes body links (LinkedIn, Instagram), move the link to the first comment or bio and provide that text. Say why once.
- **Hashtags** are platform culture, not decoration: Instagram and Mastodon expect a small set; X and Threads punish them; LinkedIn tolerates ≤3. Always specific, never generic.
- **Character counting:** X bills URLs at 23 chars and most emoji at 2; Mastodon bills URLs at 23; everyone else counts raw length.
