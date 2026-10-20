# Changelog

All notable changes to tfimpact. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project uses [Semantic Versioning](https://semver.org/). Rule IDs never change
meaning; a rule that needs different semantics gets a new ID.

## [0.2.1] - 2026-10-20

### Fixed
- The Action's `verify-provenance: 'true'` verifies. It passed `--cert-identity` and
  `--signer-workflow` together, which `gh` refuses, so the step failed for everyone who
  turned it on. (#25)
- The SBOMs are CycloneDX, as the release notes have always described them. The `sboms`
  block named no arguments, so goreleaser used its default syft invocation and v0.2.0's
  documents are SPDX-2.3. (#24)
- The `cosign verify-blob` command in the release configuration verifies the signature
  it describes: the certificate identity is matched case-insensitively, because GitHub
  writes the organisation into the SAN as `Infraedgecloud-tools`. (#24)
- The README describes the release that exists. Its install section was written against
  v0.1.0 and said the packages, SBOMs, signed checksums and attestations v0.2.0 publishes
  were not established. (#26)

### Changed
- The Action installs `0.2.0` by default, not `0.1.0`, and verifies provenance by
  default. Pin `version: '0.1.0'` to keep the previous binary. (#25)
- Built with Go 1.27. The module was held at 1.26 while staticcheck could not read Go
  1.27 export data; staticcheck 2026.2.2 can. (#23)
- `goreleaser/goreleaser-action` 7.2.3 → 7.3.0. (#16)

## [0.2.0] - 2026-10-15

### Added
- `tfimpact explain tfplan` reads the saved plan `terraform plan -out` wrote, running
  `terraform show -json` (or `tofu`) in the current directory. (#6)
- `--format sarif` and `--sarif FILE`: SARIF 2.1.0 for code scanning. (#8)
- `docs/findings.schema.json` describes the `--format json` envelope, with a test that
  keeps the two in step. (#8)
- Colour in the text report on a terminal; `NO_COLOR`, `TERM=dumb` and `--color` respected. (#8)
- `tfimpact completion bash|zsh|fish`. (#8)
- `PLAN-IAM-002`: a policy document not known until apply is reported as not assessed
  rather than skipped. (#7)
- Knowledge files: `no_object`, `replace_routine`, `data_severity`, `starts_with`
  conditions, and `only` conditions on flags. (#5, #7)
- Release workflow configuration for Homebrew, Scoop, deb/rpm/apk, CycloneDX SBOMs
  and signed checksums. Availability must be verified per release; these artifacts
  are not established for v0.1.0.
- Action inputs `working-directory` and `config`; output `sarif`.

### Changed
- An ingress rule covering several administrative ports names all of them, in port
  order, on every run; the finding ID is stable. (#1)
- A replacement that changes nothing outside Terraform (a certificate validation wait, an
  underscore-named DNS record) is listed as routine. The headline counts a medium
  replacement as "replaced", not "interrupts service". (#5)
- Opening only tcp 80 or 443 to the internet is medium; administrative and database
  ports stay critical. Losing a CloudWatch log group is medium. (#5)
- IAM diff: action names keep their case; `Resource = "*"` is judged per action; the same
  action on a broader resource is a widening; escalation primitives such as
  `iam:PassRole` are medium with an explanation; the missing-`sub` rule applies to
  web-identity trusts only. (#7)
- A destroyed or replaced resource of a type with no knowledge file says so. (#7)
- On a new resource a flag's title states the fact rather than a transition. (#7)
- `go install` builds report the module version rather than `dev`. (#6)

### Fixed
- Suppressions match `for_each` and `count` addresses; `*` is the only wildcard. (#2)
- The exposure, policy and ingress checks run on replacements, not only on updates and
  creates. (#3)
- The README's release install block named archives without their version and
  downloaded a 404 page. (#6)
- An attribute that becomes unknown after apply no longer prints the value it held
  before, and a sensitive container now hides every attribute under it. Report this
  class of defect privately: see SECURITY.md.
- A plan Terraform recorded as failed, incomplete or deferred is rejected instead of
  being read as a safe no-op. A genuine no-op with `applyable=false` is still valid.
- Resources, providers, actions and modes the catalogue does not cover are reported as
  unassessed rather than counted as safe.
- Report paths can no longer overwrite the plan they were generated from, or collide
  with each other; each file is replaced atomically.
- Human-readable output escapes markup and control characters that came from the plan.

### Security
- `--format json` runs no subprocess at all. Saved-plan execution has a time and output
  budget, and its diagnostics are withheld rather than echoed.
- GitHub Action: binaries are installed at exact versions into isolated directories,
  the checksum selects exactly one archive, extraction streams only the named binary,
  and an optional comment can no longer replace the scan's exit status.

## [0.1.0] - 2026-10-14

First release: the plan reader, nine rules, 63 AWS knowledge files, Markdown, text and
JSON output, suppressions, the GitHub Action, and GitLab/Azure DevOps reference examples.
Release assets are six OS/architecture archives and `checksums.txt`, each with a
build-provenance attestation. Signed checksums, SBOMs and Linux packages start at
v0.2.0.

[Unreleased]: https://github.com/infraedgecloud-tools/tfimpact/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/infraedgecloud-tools/tfimpact/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/infraedgecloud-tools/tfimpact/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/infraedgecloud-tools/tfimpact/releases/tag/v0.1.0
