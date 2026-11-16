#!/usr/bin/env node
// Deterministic pre-flight for a social post draft. Computes platform-accurate
// character counts and flags the mechanical problems a human shouldn't eyeball —
// length, truncation, link placement, hashtag count/position, engagement bait,
// and formatting gotchas. Judgment calls (hook strength, media alt text, voice)
// stay with the agent; this script handles what a machine does better.
//
// Usage:
//   bun scripts/lint-post.mjs --platform x --text "your draft"
//   bun scripts/lint-post.mjs -p linkedin --file draft.txt
//   cat draft.txt | bun scripts/lint-post.mjs --platform instagram
//   bun scripts/lint-post.mjs --all --file draft.txt      # every platform
//   bun scripts/lint-post.mjs --list                      # supported platforms
//
// Output: JSON to stdout — { platform, counts, checks: [{check, verdict, detail}], summary }.
//   verdict is "pass" | "warn" | "fix". Diagnostics go to stderr.
// Exit codes: 0 = no fix-level issues, 1 = at least one fix, 2 = usage error.
import { readFileSync } from "node:fs";

const LIMITS = JSON.parse(
  readFileSync(
    new URL("../references/platform-limits.json", import.meta.url),
    "utf8",
  ),
);

const HELP = `lint-post — deterministic pre-flight for a social post draft

Usage:
  lint-post --platform <name> --text "<draft>"
  lint-post -p <name> --file <path>
  cat draft.txt | lint-post --platform <name>
  lint-post --all --file <path>
  lint-post --list

Options:
  -p, --platform <name>   Platform to check (${Object.keys(LIMITS).join(", ")})
      --all               Check the draft against every platform
  -t, --text <string>     Draft text inline
  -f, --file <path>       Read the draft from a file
      --list              List supported platforms and exit
  -h, --help              Show this help

Exit codes: 0 = clean (pass/warn only), 1 = at least one fix, 2 = usage error.`;

function parseArgs(argv) {
  const args = {
    platform: null,
    all: false,
    text: null,
    file: null,
    list: false,
    help: false,
  };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === "-h" || a === "--help") args.help = true;
    else if (a === "--list") args.list = true;
    else if (a === "--all") args.all = true;
    else if (a === "-p" || a === "--platform") args.platform = argv[++i];
    else if (a === "-t" || a === "--text") args.text = argv[++i];
    else if (a === "-f" || a === "--file") args.file = argv[++i];
    else return { error: `unknown argument: ${a}` };
  }
  return args;
}

