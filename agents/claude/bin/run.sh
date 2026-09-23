#!/bin/sh
# Builds the hook on first use and whenever its source changes; a hook that
# cannot build lets every edit through rather than blocking the session.
src="$(dirname "$0")/../hook"
data="${CLAUDE_PLUGIN_DATA:-${TMPDIR:-/tmp}/waffle-plugin}"
bin="$data/waffle-hook"
sum="$(cat "$src"/*.go "$src/go.mod" | cksum)"

if [ ! -x "$bin" ] || [ "$(cat "$bin.sum" 2>/dev/null)" != "$sum" ]; then
	mkdir -p "$data"
	if ! GOWORK=off go -C "$src" build -o "$bin.$$" . 2>"$data/build.log"; then
		rm -f "$bin.$$"
		if [ "$1" = session-start ]; then
			echo "waffle plugin: its hook did not build (see $data/build.log), so Rule #11 modes are not enforced in this session."
		fi
		exit 0
	fi
	mv "$bin.$$" "$bin"
	echo "$sum" >"$bin.sum"
fi
exec "$bin" "$@"
