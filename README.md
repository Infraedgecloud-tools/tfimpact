<p align="center"><img src="docs/assets/wordmark.svg" width="420" alt="tfimpact"></p>

<p align="center">
  <a href="https://github.com/infraedgecloud-tools/tfimpact/releases/latest"><img src="https://img.shields.io/github/v/release/infraedgecloud-tools/tfimpact?display_name=tag&color=0B2545" alt="Release"></a>
  <a href="https://github.com/infraedgecloud-tools/tfimpact/actions/workflows/ci.yml"><img src="https://github.com/infraedgecloud-tools/tfimpact/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/infraedgecloud-tools/tfimpact"><img src="https://goreportcard.com/badge/github.com/infraedgecloud-tools/tfimpact" alt="Go Report Card"></a>
  <a href="https://pkg.go.dev/github.com/infraedgecloud-tools/tfimpact"><img src="https://pkg.go.dev/badge/github.com/infraedgecloud-tools/tfimpact.svg" alt="Go Reference"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/licence-Apache--2.0-1A63B0" alt="Apache-2.0"></a>
</p>

**tfimpact explains what a Terraform or OpenTofu plan will do to your AWS resources.**
Not how many resources change: which change destroys data, which one restarts
something, which rename will quietly recreate a bucket, and which policy just got wider.

It reads the plan Terraform already wrote, runs on your CI runner or your laptop, and
sends nothing anywhere. Deterministic. No account, no cloud access, no LLM.

<p align="center"><img src="docs/assets/demo.svg" width="960" alt="tfimpact explain on the bundled example plan: one change destroys data, one widens exposure, one interrupts service, thirty-eight routine"></p>

<details>
<summary>The same report as text</summary>

```text
tfimpact: 1 change destroys data, 1 change widens exposure, 1 change interrupts service, 38 routine
Plan: 1 to add, 40 to change, 1 to destroy (1 replacement, 6 unchanged).

CRITICAL (2)
  * aws_dynamodb_table.sessions will be destroyed and recreated, losing every item in the table [PLAN-DATA-002]
    The DynamoDB table is replaced because hash_key ("session_id" → "user_id") cannot be changed in place.
    Deletion protection is off; point-in-time recovery is off, so there is no automatic backup to restore from.
  * aws_security_group.bastion opens SSH to the internet [PLAN-NET-001]
    The security group gains ingress for tcp port 22 from 0.0.0.0/0.

HIGH (1)
  * aws_instance.bastion: changing user_data in place has a service effect [PLAN-DISRUPT-001]
    The instance is stopped and started to apply the new user data ...

ROUTINE (38)
  * 10 x aws_cloudwatch_log_group: retention_in_days 30 → 90
  * 24 x aws_ssm_parameter: tags only
```

</details>


That plan says `1 to add, 40 to change, 1 to destroy`. Every existing summariser we
tested lists the 40 updates and puts the one replacement at the bottom. None of them
says that the table's items are gone, or that the "update in-place" on the bastion is a
stop/start.

## Try it in a minute

```bash
terraform plan -out=tfplan
terraform show -json tfplan > plan.json
tfimpact explain plan.json
```

**Unreleased main features (absent from v0.1.0):** saved-plan helper, SARIF output,
shell completion, and the associated current-source flags shown below.

Given the saved plan, current unreleased source runs `terraform show -json` on it for you (or `tofu`), in
the directory you ran `plan` in. In CI, where the JSON is usually written anyway, hand it
the JSON or pipe it:

```bash
terraform show -json tfplan | tfimpact explain -
```

## Install

Availability checked on 15 October 2026 against the published **v0.1.0** assets.
Workflow configuration describes future packaging; it does not prove publication.
The published version is legacy: the hardening described in this change requires
an owner-approved release. Installing v0.1.0 does not install these pending changes.

| Channel | v0.1.0 availability | Verification in this change |
|---|---|---|
| Release archives | Linux/macOS amd64 and arm64 `.tar.gz`; Windows amd64 and arm64 `.zip`; `checksums.txt` | Linux amd64 downloaded, checksum checked and CLI smoke tested; other platforms pending native runners |
| Go source install | Versioned module path below | See [validation evidence](docs/validation.md) |
| GitHub Action | Source available; pin source SHA and binary version separately | Embedded Bash consumer tests; live hosted runner and fork permissions pending |
| Homebrew / Scoop | Upcoming; tap and bucket were empty when checked | No supported installation command yet |
| deb / rpm / apk | Not attached to v0.1.0 | Upcoming |
| Container | Not established by the archive release | Registry pull/run verification pending |
| SBOM / signed checksums / attestations | Not established for v0.1.0 | No provenance claim for that release |

