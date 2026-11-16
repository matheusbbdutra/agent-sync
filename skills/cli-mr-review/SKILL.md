---
name: cli-mr-review
description: "Use when reviewing or collecting MR/PR via glab (GitLab) or gh (GitHub). Collection and review of MR/PR via glab (GitLab) or gh (GitHub) with automatic self-hosted host detection."
---

# Skill: MR/PR review via glab/gh

## When to prefer this path over the default `mr-reviewer`

- The user **already has** `glab` or `gh` authenticated and wants to avoid extra JSON config.
- The host is self-hosted and was configured in `~/.config/glab-cli/config.yml` or `gh auth login`.
- Compatibility with **old self-hosted GitLab 12 is not a requirement** — `glab` follows the modern v4 API.

When **NOT** to use: GitLab 12, environments without `glab`/`gh` on the PATH, or when the user explicitly asks for `mr-review-local` (pure HTTP, zero deps).

## Summarized workflow

1. Detect provider: direct question to the user OR infer from the remote (`gitlab.com`/`github.com` in the host).
2. Resolve `repo` (local checkout path) and `mr-iid` (IID/number).
3. Run `mr-collect-cli -provider <gitlab|github> -repo <path> -mr-iid <id> [-host <url>]`.
4. Consume the `reviewChange` JSON the same way `mr-review-local` produces it.

## Troubleshooting

| Symptom | Probable cause | Action |
|---|---|---|
| `glab: command not found` | `glab` not installed | Install via `brew install glab` / `apt install glab` / [releases](https://gitlab.com/gitlab-org/cli/-/releases). No automatic fallback — the user must install it. |
| `401 Unauthorized` | Missing/expired token | `glab auth login --hostname <url>` or `gh auth login`. Do not ask to paste the token in the chat — the `--token` flag does not exist in the secure flow. |
| `mr not found` (GitLab) | Wrong IID OR wrong project | Confirm via `glab mr list --source-branch <branch>` to list the branch's open MRs. |
| `Could not resolve host` | `-host` typo OR self-hosted not configured | Check that `~/.config/glab-cli/config.yml` has the entry `[host "https://gitlab.empresa.com"]`. |
| Patch comes up empty | Diff too large OR the MR is only a rename | `mr-collect-cli -max-bytes` (default 256KB). If still empty, consult the web UI. |
| Error `subcomando glab X não está na allowlist` [error: glab subcommand X is not in the allowlist] | Bug or attempt to run merge/approve | Read-only BY CONSTRUCTION: the binary rejects it. Report as a finding if it comes from the agent, never from the legitimate user. |

## Self-hosted: how the host is detected

`mr-collect-cli` reads `git -C <repo> remote get-url origin` and parses it with two regexes:

- `git@gitlab.empresa.com:group/proj.git` → host `gitlab.empresa.com`
- `https://gitlab.empresa.com/group/proj.git` → host `gitlab.empresa.com`
- `http://gitlab.empresa.com:8080/group/proj.git` → host `gitlab.empresa.com:8080`

To force the host: `-host https://gitlab.empresa.com` (overrides the parse).

## Security

- The token comes from the credential helper / env var configured by `glab auth login` / `gh auth login` itself. The binary **does not read** nor **persist** tokens.
- Hardcoded allowlist of subcommands: `view`, `diff`. Any other (`merge`, `approve`, `close`, etc.) fails before `exec.Command`.
- Redaction via `internal/secretscan` in the patch before emitting JSON (same protection as `mr-review-local`).

## Quick verification

```bash
# Binary available?
which mr-collect-cli || echo "run: make install"

# glab authenticated?
glab auth status

# gh authenticated?
gh auth status

# Does collection work (GitLab)?
mr-collect-cli -provider gitlab -repo /path/to/checkout -mr-iid 42

# Does collection work (GitHub)?
mr-collect-cli -provider github -repo /path/to/checkout -mr-iid 42
```

## References

- [code-reviewer](reference/code-review-checklist.md) — per-domain check catalogue when reviewing a change
- [review-standard](reference/review-standard.md) — approval criterion, finding severity (`Nit:` vs blocking), change size, response deadline, conflict and escalation
