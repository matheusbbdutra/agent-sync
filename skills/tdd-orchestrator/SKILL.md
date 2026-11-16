---
name: tdd-orchestrator
description: "Use when implementing features or fixing bugs via Red-Green-Refactor TDD cycles. TDD cycle (Red-Green-Refactor) with edge case coverage and test-guided design. discipline, multi-agent workflow coordination, and comprehensive test-driven development practices. Enforces TDD best practices across teams with AI-assisted testing and modern frameworks. Use PROACTIVELY for TDD implementation and governance."
---

## Use this skill when

- You are going to write/update tests in any module of this repo.
- Implements logic that does not yet have coverage (red-green-refactor).
- You are going to refactor code that already has tests (regression safety net).

## Do not use this skill when

- Task is purely documentation/markdown.
- Coverage already exists and the change is trivial (typo, log, copy).
- Problem is harness/orchestration (use `agent-react` or `debugging-strategies`).

## Mechanics in this repo

1. **Test first.** Do not write implementation without a corresponding failing test.
2. **Location.** Tests in the same package as the code: `pkg.go` → `pkg_test.go`. Coverage per command: `tools/cmd/<cmd>/`.
3. **Commands.** `go test ./...` for everything; `go test -run TestX ./pkg/` for a target; `go test -race ./...` before a PR with concurrency.
4. **Coverage.** No numeric threshold enforced today. Focus on edge cases and error paths — not on %.
5. **Mocks.** Prefer interfaces and `testify/mock` or manual mocks. `gomock` rarely needed.
6. **Table-driven.** When ≥3 similar test cases, use `[]struct{...}{...}` + `t.Run`.

## Anti-patterns (reject before PR)

- Test that does setup but never asserts.
- Test that depends on execution order.
- Sleep/time.Sleep to synchronize (use channels or polling).
- High coverage via trivial tests (getter smoke tests).
- Test that ignores the error (`_, _ = fn()`).
