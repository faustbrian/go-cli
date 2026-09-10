#!/usr/bin/env bash
set -euo pipefail

repository="$(git rev-parse --show-toplevel)"
release_script="${RELEASE_SCRIPT:-${repository}/scripts/build-release.sh}"
workspace="$(mktemp -d)"
cleanup() {
  chmod -R u+w "${workspace}" 2>/dev/null || true
  rm -rf -- "${workspace}"
}
trap cleanup EXIT HUP INT TERM

fixture="${workspace}/repository"
mkdir -p "${fixture}/scripts"
cp "${release_script}" "${fixture}/scripts/build-release.sh"
chmod +x "${fixture}/scripts/build-release.sh"
printf 'tagged source\n' >"${fixture}/README.md"
printf 'module example.com/cli\n\ngo 1.27.0\n' >"${fixture}/go.mod"

git -C "${fixture}" init --quiet
git -C "${fixture}" config user.email release-test@example.com
git -C "${fixture}" config user.name 'Release Test'
git -C "${fixture}" add README.md go.mod scripts/build-release.sh
git -C "${fixture}" commit --quiet -m 'tagged source'
git -C "${fixture}" tag -a v1.2.3 -m v1.2.3

printf 'later head\n' >"${fixture}/README.md"
printf 'must not be archived\n' >"${fixture}/after-tag.txt"
git -C "${fixture}" add README.md after-tag.txt
git -C "${fixture}" commit --quiet -m 'later head'

(
  cd "${fixture}"
  ./scripts/build-release.sh v1.2.3 "${workspace}/artifacts"
)
mkdir "${workspace}/archive"
tar -xzf "${workspace}/artifacts/cli-v1.2.3.tar.gz" -C "${workspace}/archive"

cmp -s "${workspace}/archive/cli-v1.2.3/README.md" <(printf 'tagged source\n')
test -f "${workspace}/archive/cli-v1.2.3/go.mod"
test ! -e "${workspace}/archive/cli-v1.2.3/after-tag.txt"
