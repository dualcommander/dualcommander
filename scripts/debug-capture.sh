#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
exe=${EXE:-dist/dualcommander$(go env GOEXE)}
dir=$(dirname "$exe")
log=$dir/dualcommander-debug.log
stderr_log=dist/dualcommander-stderr.log
excerpt=dist/dualcommander-debug-excerpt.txt
[ -x "$exe" ] || { echo "Cannot run $exe (make build, or set EXE=path)." >&2; exit 1; }
mkdir -p dist
rm -f "$log" "$stderr_log" "$excerpt"
echo 'Reproduce the blank pane, then quit right after the key press:'
echo '  1. Navigate a pane to your home folder.'
echo '  2. Note whether the whole pane is empty or rows are present but blank.'
echo '  3. Press one key (pane fills in), then quit with Cmd+Q.'
DUALCOMMANDER_DEBUG=1 "$exe" 2>"$stderr_log" || true
[ -s "$log" ] || { echo "No debug log written to $log." >&2; exit 1; }
{
    date -u +'# %Y-%m-%dT%H:%M:%SZ'
    uname -srm
    if command -v sw_vers >/dev/null; then sw_vers; fi
    go version
    git rev-parse --short=12 HEAD
    echo '## Message counts'
    sed -nE 's/.* msg=("[^"]*"|[^ ]*).*/\1/p' "$log" | sort | uniq -c | sort -rn
    echo '## View size, scroll and cursor visibility (all list state lines)'
    grep -E 'msg="list state"' "$log" |
        sed -E 's/ (name|selected)=("([^"\\]|\\.)*"|[^ ]*)//g' || true
    echo '## Non-debug stderr (first 50 lines)'
    grep -v 'level=DEBUG' "$stderr_log" | head -50 || true
    echo '## From last navigate, file names removed (first 300 lines)'
    awk '/ msg=navigate /{n=NR} {l[NR]=$0} END{for(i=n?n:1;i<=NR;i++)print l[i]}' "$log" |
        sed -E 's/ (name|selected)=("([^"\\]|\\.)*"|[^ ]*)//g' | head -300
} >"$excerpt"
echo "Review $excerpt before sharing: it contains folder paths."
echo "Full log, keep local: $log"