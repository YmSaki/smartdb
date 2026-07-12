# CLAUDE.md

Guidance for Claude Code sessions working in this repo.

## Project

SmartDB: a self-hosted, SQLite-as-a-service platform (Go). See `docs/spec.md` for
the full requirements doc, `docs/api.md` for the HTTP API reference, and
`docs/auth.md` for the original auth design memo.

## Build / test

```bash
go build ./...
go vet ./...
go test ./...
task fmt    # go fmt ./...
task lint   # golangci-lint run (may not run in some environments — see below)
```

CI (`.github/workflows/ci.yml`) enforces `go build`, `go vet`, `go test ./... -v` on
push/PR to `master`. `golangci-lint` is a local convenience only, not CI-enforced —
if the environment's binary predates the repo's Go version it will refuse to run;
note that rather than skip the whole quality gate.

## Skills

- `codebase-review` — persona-driven review that verifies suspected bugs with a
  throwaway repro before filing issues, and checks for duplicates first.
- `resolve-issue` — picks up one open issue, re-verifies its plan against current
  code, implements, and ships a PR. **Read it before doing multi-issue "issue
  消化" work** — it now includes the backlog-tracking rule below.

## Session retrospective notes (read before doing multi-issue work)

These are corrections received from the repo owner during actual sessions, kept
here so they don't have to be re-learned. Follow them as durable rules, not just
context for the specific incident.

1. **Track the backlog explicitly — never prioritize open issues from memory.**
   A verified, self-diagnosed High-severity bug (#15) was silently dropped
   partway through a multi-issue "work through the backlog" session because
   priority ordering was being held in conversation context instead of a
   tracked list. When resolving more than one or two issues in a sitting, use
   `TaskCreate`/`TaskList` to hold the full open-issue set up front, and check
   items off as PRs merge — don't re-derive "what's next" from memory each
   time.

2. **Fix recurring automation friction at the root, not by hand each time.**
   The automated PR-review bot (`.github/workflows/auto-pr-review.yml`) tried
   and failed to file follow-up issues four separate times because the
   `from-review` label didn't exist and `gh label create` wasn't in its
   `--allowedTools`. Each time, the fix was to manually file the issue by
   hand instead of closing the actual gap. If you hit the same tooling/CI
   failure twice, stop and fix the underlying cause (workflow config, missing
   label, missing permission) rather than continuing to patch around it —
   you very likely own the thing that's broken (this repo's CI workflows were
   authored by a prior Claude Code session, not a third party).

3. **Re-check shared-infrastructure changes against every other issue touched
   in the same session, not just the one you're solving.** Adding a
   deleted/wiped-project check to `auth.RequireProjectAccess` (for #26)
   initially blocked System Key's emergency apikeys access — a behavior
   *just* established as intentional in #14/#25 earlier the same session.
   The automated reviewer caught it, but it should have been caught before
   pushing: when a fix touches code that other recently-resolved issues also
   depend on, explicitly re-derive whether they still hold, don't rely solely
   on CI/review to catch cross-issue regressions.

4. **When you add a check to shared middleware, audit callers for now-redundant
   work.** `RequireProjectAccess` gained a `project.GetProject` lookup for
   #26; `GetProjectDetailHandler` and `PatchProjectHandler` already did their
   own `project.GetProject` call downstream, so those routes now query the
   same row 2-3× per request. Not incorrect, but it's the kind of local
   patch that a full pass would have consolidated — at minimum, note it
   (a follow-up issue is fine) rather than letting it go unnoticed.

5. **Use git worktrees to parallelize independent fixes instead of serializing
   everything through one branch.** For genuinely independent issues, spawn a
   background `Agent` with `isolation: "worktree"` and have it own its own
   branch/PR end-to-end, rather than doing every fix sequentially on a single
   shared branch.
