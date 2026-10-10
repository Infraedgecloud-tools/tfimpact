# Plan fixtures

Ten plan JSON files produced by real Terraform runs against a local
[moto](https://github.com/getmoto/moto) mock of AWS. Each directory holds `before.tf`
(applied first), `after.tf` (planned against that state), `plan.txt` (Terraform's own
output), `plan.json` (`terraform show -json`) and `versions.txt`. Never hand-edit a
`plan.json`; change the `.tf` files and run `./generate.sh <name>`.

| Fixture | Exercises |
|---|---|
| `rds-replacement` | `PLAN-DATA-002`: RDS replaced by `storage_encrypted`, no final snapshot, deletion protection off; identifier renamed and class changed in the same plan |
| `rename` | `PLAN-RENAME-001`: bucket renamed without `moved`; a queue renamed with `moved` (move, not a finding); a dependent versioning resource |
| `iam-widening` | `PLAN-IAM-001` on an identity policy (`s3:*` on `*`) and a trust policy (`sub` widened to `repo:acme/*`); `PLAN-NET-001` SSH to the world |
| `tag-only` | nothing: five tag-only updates and a `default_tags` change are routine |
| `noisy` | a buried DynamoDB replacement and SSH widening above 40 routine updates; `user_data` stop/start (`PLAN-DISRUPT-001`); 6 no-ops |
| `hardening-regression` | `PLAN-EXPOSE-001` on a public access block, CloudTrail logging, `publicly_accessible`, deletion protection; `PLAN-NET-001` PostgreSQL via `aws_vpc_security_group_ingress_rule`; `PLAN-IAM-001` trust to `Principal = "*"`; `PLAN-DATA-001` on a KMS key and a secret with recovery windows |
| `fresh-create` | creates only: RDP open on creation, a `*` on `*` inline policy, a subnet with public IPs by default; everything else routine |
| `destroy-all` | ten deletes: data loss with safeguards (final snapshot, deletion protection, point-in-time recovery, `force_destroy`), `skip_destroy` as a state removal, protective and service-interrupting deletes |
| `keys-and-lifecycle` | a `for_each` key rename detected as `PLAN-RENAME-001`; a `create_before_destroy` replacement; a `removed` block (forget) |
| `drift` | an object changed and another deleted outside Terraform between apply and plan (`between.sh`, planned with refresh) |

`rename/plan.opentofu-1.13.1.json` is the same plan produced by OpenTofu; a test asserts
the findings are identical.

## Regenerating

```bash
./generate.sh            # all ten, about ten minutes
./generate.sh drift      # one
```

Requires `terraform` (>= 1.5), `docker`, `curl` and, for `drift`, the `aws` CLI. The
script starts moto on port 5000, points the provider at it through `AWS_ENDPOINT_URL`
with dummy keys, and resets moto between fixtures. Nothing contacts a real AWS account.
The fixtures contain the string `fixture-password-not-real` on purpose: tests assert it
never reaches any output.
