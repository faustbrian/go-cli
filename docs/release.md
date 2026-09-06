# Release policy

Releases use root repository tags named `vX.Y.Z`. Maintainers update the
unreleased changelog, run the complete repository checks from the repository
root, review API and dependency changes, and create a signed annotated tag.
The current CI workflow validates branch changes and provides an explicit
release dry run; publishing the GitHub release is a maintainer-operated step.

`scripts/build-release.sh` produces a deterministic source archive from the
exact supplied root tag. Published releases include a CycloneDX SBOM,
SHA-256 checksums, a detached checksum signature, and build provenance. No
application binary is published because this repository is a library; the
reference generator is a development tool, not a consumer executable.

Consumers must use the verification instructions shipped with each release to
verify its provenance and checksum signature, then compare the checksums file.
Patch releases preserve documented behavior, minor releases add compatible
API, and major releases may intentionally break public contracts after
migration guidance and changelog notice. Security fixes follow `SECURITY.md`.

To reproduce a source archive locally from the repository root:

```sh
./scripts/build-release.sh v1.2.3 "$(mktemp -d)/artifacts"
```
