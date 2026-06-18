#!/usr/bin/env bash
# Self-healing runner for the Claude Code hook binaries in this repo.
#
# The global ~/.claude/settings.json hooks invoke compiled binaries from
# bin/. Those are dev artifacts (gitignored, wiped by `just clean`), so a
# missing binary used to make every hook fail with "No such file or
# directory". This wrapper rebuilds the requested binary on demand, then
# execs it so its real exit code — including the intentional blocking
# exit 2 from validators — is preserved.
#
# Usage (from a hook command): scripts/hook-runner.sh <cmd-name> [args...]
#
# Never wedges tools: if the build can't run (e.g. go missing) the wrapper
# falls through and allows the action rather than blocking on a build error.
set -u

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
name="${1:?hook-runner: missing binary name}"
shift

bin="$repo/bin/$name"

if [ ! -x "$bin" ]; then
    # Build with stdin detached so the tool-call JSON on our stdin stays
    # intact for the exec below; swallow build output to keep hooks quiet.
    if ! (cd "$repo" && go build -o "$bin" "./cmd/$name") </dev/null >/dev/null 2>&1; then
        exit 0
    fi
fi

exec "$bin" "$@"
