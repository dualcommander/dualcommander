#!/bin/sh
set -eu
root=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/go" <<'EOF'
#!/bin/sh
set -eu
[ "$*" = 'install fyne.io/tools/cmd/fyne@v1.7.3' ] || exit 99
count=$(cat "$STATE")
count=$((count + 1))
printf '%s\n' "$count" > "$STATE"
[ "$count" -gt "$FAILURES" ]
EOF
cat > "$tmp/sleep" <<'EOF'
#!/bin/sh
printf '%s\n' "$1" >> "$DELAYS"
EOF
chmod +x "$tmp/go" "$tmp/sleep"
export STATE="$tmp/state" DELAYS="$tmp/delays"
for failures in 0 1 2 3; do
    printf '0\n' > "$STATE"
    : > "$DELAYS"
    status=0
    PATH="$tmp:$PATH" FAILURES="$failures" sh "$root/scripts/install-packager.sh" || status=$?
    if [ "$failures" -lt 3 ]; then
        [ "$status" -eq 0 ]
        [ "$(cat "$STATE")" -eq "$((failures + 1))" ]
    else
        [ "$status" -eq 1 ]
        [ "$(cat "$STATE")" -eq 3 ]
    fi
    case "$failures" in
        0) expected='' ;;
        1) expected='5' ;;
        *) expected=$(printf '5\n10') ;;
    esac
    [ "$(cat "$DELAYS")" = "$expected" ]
done
printf 'Packager install checks passed\n'