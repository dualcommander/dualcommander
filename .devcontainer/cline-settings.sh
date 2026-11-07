#!/bin/sh
set -eu
umask 077

data=${CLINE_DATA_DIR:-${CLINE_DIR:-$HOME/.cline}/data}
case ${1:-} in
  snapshot) source_dir=$data; target=${2:?Profile path required} ;;
  seed) source_dir=${2:?Profile path required}; target=$data ;;
  *) echo "Usage: $0 snapshot|seed PROFILE" >&2; exit 1 ;;
esac

set -- globalState.json secrets.json settings/providers.json settings/global-settings.json
test -f "$source_dir/globalState.json"
test "$source_dir" != "$target"
test ! -L "$target"
test ! -L "$target/settings"
test ! -L "$target/settings/cline_mcp_settings.json"
for file in "$@"; do
  test ! -L "$target/$file"
  if test -f "$source_dir/$file"; then
    python3 -m json.tool "$source_dir/$file" >/dev/null
  fi
done

mkdir -p "$target/settings"
chmod 700 "$target" "$target/settings"
for file in "$@"; do
  rm -f "$target/$file"
  if test -f "$source_dir/$file"; then
    install -m 600 "$source_dir/$file" "$target/$file"
    if test "$target" = "$data"; then
      sed -i 's/localhost/host.docker.internal/g' "$target/$file"
    fi
  fi
done

if test "$target" = "$data"; then
  printf '%s\n' '{"mcpServers": {}}' >"$target/settings/cline_mcp_settings.json"
  chmod 600 "$target/settings/cline_mcp_settings.json"
fi