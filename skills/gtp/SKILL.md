---
name: gtp
description: Grill the product. Run when the user says "gtp" or /gtp, or asks to grill a product, game, mod, app, site or tool from the user's side. Ask the hard questions a would-be player, user or downloader would ask (why would I use, play, download or keep this?), then run a separate pass on user experience, attention and retention. Answer both honestly from the actual product, then fix the problems and gaps that show up.
license: MIT
metadata:
  author: kevlar10x
  version: "1.2.0"
  repository: https://github.com/kevlar10x/gtp
---

# GTP: grill the product

Look at the current product from the side of the person deciding whether to bother with it. The goal is to find every reason someone would scroll past, bounce, or uninstall, and then remove those reasons. Every run has two passes: the first asks whether someone would pick it up, the second whether it holds them once they have.

## 1. Know the product

Read what exists: code, files, mod page or store page, readme, screenshots, and what's been discussed. Work out who it's for and where they'd come across it (Nexus, GitHub, a store, a link someone shared).

## 2. First pass: would I bother?

Ask what a real person asks, in the order they meet the product. Pick the 12 to 16 that matter most for this one:

- **Find:** Would I notice it? Do the title and thumbnail say what it is in two seconds?
- **Why:** What itch does it scratch? Why this over the alternatives, or over doing nothing (vanilla game, another mod, an app I already have)?
- **Trust:** Does it look safe and looked after? Real screenshots? Clear requirements, compatibility and version?
- **Try:** How long from download to the good part? Install steps, setup, first run, first minute.
- **Value:** What's the moment that makes it worth it, and is it obvious or buried?
- **Friction:** What breaks, confuses or annoys? Conflicts with other mods, saves or game updates? Does it uninstall cleanly?

For games, also ask: is the first minute fun, is the goal clear, does the difficulty fit?

## 3. Second pass: user experience, attention, retention

Run this every time, as its own pass with its own list. It picks up where the first pass stops, with someone already using the product. Pick the 8 to 12 that matter most:

- **First look:** Where do my eyes land? Is the main thing the biggest thing on screen? What could go?
- **Next step:** Do I always know what to do next? Does every action show that it worked?
- **Flow:** Where do I wait, reread or hunt for something? Which steps could go?
- **Attention:** When would I get bored or look away? How soon does something good happen, and how often after that?
- **Recovery:** When I get something wrong, can I see why and get back without losing anything?
- **Comfort:** Can I read it and use it on my screen, with my input, at my sound level?
- **Return:** Why open it again tomorrow? What's waiting for me, unfinished or new?
- **Keep:** What would make me remove it after a week? Would I tell someone about it?

For games, also ask: does the pacing hold after the first ten minutes, do hits and wins feel good, is there a reason to play again?

Retention means being worth coming back to. Leave out streak guilt, fake urgency, nag prompts and anything else that keeps people by annoying them.

## 4. Answer them yourself, honestly

Answer each from the real product, not from what it was meant to be. A weak answer, "not sure" or "no" marks a gap. Don't go easy on it: the user runs this to find problems before players do.

## 5. Fix the gaps

Fix everything that's safe to fix now: code, defaults, UX, mod page or readme copy, install steps, compatibility notes, screenshot plan. The usual rules still apply: humanizer for public writing, impeccable or anti-slop-art for visuals, publishing rules before anything goes public, and verify fixes before calling them done.

Gaps that need money, a public post, or a change to what the product fundamentally is: list them for the user instead of acting.

## 6. Suggest what's next

Once the gaps are dealt with, list 3 to 5 new things worth building, the kind that give someone a new reason to pick it up or come back. Rank them by how much a user would notice against how much work they are, one line each. Don't build them unless the user says so.

## 7. Report compactly

```
GTP: <product>
1. <question> → <honest answer> → fixed: <what>
2. <question> → needs you: <what to do>

UX: <product>
1. <question> → <honest answer> → fixed: <what>
2. <question> → needs you: <what to do>

Next:
1. <idea>: <why it's worth it> (small / medium / big)
```

A needs-you line skips the answer and says what to do in six plain words or fewer.

Then one line on what changed.
