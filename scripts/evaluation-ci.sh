#!/usr/bin/env bash
# Shared local/CI entry point. Always write a report after a test run, even on failure.
set -euo pipefail
cd "$(dirname "$0")/.."
tier=${1:-short}
output=${2:-evaluation-results}
mkdir -p "$output"
case "$tier" in
  static)
    go build ./cmd/...
    go vet ./...
    go run ./cmd/evaluate manifest -out "$output/manifest.json"
    # Historical files are not all gofmt-clean. Check the new evaluator, and
    # changed Go files when CI supplies a comparison base, without reformatting history.
    files=$(gofmt -l internal/evaluation cmd/evaluate)
    if [[ -n "$files" ]]; then printf '%s\n' "$files"; exit 1; fi
    if [[ -n "${FORMAT_BASE:-}" ]]; then
      if [[ "$FORMAT_BASE" =~ ^0+$ ]]; then FORMAT_BASE=$(git hash-object -t tree /dev/null); fi
      git rev-parse --verify "$FORMAT_BASE" >/dev/null
      changed=$(git diff --name-only --diff-filter=ACMR "$FORMAT_BASE" HEAD -- '*.go')
      while IFS= read -r file; do
        [[ ! -f "$file" ]] && continue
        [[ -z "$(gofmt -l "$file")" ]] || { echo "Not gofmt clean: $file"; exit 1; }
      done <<< "$changed"
    fi
    ;;
  short|full)
    args=(-json -count=1 -timeout 45m)
    [[ "$tier" == short ]] && args+=(-short)
    set +e
    go test "${args[@]}" ./... >"$output/go-test.jsonl" 2>"$output/go-test.stderr"
    test_status=$?
    go run ./cmd/evaluate regression -tier "$tier" -log "$output/go-test.jsonl" -out "$output/report.json"
    report_status=$?
    set -e
    if (( test_status != 0 || report_status != 0 )); then
      cat "$output/go-test.stderr"
      echo "Tests failed or coverage incomplete; inspect $output"
      exit 1
    fi
    ;;
  *) echo "usage: $0 static|short|full [output-directory]" >&2; exit 2 ;;
esac