```bash
# Explicit Linux amd64 release archive. Inspect the release before selecting another platform.
v=0.1.0
archive="tfimpact_${v}_linux_amd64.tar.gz"
base="https://github.com/Infraedgecloud-tools/tfimpact/releases/download/v${v}"
curl -fsSLO "$base/$archive"
curl -fsSLO "$base/checksums.txt"
test -f "$archive"
awk -v archive="$archive" '$2 == archive {print}' checksums.txt > selected-checksum.txt
test "$(wc -l < selected-checksum.txt)" -eq 1
sha256sum -c selected-checksum.txt
# Stream only the named executable rather than extracting archive-controlled paths.
tar -xOzf "$archive" tfimpact > tfimpact
chmod +x tfimpact
./tfimpact version

# Alternatively, a pinned source install with the Go toolchain required by the module:
go install github.com/infraedgecloud-tools/tfimpact/cmd/tfimpact@v0.1.0
```

Release checksums detect corruption; a checksum downloaded beside a binary does not
establish build provenance. Do not assume the signatures or attestations configured
in the current release workflow exist on earlier releases.

Try it on the bundled fixtures without any AWS account:

```bash
tfimpact explain testdata/plans/noisy/plan.json
tfimpact explain testdata/plans/rename/plan.json --format markdown
```

## In a pull request

```yaml
permissions:
  contents: read

steps:
  - uses: actions/checkout@<sha>
  - uses: hashicorp/setup-terraform@<sha>
  - run: terraform init && terraform plan -out=tfplan && terraform show -json tfplan > plan.json
  - uses: infraedgecloud-tools/tfimpact@<sha>   # pin to a release tag or commit
    with:
      plan: plan.json
      version: '0.1.0'      # binary release; pin independently of the Action source
      comment: 'false'      # works with a read-only/fork PR token
      fail-on: high        # exit 1 when an unsuppressed finding is high or critical
```

The action writes the report to the job summary. Comments are opt-in (`comment: 'true'`)
and need `pull-requests: write`; denied comment permissions do not replace the scan's
exit code. Use ordinary `pull_request` workflows and read-only tokens for fork scans.
Do not check out untrusted PR code under `pull_request_target` with privileged credentials.

The Action defaults to binary version `0.1.0`; moving version labels are rejected.
Pin both the Action source SHA and binary version. `verify-provenance: 'true'` requires
an attestation from this repository's release workflow and the exact selected tag;
it fails closed if verification is unavailable. The default is false for compatibility
with the legacy v0.1.0 release, whose provenance is not established. Enable verification
only after the selected release has been verified by the maintainer.
GitLab CI and Azure DevOps snippets in [`docs/ci/`](docs/ci/) are reference examples;
they have not been run on those providers in this change.

## What it looks for

| Rule | What it says |
|---|---|
| `PLAN-DATA-001` / `PLAN-DATA-002` | A resource that holds data is destroyed or replaced. Names what is lost and whether a final snapshot, deletion protection, point-in-time recovery or a recovery window softens it. |
| `PLAN-RENAME-001` | A resource was renamed in code without a `moved` block: the plan destroys one address and creates another with the same name. |
| `PLAN-REPLACE-001` | Anything else replaced, with the attribute that forced it and the service effect (new DNS name, new IP, dependent resources). A replacement that changes nothing outside Terraform, such as a certificate validation wait or an underscore-named DNS record, is listed as routine instead. |
| `PLAN-DISRUPT-001` | An in-place update that still interrupts service: an instance class change, a `user_data` change that stops and starts the instance, an RDS identifier rename that changes the endpoint. |
| `PLAN-IAM-001` | A policy, trust policy or resource policy got wider: wildcard actions, `Resource = "*"`, `Principal = "*"`, a removed or loosened condition, the same action on a broader resource, an escalation primitive such as `iam:PassRole`, a web-identity trust with no `sub`. |
| `PLAN-IAM-002` | A policy document that is not known until apply, so it was not assessed. |
| `PLAN-NET-001` | Ingress opened to `0.0.0.0/0` or `::/0`, with the port and service named. |
| `PLAN-EXPOSE-001` | A protection turned off or an exposure flag turned on: public access block, encryption, deletion protection, logging, `publicly_accessible`. |
| `PLAN-DELETE-001` | Other deletions, raised when the resource is protective (a trail, a flow log, a public access block). |

Supported changes without a finding are listed as routine in one line per group, so forty tag changes take one
line and the dangerous change does not scroll off the screen.

Resource knowledge lives in [`internal/knowledge/aws/`](internal/knowledge/aws/), one
TOML file per resource type. Adding a resource is a data change plus a fixture, not Go.
`tfimpact knowledge` lists the types it knows, `tfimpact knowledge aws_db_instance` shows
one, and `tfimpact rules` prints the rule catalogue ([docs/rules.md](docs/rules.md)).

## Sensitive values

`terraform show -json` writes sensitive values in plain text. Redaction behaviour
and the available flags depend on the selected binary version. Keep plan files and
generated reports inside trusted CI storage, and review reports before sharing.
Attribute names, resource identities and file paths can also carry confidential data.
Do not attach unreviewed plans or reports to public issues.

