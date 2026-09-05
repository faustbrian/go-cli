#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repository_root}"

untracked_go="$({
  git ls-files --others --exclude-standard -- '*.go'
  git ls-files --others --ignored --exclude-standard -- '*.go'
} | LC_ALL=C sort -u)"
if [[ -n "${untracked_go}" ]]; then
  printf 'benchmark inputs include untracked or ignored Go files:\n%s\n' "${untracked_go}" >&2
  exit 1
fi

benchmark_inputs() {
  {
    git ls-files -- '*.go' | awk '!/_test\.go$/ || /^benchmarks\//'
    printf '%s\n' \
      go.mod \
      go.sum \
      go.work \
      benchmarks/go.mod \
      benchmarks/go.sum \
      benchmarks/competitors.txt \
      scripts/capture-benchmark-evidence.sh \
      scripts/benchmark-input-fingerprint.sh \
      verification/package.mk
  } | LC_ALL=C sort -u
}

benchmark_manifest() {
  benchmark_inputs | while IFS= read -r path; do
    if [[ ! -f "${path}" ]]; then
      printf 'missing benchmark input: %s\n' "${path}" >&2
      exit 1
    fi
    digest="$(shasum -a 256 "${path}" | awk '{print $1}')"
    printf '%s  %s\n' "${digest}" "${path}"
  done
}

case "${1:-}" in
  '') benchmark_manifest | shasum -a 256 | awk '{print $1}' ;;
  --manifest) benchmark_manifest ;;
  *) printf 'usage: %s [--manifest]\n' "${0##*/}" >&2; exit 2 ;;
esac
