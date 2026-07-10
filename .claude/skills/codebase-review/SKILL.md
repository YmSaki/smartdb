---
name: codebase-review
description: Review the smartdb codebase for bugs, spec/implementation gaps, and improvements, then file well-structured GitHub issues written as handoff instructions for a follow-up implementation session. Verifies suspected bugs with a throwaway reproduction before filing anything, and checks existing open issues first to avoid duplicates. Use when asked to review this project and share findings/impressions, "bug hunt", or open issues for later implementation. Does not modify product code — research and issue-filing only.
---

# Codebase Review → Issue Filing

Read-only investigation of `smartdb` that ends in GitHub issues, not code changes.
Goal: every issue filed should be something a fresh Sonnet session can pick up and
resolve using `resolve-issue` without needing to re-derive context.

## 1. Ground yourself in the spec

Read, in order:
- `README.md`
- `docs/spec.md` (the authoritative behavior contract — cross-reference every
  finding against this)
- `docs/api.md`
- `docs/auth.md` if present

Mismatches between these documents and the code are first-class findings
("Spec Gap"), not just bugs.

## 2. Check what's already tracked

Before reading code, call `mcp__github__list_issues` (state OPEN) to see what's
already been filed. Don't re-file a known issue — if you find corroborating
evidence for an existing one, that's not a new issue.

## 3. Survey the code area by area

Don't grep blindly. Walk the packages that matter for a self-hosted SQLite
platform, in this order of leverage:
1. `internal/auth` — role checks, token handling, bootstrap
2. `internal/api/v1` — request handlers (cross-check against `docs/api.md`)
3. `internal/project` — SQL execution, stats, pooling
4. `internal/backup` — backup/restore/retention/scheduler
5. `internal/ats` — SQL classifier/lexer (security-relevant: this is the
   privilege gate for `POST /projects/{project}/sql`)
6. `internal/domain`, `internal/config`, `internal/handler` — cross-cutting
   concerns (locking, timeouts, validation, CORS, body limits)
7. `cmd/smartdb` — wiring; look for things implemented but never called
   (dead code is a real finding — see #8, #13 as examples)

Read actual file contents with Read, not just filenames. A plausible-looking
function name is not evidence of correct behavior.

## 4. Verify before filing — don't guess

Any suspected concrete bug (not a design/spec-gap judgment call) MUST be
verified with a throwaway reproduction before it goes in an issue:
- For SQL/logic bugs: write a `_test.go` (or a scratch `go test` snippet) that
  exercises the suspect code path and shows the actual vs. expected output.
  Prefer a temporary test file inside the relevant package
  (`internal/project/scratch_x_test.go`) so it uses the real project module
  and dependencies — put throwaway non-Go scratch files in the scratchpad
  directory instead.
- Run it, capture the real output, and quote it in the issue body.
- **Clean up afterward**: delete the scratch test file and any DB files it
  created (`system.db`, `system.db-wal`, `system.db-shm`, temp dirs). Run
  `git status --short` before filing anything — it must be clean (no
  untracked droppings from your own verification).

Findings that are genuinely judgment calls (e.g. "should the spec or the
implementation change?") don't need a repro — say so explicitly and present
the options instead of asserting one is "correct".

## 5. File issues in the established format

Use `mcp__github__issue_write` (method: create). Title prefix by category:
`[Bug/Critical|High|Low]`, `[Security/Critical|High]`, `[Spec Gap]`,
`[Perf]`, `[Spec Gap/Security]` for overlap cases.

Body template (Japanese, matches existing issues #8–#17 — keep the voice
consistent with the rest of the tracker):

```markdown
## 概要
What's wrong, with the actual code snippet quoted (not paraphrased).

## 再現  (omit this section if not a verified bug — see step 4)
The reproduction you ran and its real output.

## 影響
Concrete consequence: which endpoint/flow breaks, under what condition,
and why it matters for a self-hosted SQLite platform specifically
(data isolation, corruption risk, spec's own success criteria, etc.).
Reference the relevant spec section (e.g. "spec.md §11") when applicable.

## 想定される修正方針（Sonnetへの作業指示）
Numbered, concrete steps a fresh session can execute without re-reading
the whole codebase: exact files, exact functions, what test to add and
what it should assert. If there's a real design fork (e.g. "fix the code"
vs "fix the spec"), present it as 方針A/方針B with tradeoffs and say the
implementer should confirm direction before starting — don't just pick one
silently when it's a product decision, not an engineering one.

## 影響範囲
Bullet list of files that will need to change.
```

## 6. Wrap up

Summarize to the user in prose (not just a bare list) — this is meant to
read like a stakeholder's honest impression of the project, not a linter
report. State what you liked, then the issues filed with numbers and a
one-line severity/summary each. Keep code changes at zero; if the user
wants a fix, that's a separate `resolve-issue` pass.
