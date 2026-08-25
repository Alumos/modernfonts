#!/bin/sh
set -eu

data_dir="${DATA_DIR:-/app/data}"
mkdir -p "$data_dir"

needs_chown=false
if ! su-exec appuser test -w "$data_dir"; then
  needs_chown=true
fi

for path in "$data_dir"/app.db "$data_dir"/app.db-* "$data_dir"/jwt-secret "$data_dir"/uploads "$data_dir"/uploads/*; do
  [ -e "$path" ] || continue
  if ! su-exec appuser test -w "$path"; then
    needs_chown=true
    break
  fi
done

if [ "$needs_chown" = true ]; then
  chown -R appuser:appuser "$data_dir"
fi

exec su-exec appuser "$@"
