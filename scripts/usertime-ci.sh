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
  python3 - "$output" "$base" <<'PYREPORT'
import json,sys,subprocess
from pathlib import Path
r=dict(schema=1,artifact_status="fail",accuracy_status="unverified",base_commit=sys.argv[2],head_commit=subprocess.check_output(["git","rev-parse","HEAD"],text=True).strip(),error="Build or setup failed before comparison; see job log")
p=Path(sys.argv[1]);(p/"report.json").write_text(json.dumps(r,indent=2)+"\n");(p/"report.md").write_text("# User-time accuracy report\nArtifact: **fail**; hardware accuracy: **unverified**\n"+r["error"]+"\n")
PYREPORT
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
python3 scripts/usertime.py compare --head "$head_binary" "${base_args[@]}" --base-commit "$base" --reference evaluation/usertime/performance-v1.json --output "$output/report.json" ${device_args[@]+"${device_args[@]}"}
