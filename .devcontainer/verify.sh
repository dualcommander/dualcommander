#!/bin/sh
set -eu
cd /workspaces/dualcommander
[ "$(id -u)" -ne 0 ]
grep -Eq '^CapEff:[[:space:]]+0+$' /proc/self/status
grep -Eq '^CapBnd:[[:space:]]+0+$' /proc/self/status
grep -Eq '^NoNewPrivs:[[:space:]]+1$' /proc/self/status
[ -z "${SSH_AUTH_SOCK:-}" ]
[ ! -S /var/run/docker.sock ]
[ ! -S /run/docker.sock ]
[ ! -e /home/developer/.ssh ]
[ ! -e /home/developer/.aws ]
[ ! -e /home/developer/.kube ]
[ -d .git ] || {
    echo 'Use a standalone clone: external Git worktrees/submodule metadata are not mounted.' >&2
    exit 1
}
[ "$(git rev-parse --show-toplevel)" = /workspaces/dualcommander ]
echo 'Sandbox checks passed: non-root, no capabilities, no privilege escalation, repository root verified.'