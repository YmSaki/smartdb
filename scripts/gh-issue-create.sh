#!/usr/bin/env bash
# Fixed-argument wrapper for `gh issue create`, used by
# .github/workflows/auto-pr-review.yml's review agent to file follow-up
# issues.
#
# The point of this wrapper (as opposed to allowlisting `gh issue create`
# itself with a wildcard) is that it never exposes gh's file-reading flags
# (--body-file / -F) to the agent. auto-pr-review.yml checks out and reads
# unreviewed PR content, so a prompt-injected review could otherwise be
# steered into running `gh issue create -F <runner file>` to exfiltrate
# arbitrary file contents (e.g. secrets available to the job) into a public
# issue. Title and body are always passed as literal argv values here,
# which bash never re-splits into additional flags, so no combination of
# their content can smuggle an extra gh flag through.
#
# Usage: gh-issue-create.sh <title> <body>
set -euo pipefail

if [ "$#" -ne 2 ]; then
    echo "usage: $0 <title> <body>" >&2
    exit 1
fi

exec gh issue create --title "$1" --body "$2" --label from-review
