# Candidate validation — 15 October 2026

Base: `e1db785d95eb9cfbbb4d2aa3a477d04f8c440cf2` in
`Infraedgecloud-tools/tfimpact`. Downloaded source bytes were checked against every
Git blob SHA before modification. The untouched complete checkout passed
`go test ./...` with Go 1.26.9 on Linux amd64. Missing-fixture failures during
initial source transfer were resolved by completing and verifying that transfer;
they were not repository failures.

## Executed checks

Toolchain: official Go 1.26.9 linux/amd64. Environment: Linux 6.18.44,
Intel Xeon Platinum 8573C; no AWS account or cloud mutation.

One local benchmark per size used synthetic known S3 creates, compact JSON and
`explain plan.json --json-only --quiet`; elapsed wall time came from Python's
monotonic clock and peak child RSS from `os.wait4` after `exec` of the candidate.
These are measurements of this fixture and host, not service-level guarantees.

| Resources | Input bytes | Elapsed | Peak RSS KiB |
|---|---:|---:|---:|
| 10 | 1,455 | 0.009 s | 8,144 |
| 10,000 | 1,467,825 | 0.198 s | 42,820 |
| 100,000 | 14,877,825 | 1.502 s | 366,460 |

The parser materialises plan objects; a 64 MiB input ceiling is not a 64 MiB
memory ceiling. Large plans need substantially more memory than their file size.

| Check | Result |
|---|---|
| `go test ./...` | Pass, including existing Terraform/OpenTofu fixture goldens |
| `go test -race ./...` | Pass |
| `go vet ./...` and `gofmt -l .` | Pass, formatting prints nothing |
| `go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...` | Pass after fixing two introduced diagnostics |
| `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` | No vulnerabilities found |
| `go test ./internal/sensitive -run '^$' -fuzz FuzzUnknownSensitiveValues -fuzztime=3s -parallel=2` | Pass; 162,678 generated executions in the recorded run |
| Action YAML parse and `bash -n` for each run block | Pass |
| Embedded Action Bash consumer harness | Pass; exact version, bad version/path/checksum/member, provenance success/denial, custom paths and scan exits 0/1/2 |
| Built RC archive installed outside checkout | Pass; help/version, JSON output, clean fixture, severity gate, strict JSON with Terraform absent |
| Published v0.1.0 Linux amd64 archive | Download/checksum/help/version/clean+failing fixture/JSON pass |
| `GOBIN=<fresh-dir> go install github.com/infraedgecloud-tools/tfimpact/cmd/tfimpact@v0.1.0` | Install and CLI smoke pass; this historical source install reports `tfimpact dev` |

The Action harness executes the actual embedded Bash. Download and attestation
commands are local test doubles; this validates policy arguments and failure
propagation, not a successful cryptographic attestation of a published artifact.
The v0.1.0 binary does not support SARIF. Its absence is represented by an empty
Action output. The candidate's local source build does support SARIF.

## Supported boundaries and remaining gates

The parser accepts documented major format 1, additive minor fields and legacy
plans without optional completion fields. Explicit failed/incomplete/deferred
plans fail with exit 2, while `applyable: false` alone allows valid no-op plans.
Duplicate keys, invalid collection shapes, excessive depth and inputs over
64 MiB fail. Unsupported modes/providers/types/actions are visibly unassessed;
`--fail-on low` includes coverage findings.

Saved-plan execution is opt-in by supplying a binary saved-plan path. Strict
`--json-only` and Action stdin paths never invoke Terraform/OpenTofu. The saved-plan
helper has time and output budgets and withholds diagnostics. Output destinations
are checked against the input and each other; report replacement is atomic per file.

Resource addresses and names are user-controlled identifiers. Review reports before
sharing. IAM effects, rename intent, service impact and external dependencies are
bounded heuristics, not effective-permission or whole-account proofs. JSON-only
smoke tests ran without Terraform available; an OS-enforced network-denial run was
not available and is not claimed.

Native macOS/Windows/arm64, live GitHub hosted/fork-permission runs, GitLab/Azure
execution, container registry pulls, Homebrew/Scoop installation, package archives,
published provenance/signature/SBOM verification and Terraform fixture regeneration
remain pending. Existing provider fixtures retain their recorded versions; new
adversarial tests are labelled synthetic and do not claim provider-generated evidence.

## Owner release checklist

- Review the private security patch and coordinate disclosure before publication.
- Review and merge the separate Action/install PR and candidate correctness changes.
- Run native platform and hosted Action consumer gates, including read-only fork tokens.
- Create the approved release; inspect each produced archive/package and binary version.
- Verify expected workflow, repository and exact tag on real attestations, then enable
  provenance by default only for a release that actually supplies it.
- Populate Homebrew/Scoop only after testing their real formula/manifest installations.
- Update the availability matrix and website after successful release checks.

No merge, tag, package publication, deployment or cloud operation was performed.
