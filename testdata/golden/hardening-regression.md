<!-- tfimpact -->
## tfimpact: 2 changes destroy data, 7 changes widen exposure, 1 routine

**Plan:** 0 to add, 6 to change, 2 to destroy (5 unchanged) · Terraform/OpenTofu 1.15.5 · 13 resources

### 🔴 Critical (4)

- **`aws_db_instance.app`: the database becomes reachable from the internet (`publicly_accessible` = true).** `publicly_accessible` changes false → true. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`
- **`aws_iam_role.lambda`: the trust policy is widened.** `assume_role_policy` now allows any principal (`Principal = "*"`) with no restricting condition. _Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change._ `PLAN-IAM-001`
- **`aws_kms_key.data` will be destroyed, losing the key material.** The KMS key is destroyed because it is no longer in the configuration. Once the key is deleted, everything encrypted with it (volumes, snapshots, objects, secrets) can never be decrypted again. The deletion window is the minimum 7 days. _If removing it is intended, take a backup or snapshot first and keep it until the change has settled. If it was renamed or moved in code, add a `moved` block instead of recreating it._ `PLAN-DATA-001`
- **`aws_vpc_security_group_ingress_rule.postgres` opens PostgreSQL to the internet.** The security group ingress rule gains ingress for tcp port 5432 from 0.0.0.0/0. _Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change._ `PLAN-NET-001`

### 🟠 High (4)

- **`aws_cloudtrail.main`: logging is being turned off.** `enable_logging` changes true → false. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`
- **`aws_s3_bucket_public_access_block.data`: public bucket policies are no longer blocked.** `block_public_policy` changes true → false. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`
- **`aws_s3_bucket_public_access_block.data`: cross-account access to a public bucket is no longer restricted.** `restrict_public_buckets` changes true → false. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`
- **`aws_secretsmanager_secret.db` will be destroyed, losing the secret value and all its versions.** The Secrets Manager secret is destroyed because it is no longer in the configuration. A recovery window applies (`recovery_window_in_days`), so the secret can be restored in that time. _If removing it is intended, take a backup or snapshot first and keep it until the change has settled. If it was renamed or moved in code, add a `moved` block instead of recreating it._ `PLAN-DATA-001`

### 🟡 Medium (1)

- **`aws_db_instance.app`: deletion protection is being turned off, which removes the last guard against an accidental destroy.** `deletion_protection` changes true → false. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`

### Routine changes (1)

- `aws_s3_bucket.data`: tags only

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
