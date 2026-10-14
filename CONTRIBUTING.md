# Contributing to tfimpact

Small pull requests are easiest to review. Open an issue first for anything large.

## Sign off your commits (DCO)

Every commit needs a `Signed-off-by` line. Add it with:

```sh
git commit -s
```

The sign-off certifies the [Developer Certificate of Origin](https://developercertificate.org/): you wrote the change or have the right to submit it under the project's licence (Apache-2.0). Use your real name and an email address you control. A pull request with unsigned commits will not be merged.

## Commit messages

Use a conventional prefix (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`), imperative mood, under 72 characters.

## Checks to run

```sh
go test ./...
gofmt -l .          # must print nothing
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
```

CI runs the same checks, plus `go test -race` and `govulncheck`.

## Adding knowledge of an AWS resource

Resource behaviour lives in data, not code:

1. Add one TOML file at `internal/knowledge/aws/<resource_type>.toml`, for example `aws_db_instance.toml`.
2. Add a plan fixture under `testdata/plans/` that exercises the new rules.
3. Generate the fixture with `testdata/plans/generate.sh`, which applies real Terraform against moto in Docker. Never hand-edit a `plan.json`.
4. Never commit a plan containing real values. Fixtures come from moto only.

The moto fixtures are for provider behaviour: what Terraform really plans for a resource.
A knowledge-file change or a rule edge case does not need one. Write a small synthetic plan
in `internal/rules/rules_test.go` with the `rc(...)` helper, as the existing edge-case tests
do; it runs in milliseconds and needs no Docker. Add a moto fixture when the question is
what the provider does, not what tfimpact says about it.
5. Add or extend a test that asserts the findings for that fixture.

## False-positive policy

A rule that fires on a safe change is a bug. Open an issue with the false-positive template and attach the fixture, redacted. A fix comes with a regression fixture. A missed destructive change is more serious and is treated as a security issue (see SECURITY.md).

## Plan files contain secrets

`terraform show -json` prints sensitive values in plain text. Redact secrets and real identifiers before attaching a plan to an issue or pull request.

## Conduct

Participation is covered by the [Code of Conduct](CODE_OF_CONDUCT.md).
