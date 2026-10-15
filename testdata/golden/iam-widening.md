<!-- tfimpact -->
## tfimpact: 3 changes widen exposure

**Plan:** 0 to add, 3 to change, 0 to destroy (3 unchanged) · Terraform/OpenTofu 1.15.5 · 6 resources

### 🔴 Critical (1)

- **&#96;aws\_security\_group.admin&#96; opens SSH to the internet.** The security group gains ingress for tcp port 22 from 0.0.0.0/0. _Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change._ `PLAN-NET-001`

### 🟠 High (2)

- **&#96;aws\_iam\_policy.deploy&#96;: the policy is widened.** &#96;policy&#96; now grants &#96;s3:\*&#96; on all resources. _Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change._ `PLAN-IAM-001`
- **&#96;aws\_iam\_role.deploy&#96;: the trust policy is widened.** &#96;assume\_role\_policy&#96; now widens the &#96;token.actions.githubusercontent.com:sub&#96; condition from &#96;repo:acme/platform:ref:refs/heads/main&#96; to &#96;repo:acme/\*&#96;. _Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change._ `PLAN-IAM-001`

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
