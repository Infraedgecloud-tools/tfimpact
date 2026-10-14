<!-- tfimpact -->
## tfimpact: 3 changes widen exposure

**Plan:** 0 to add, 3 to change, 0 to destroy (3 unchanged) · Terraform/OpenTofu 1.15.5 · 6 resources

### 🔴 Critical (1)

- **`aws_security_group.admin` opens SSH to the internet.** The security group gains ingress for tcp port 22 from 0.0.0.0/0. _Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change._ `PLAN-NET-001`

### 🟠 High (2)

- **`aws_iam_policy.deploy`: the policy is widened.** `policy` now grants `s3:*` on all resources. _Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change._ `PLAN-IAM-001`
- **`aws_iam_role.deploy`: the trust policy is widened.** `assume_role_policy` now widens the `token.actions.githubusercontent.com:sub` condition from `repo:acme/platform:ref:refs/heads/main` to `repo:acme/*`. _Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change._ `PLAN-IAM-001`

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