## Suppressions

Put a `.tfimpact.toml` next to where you run it. A suppression needs a reason and may
expire. Suppressed findings stay in the report under their own heading and do not
count towards `--fail-on`.

```toml
version = 1

[[suppress]]
rule = "PLAN-DATA-002"
address = "aws_dynamodb_table.sessions"
reason = "sessions are rebuilt on login; approved in CHG-123"
expires = "2026-12-31"
```

`address` is the resource address as the report prints it. `*` matches any run of
characters and is the only wildcard; everything else is literal, so
`module.app.*`, `aws_s3_bucket.logs["eu"]` and `aws_db_instance.*[0]` all mean what
they look like. Leaving `address` out suppresses the rule everywhere.

## Output

- `--format text` for terminals (the default when stdout is a terminal). Colour follows
  the terminal: on when stdout is a TTY, off under `NO_COLOR` or `TERM=dumb`, and
  `--color always|never` overrides.
- `--format markdown` for pull-request comments (the default in CI)
- `--format json` for machines: the findings schema 1.1 envelope, described by
  [`docs/findings.schema.json`](docs/findings.schema.json)
- `--format sarif` for code-scanning tabs and security dashboards: SARIF 2.1.0, one
  result per finding with the resource address as its logical location and the
  finding's stable ID as its fingerprint
- `--output FILE`, `--json FILE` and `--sarif FILE` write files alongside whatever
  goes to stdout
- Exit code 0, or 1 when `--fail-on LEVEL` is reached, or 2 on bad input,
  explicit `errored: true`, or explicit `complete: false`. `applyable: false` alone
  does not reject a valid empty no-op plan. Older plans without these optional fields
  remain supported. Unknown minor fields are ignored; unsupported major formats fail.
- Unrecognised resource types, providers and action combinations emit
  `PLAN-COVERAGE-001` as unassessed. `--fail-on low` gates these findings; broader
  thresholds do not imply full assessment. Generic plan action facts can still be shown.
- JSON input is limited to 64 MiB. `--json-only` rejects binary saved plans before
  any subprocess. JSON files and stdin are parsed locally without Terraform/OpenTofu.
  These hardening changes are pending release and are absent from v0.1.0.

Upload the SARIF to GitHub code scanning from the same job:

```yaml
  - run: tfimpact explain plan.json --sarif tfimpact.sarif --quiet
  - uses: github/codeql-action/upload-sarif@<sha>
    with:
      sarif_file: tfimpact.sarif
```

Shell completion: `source <(tfimpact completion bash)`, `source <(tfimpact completion zsh)`
or `tfimpact completion fish | source`.

### The comment subcommand

The GitHub Action calls `tfimpact comment --body FILE` after `explain` to create or
update one comment on the pull request. It reads `GITHUB_TOKEN`, `GITHUB_REPOSITORY` and
the event payload from the runner's environment, finds its earlier comment by a marker
in the body, and edits that rather than posting another. Outside Actions it has nothing
to talk to and says so.

## What it does not do

- It is not a scanner of your `.tf` files. It reads the plan, so it only sees what is
  about to change. Pair it with tflint, Checkov or Trivy for static policy.
- It knows AWS. Other providers are listed as plain changes.
- It does not estimate cost, and it does not call AWS to look at what is deployed.
- It needs the saved plan (`plan -out`), as the plan file or as `show -json` output. The streaming
  `terraform plan -json` output has no attribute detail.
- It has no AI mode. The explanations are templates filled from the plan, so the same
  plan always gives the same words.

Tested with Terraform 1.15 and OpenTofu 1.13 plan JSON (format 1.2). The format is
covered by Terraform's 1.x compatibility promise; unknown fields are ignored.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). The fastest useful contribution is a knowledge
file for a resource type you know well, with a plan fixture that exercises it.
Fixtures are generated with real Terraform against a local [moto](https://github.com/getmoto/moto)
mock (`testdata/plans/generate.sh`); never hand-edit a plan.json.

## Licence

Apache-2.0. tfimpact is made by [InfraEdge](https://infraedgecloud.com).

## Assessment limits

Downtime and rename effects are resource heuristics; action lists and replacement paths
are plan facts. Equal names across delete/create entries suggest a rename and do not prove
operator intent. IAM comparison does not compute effective permissions across policies,
SCPs, boundaries or identity sessions. This is not dependency blast-radius analysis.

Saved binary plans explicitly invoke `terraform show -json` or `tofu show -json` from PATH
(or the executable selected by `TFIMPACT_TERRAFORM`) with a 60-second deadline. This can
load provider plugins from the initialised configuration. Only use it with trusted
executables and provider installations. The helper never runs init, plan or apply.
Use pre-generated show-JSON with `--json-only` for strict offline scanning; do not set
`TFIMPACT_TERRAFORM` from untrusted repository content. Subprocess diagnostics are withheld
from tfimpact errors because they may contain sensitive material.
