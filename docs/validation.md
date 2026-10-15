# Action and installation validation — 15 October 2026

Base: e1db785d95eb9cfbbb4d2aa3a477d04f8c440cf2. Go 1.26.9, Linux amd64.
The complete source transfer was verified against every Git blob SHA before editing.
The untouched baseline passed `go test ./...`.

Executed: `go test ./...`, `go test -race ./...`, `go vet ./...`,
`gofmt -l .` and pinned staticcheck v0.8.2. The Action tests execute the actual
embedded Bash with download/attestation doubles, checking exact version selection,
checksum failure, archive-member safety, workflow/repository/tag verification
constraints, custom paths, and scan exit codes 0/1/2. YAML parse and every run
block's `bash -n` also pass. Comment failure policy is inspected; a real fork-token
posting attempt is pending.

Published v0.1.0 assets were inspected: six OS/architecture archives and checksums.
Linux amd64 downloaded and checksum verified; help/version, a clean fixture, a
threshold-failing fixture and machine-readable JSON passed outside the source tree.
A fresh `go install .../cmd/tfimpact@v0.1.0` also passed CLI smoke; that historical
source version reports `tfimpact dev`. No signature, provenance, SBOM, native package,
tap/bucket or container availability is inferred from workflow configuration.

Remaining: native macOS/Windows/arm64, hosted Action/fork permissions, real provenance,
GitLab/Azure execution, container pull/run and tap/bucket installation. The Action
uses checksums by default for compatibility with legacy v0.1.0; this does not prove
provenance. Explicit `verify-provenance: true` fails closed and must be enabled for
a release whose exact workflow/ref attestations have actually been verified.

No merge, release tag, package publication, deployment or cloud operation occurred.
