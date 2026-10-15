<!-- tfimpact -->
## tfimpact: 2 changes destroy data, 7 changes widen exposure, 1 routine

**Plan:** 0 to add, 6 to change, 2 to destroy (5 unchanged) · Terraform/OpenTofu 1.15.5 · 13 resources

### 🔴 Critical (4)

- **&#96;aws\_db\_instance.app&#96;: the database becomes reachable from the internet \(&#96;publicly\_accessible&#96; = true\).** &#96;publicly\_accessible&#96; changes false → true. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`
- **&#96;aws\_iam\_role.lambda&#96;: the trust policy is widened.** &#96;assume\_role\_policy&#96; now allows any principal \(&#96;Principal = &#34;\*&#34;&#96;\) with no restricting condition. _Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change._ `PLAN-IAM-001`
- **&#96;aws\_kms\_key.data&#96; will be destroyed, losing the key material.** The KMS key is destroyed because it is no longer in the configuration. Once the key is deleted, everything encrypted with it \(volumes, snapshots, objects, secrets\) can never be decrypted again. The deletion window is the minimum 7 days. _If removing it is intended, take a backup or snapshot first and keep it until the change has settled. If it was renamed or moved in code, add a &#96;moved&#96; block instead of recreating it._ `PLAN-DATA-001`
- **&#96;aws\_vpc\_security\_group\_ingress\_rule.postgres&#96; opens PostgreSQL to the internet.** The security group ingress rule gains ingress for tcp port 5432 from 0.0.0.0/0. _Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change._ `PLAN-NET-001`

### 🟠 High (4)

- **&#96;aws\_cloudtrail.main&#96;: logging is being turned off.** &#96;enable\_logging&#96; changes true → false. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`
- **&#96;aws\_s3\_bucket\_public\_access\_block.data&#96;: public bucket policies are no longer blocked.** &#96;block\_public\_policy&#96; changes true → false. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`
- **&#96;aws\_s3\_bucket\_public\_access\_block.data&#96;: cross-account access to a public bucket is no longer restricted.** &#96;restrict\_public\_buckets&#96; changes true → false. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`
- **&#96;aws\_secretsmanager\_secret.db&#96; will be destroyed, losing the secret value and all its versions.** The Secrets Manager secret is destroyed because it is no longer in the configuration. A recovery window applies \(&#96;recovery\_window\_in\_days&#96;\), so the secret can be restored in that time. _If removing it is intended, take a backup or snapshot first and keep it until the change has settled. If it was renamed or moved in code, add a &#96;moved&#96; block instead of recreating it._ `PLAN-DATA-001`

### 🟡 Medium (1)

- **&#96;aws\_db\_instance.app&#96;: deletion protection is being turned off, which removes the last guard against an accidental destroy.** &#96;deletion\_protection&#96; changes true → false. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`

### Routine changes (1)

- `aws\_s3\_bucket.data`: tags only

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
