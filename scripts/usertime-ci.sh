#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
base=${1:?base revision required}
output=${2:-evaluation-results/usertime}
# Optional modeled device (evaluation/devices.json); the default device if omitted.
device_args=()
if [ -n "${3:-}" ]; then device_args=(--device "$3"); fi
mkdir -p "$output"
rm -f "$output/report.json" "$output/report.md"
base=$(git rev-parse "$base^{commit}")
# The target branch may have advanced since this branch diverged. Compare
# its exact supplied revision, without requiring ancestry.
head_binary=$(mktemp /tmp/poros-user-head.XXXXXX)
base_binary=$(mktemp /tmp/poros-user-base.XXXXXX)
base_tree=$(mktemp -d /tmp/poros-user-base-tree.XXXXXX)
cleanup() {
 status=$?
 if [ "$status" -ne 0 ] && [ ! -f "$output/report.json" ]; then
  # The shared failure record, filed under the device this run was asked for.
  python3 scripts/accuracy_site.py failure --output "$output" --commit "$(git rev-parse HEAD)" --base-commit "$base" \
   --message "Build or setup failed before comparison; see job log" ${device_args[@]+"${device_args[@]}"}
 fi
 git worktree remove --force "$base_tree" >/dev/null 2>&1 || true
 rm -f "$head_binary" "$base_binary"
 rmdir "$base_tree" >/dev/null 2>&1 || true
}
trap cleanup EXIT
go build -o "$head_binary" ./cmd/poros
if git cat-file -e "$base:pkg/elfsim/run.go" 2>/dev/null; then
 git worktree add --detach "$base_tree" "$base"
 (cd "$base_tree" && go build -o "$base_binary" ./cmd/poros)
 base_args=(--base "$base_binary")
else
 base_args=(--base-unavailable)
fi
python3 scripts/usertime_test.py
python3 scripts/compute_usertime_test.py
# The comparison takes the application reference its device declares.
python3 scripts/usertime.py compare --head "$head_binary" "${base_args[@]}" --base-commit "$base" --output "$output/report.json" ${device_args[@]+"${device_args[@]}"}