const URL_RE = /https?:\/\/[^\s)]+/g;
const HASHTAG_RE = /(?<![\w&])#[A-Za-z0-9_]+/g;
const EMOJI_RE = /\p{Extended_Pictographic}/gu;
const BAIT_RE =
  /\b(comment\s+["“]?(yes|below)|tag\s+(a|someone|three)|drop\s+a\s+\W?\s*(emoji|below)|like\s+(and|&|if)|follow\s+(to|for\s+more|me\s+for)|repost\s+if|dm\s+me\s+["“]?\w+["”]?\s+(to|for))\b/i;
const SMART_QUOTES = /[‘’“”]/;
const NBSP = / /;
// Unicode "math bold/italic" alphanumerics screenreaders spell out letter by letter.
const FANCY_ALNUM = /[\u{1D400}-\u{1D7FF}]/u;

/** Count characters the way the platform does (URLs and emoji get special weights).
 *  Pure arithmetic — no placeholder strings, so a formatter can't silently change the math. */
function countChars(text, platform) {
  let count = [...text].length; // code points, so astral chars count once
  let urlCount = 0;
  if (platform.urlWeight != null) {
    for (const url of text.match(URL_RE) ?? []) {
      urlCount++;
      count += platform.urlWeight - [...url].length; // swap actual length for billed width
    }
  }
  if (platform.emojiWeight && platform.emojiWeight !== 1) {
    const emoji = (text.match(EMOJI_RE) ?? []).length;
    count += emoji * (platform.emojiWeight - 1);
  }
  return { count, urlCount };
}

function checkPlatform(text, key) {
  const p = LIMITS[key];
  const checks = [];
  const add = (check, verdict, detail) =>
    checks.push({ check, verdict, detail });

  const { count, urlCount } = countChars(text, p);
  const links = text.match(URL_RE) ?? [];
  const hashtags = text.match(HASHTAG_RE) ?? [];
  const firstLine = text.split(/\r?\n/, 1)[0] ?? "";

  // length
  if (count > p.charLimit) {
    add(
      "length",
      "fix",
      `${count}/${p.charLimit} chars — over by ${count - p.charLimit}`,
    );
  } else {
    const weighted =
      p.urlWeight != null && urlCount > 0
        ? ` (URLs billed at ${p.urlWeight})`
        : "";
    add("length", "pass", `${count}/${p.charLimit} chars${weighted}`);
  }

  // truncation / hook above the fold
  if (p.truncateAt != null && count > p.truncateAt) {
    const verdict = firstLine.length > p.truncateAt ? "warn" : "pass";
    add(
      "truncation",
      verdict,
      verdict === "warn"
        ? `first line is ${firstLine.length} chars but the feed cuts to "…see more" around ${p.truncateAt} — the hook is buried`
        : `post runs past the ~${p.truncateAt}-char fold, but the first line (${firstLine.length} chars) survives it`,
    );
  }

  // links
  if (links.length > 0) {
    if (p.links.maxLinks != null && links.length > p.links.maxLinks) {
      add(
        "links",
        "fix",
        `${links.length} links; ${p.name} allows ${p.links.maxLinks}. ${p.links.advice}`,
      );
    } else if (p.links.inBody === "fix") {
      add("links", "fix", `${links.length} body link(s). ${p.links.advice}`);
    } else if (p.links.inBody === "warn") {
      add("links", "warn", `${links.length} body link(s). ${p.links.advice}`);
    } else {
      add("links", "pass", `${links.length} link(s) — within norm`);
    }
  }

  // hashtags: count, position, casing
  if (p.hashtags.max != null && hashtags.length > p.hashtags.max) {
    add(
      "hashtags",
      "warn",
      `${hashtags.length} hashtags; ${p.name} norm is ≤ ${p.hashtags.max}`,
    );
  } else if (p.hashtags.min && hashtags.length < p.hashtags.min) {
    const sev = p.hashtags.discoveryCritical ? "warn" : "pass";
    add(
      "hashtags",
      sev,
      `${hashtags.length} hashtags; ${p.name} norm is ${p.hashtags.min}–${p.hashtags.max}`,
    );
  } else if (hashtags.length > 0) {
    add("hashtags", "pass", `${hashtags.length} hashtags`);
  }
  if (p.hashtags.position === "end" && hashtags.length > 0) {
    const tail = text.slice(Math.floor(text.length * 0.8));
    const strays = hashtags.filter((h) => !tail.includes(h));
    if (strays.length > 0) {
      add(
        "hashtag-placement",
        "warn",
        `${p.name} expects hashtags at the end; found mid-post: ${strays.join(" ")}`,
      );
    }
  }
  const clumpy = hashtags.filter((h) => h.length > 11 && h === h.toLowerCase());
  if (clumpy.length > 0) {
    add(
      "hashtag-casing",
      "warn",
      `use CamelCase for multi-word tags (legibility + screenreaders): ${clumpy.join(" ")}`,
    );
  }

  // engagement bait
  if (BAIT_RE.test(text)) {
    add(
      "engagement-bait",
      "warn",
      "reads as engagement bait — platforms downrank it; ask for genuine input instead",
    );
  }

  // formatting integrity
  if (FANCY_ALNUM.test(text)) {
    add(
      "formatting",
      "fix",
      "Unicode bold/italic letters — screenreaders spell them out one by one; use plain text",
    );
  } else if (
    SMART_QUOTES.test(text) ||
    NBSP.test(text) ||
    /[ \t]+$/m.test(text)
  ) {
    add(
      "formatting",
      "warn",
      "smart quotes, non-breaking spaces, or trailing whitespace can render oddly across clients",
    );
  }
  if (/\n{3,}/.test(text)) {
    add(
      "formatting",
      "warn",
      "3+ consecutive blank lines collapse differently per platform",
    );
  }

  const summary = { pass: 0, warn: 0, fix: 0 };
  for (const c of checks) summary[c.verdict]++;
  return {
    platform: key,
    name: p.name,
    counts: {
      chars: count,
      limit: p.charLimit,
      links: links.length,
      hashtags: hashtags.length,
    },
    checks,
    summary,
  };
}

const args = parseArgs(process.argv.slice(2));
if (args.error) {
  console.error(`${args.error}\n\n${HELP}`);
  process.exit(2);
}
if (args.help) {
  console.log(HELP);
  process.exit(0);
}
if (args.list) {
  console.log(
    JSON.stringify(
      Object.entries(LIMITS).map(([k, v]) => ({
        platform: k,
        name: v.name,
        charLimit: v.charLimit,
      })),
      null,
      2,
    ),
  );
  process.exit(0);
}

let text = args.text;
if (text == null && args.file != null) {
  try {
    text = readFileSync(args.file, "utf8");
  } catch (error) {
    console.error(`cannot read --file ${args.file}: ${error.message}`);
    process.exit(2);
  }
}
if (text == null && !process.stdin.isTTY) {
  try {
    text = readFileSync(0, "utf8");
  } catch {
    text = "";
  }
}
if (text == null || text.trim().length === 0) {
  console.error(
    `no draft provided — pass --text, --file, or pipe via stdin\n\n${HELP}`,
  );
  process.exit(2);
}

const targets = args.all ? Object.keys(LIMITS) : [args.platform];
if (!args.all) {
  if (!args.platform) {
    console.error(`--platform is required (or use --all)\n\n${HELP}`);
    process.exit(2);
  }
  if (!LIMITS[args.platform]) {
    console.error(
      `unknown platform "${args.platform}". Supported: ${Object.keys(LIMITS).join(", ")}`,
    );
    process.exit(2);
  }
}

const results = targets.map((key) => checkPlatform(text, key));
const output = args.all ? { results } : results[0];
console.log(JSON.stringify(output, null, 2));

const anyFix = results.some((r) => r.summary.fix > 0);
process.exit(anyFix ? 1 : 0);
