#!/usr/bin/env bash
set -euo pipefail

version="${1:?release version is required}"
output="${2:?output directory is required}"

if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "release version must be a v-prefixed semantic version" >&2
  exit 1
fi
if [[ -e "${output}" ]]; then
  echo "release output already exists: ${output}" >&2
  exit 1
fi

repository="$(git rev-parse --show-toplevel)"
tag="refs/tags/${version}"
if [[ "$(git -C "${repository}" cat-file -t "${tag}" 2>/dev/null || true)" != "tag" ]]; then
  echo "release version must identify an annotated repository tag: ${version}" >&2
  exit 1
fi
revision="$(git -C "${repository}" rev-parse --verify "${tag}^{commit}")"

mkdir -p "${output}"
name="cli-${version}"
git -C "${repository}" archive --format=tar --prefix="${name}/" "${revision}" |
  gzip -n -9 >"${output}/${name}.tar.gz"
