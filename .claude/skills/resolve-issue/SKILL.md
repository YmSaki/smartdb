---
name: resolve-issue
description: Pick up one open GitHub issue on the smartdb repo and carry it through to a pushed, tested fix — re-verify the issue's plan against current code, implement minimally, run the build/vet/test gate, commit referencing "closes #N", push, and open a draft PR. Use when asked to work through the issue backlog, "issue消化", implement a specific issue number, or continue after codebase-review has filed issues.
---

# Resolve One Issue

Turns one open GitHub issue into a merged-ready PR. Handles exactly one issue
per run unless the user explicitly asks to batch several — issues here touch
auth/SQL execution/backup, so keep diffs reviewable and don't silently stack
unrelated fixes into one commit.

## 1. Pick the issue

If the user named a number, use it. Otherwise call `mcp__github__list_issues`
(state OPEN) and pick by severity: `Security/Critical` > `Bug/Critical|High` >
`Spec Gap/Security` > `Perf` > `Spec Gap` > `Bug/Low` > test-coverage issues.
Check for dependency notes in the issue body (e.g. "this is a prerequisite
for #8") — don't start a blocked issue before its prerequisite lands.

Read the full issue with `mcp__github__issue_read` even if you just listed
it — the body carries the actual work plan under "想定される修正方針".

## 2. Re-verify the plan against current code

Issues here were written as handoff instructions, sometimes by an earlier
session — the codebase may have moved since. Before writing any code:
- Read every file listed under "影響範囲".
- Confirm the line numbers / function names / snippets quoted in the issue
  still match. If they've drifted, adapt the plan, don't blindly patch by
  line number.
- If the issue presents a 方針A/方針B fork (a genuine product decision, not
  an engineering one), stop and ask the user which direction before writing
  code — don't guess on their behalf. Issues that explicitly say "着手前に
  方針だけ確認することを推奨" are flagging exactly this.

## 3. Branch

Check `git status` and current branch first. If a task/session has already
pinned a working branch (check for explicit branch instructions in the
conversation), keep using it — don't create a second branch for a single
issue's worth of work. Otherwise create `claude/issue-<N>-<short-slug>` off
the latest default branch.

## 4. Implement minimally

Follow the issue's numbered plan, but stay inside its actual scope — no
drive-by refactors, no fixing unrelated issues in the same commit even if
you notice them (file a new issue instead, or mention it to the user).
Match existing code conventions in the touched package (error wrapping
style, `slog` usage, handler patterns in `internal/api/v1`, etc. — read a
neighboring file in the same package before writing if unsure).

Add/update the tests the issue's plan calls for. If the issue includes a
reproduction snippet (from `codebase-review`'s verification step), turn it
into a permanent test rather than re-deriving one.

## 5. Quality gate — must pass before committing

Mirror what CI (`.github/workflows/ci.yml`) actually enforces, plus the
Taskfile conveniences:

```bash
go build ./...
go vet ./...
go test ./... -v
task fmt    # go fmt ./...
task lint   # golangci-lint run — fix findings; don't suppress without cause
```

If `golangci-lint` isn't installed in the environment, note that in your
summary rather than skipping silently — `go vet`/`go test` are the hard
requirements (CI enforces those; lint is best-effort locally).

Also run `git status --short` — no leftover scratch/db files from manual
testing should be staged or left untracked.

## 6. Update docs if the issue says to

If "影響範囲" lists `docs/spec.md` or `docs/api.md`, update them in the same
commit — stale docs are exactly the class of bug this backlog exists to fix,
don't reintroduce it.

## 7. Commit, push, PR

- Commit message: short, why-focused, ending with `(closes #N)` per this
  repo's existing convention (see `git log --oneline`).
- Push with `-u origin <branch>`.
- Open a **draft** PR. Check for a PR template first (none exists in this
  repo as of now — write Summary + Test plan sections). Reference the issue
  number so GitHub auto-links/closes it on merge.
- Report back concisely: what changed, the test/lint results, and the PR
  link. Don't re-explain the issue's contents back to the user — they wrote
  the plan (or reviewed it already).

## Notes

- This repo has no `sdb-cli` yet and Migration is unwired (#8/#9) — don't
  assume CLI entry points exist just because `docs/spec.md` describes them.
- Security-sensitive issues (SQL classifier, auth, lock registry) warrant
  an extra read-through of the diff against the exact attack scenario in
  the issue's "再現" section before calling it done — passing tests alone
  isn't sufficient evidence for those.
