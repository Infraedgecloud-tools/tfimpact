# Changelog

All notable changes to tfimpact. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project uses [Semantic Versioning](https://semver.org/). Rule IDs never change
meaning; a rule that needs different semantics gets a new ID.

## [Unreleased]

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

## [0.1.0] - 2026-10-14

First release: the plan reader, nine rules, 63 AWS knowledge files, Markdown, text and
JSON output, suppressions, the GitHub Action, and GitLab/Azure DevOps reference examples.
Observed release assets are six OS/architecture archives and `checksums.txt`.
Container availability and build-provenance attestations are not established by
those assets; see the version-specific availability matrix in the README.

[Unreleased]: https://github.com/infraedgecloud-tools/tfimpact/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/infraedgecloud-tools/tfimpact/releases/tag/v0.1.0
