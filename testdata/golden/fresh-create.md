<!-- tfimpact -->
## tfimpact: 3 changes widen exposure, 1 minor, 4 routine

**Plan:** 8 to add, 0 to change, 0 to destroy · Terraform/OpenTofu 1.15.5 · 8 resources

### 🔴 Critical (1)

- **`aws_security_group.web` opens RDP to the internet.** The security group is created with ingress for tcp port 3389 from 0.0.0.0/0; tcp port 443 from 0.0.0.0/0. _Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change._ `PLAN-NET-001`

### 🟠 High (1)

- **`aws_iam_role_policy.app`: the policy is created with broad access.** `policy` grants `*` on all resources. _Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change._ `PLAN-IAM-001`

### 🟡 Medium (1)

- **`aws_subnet.public` is created with `map_public_ip_on_launch` = true.** `map_public_ip_on_launch` is set to true on creation; on an existing subnet this is what the flag means: instances launched in this subnet get public IP addresses by default. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`

### ⚪ Low (1)

- **`aws_sqs_queue.jobs`: `policy` is not known until apply, so it was not assessed.** The SQS queue's `policy` comes from a value the plan cannot show (a data source, a template or an attribute of another resource). _Review the policy document in the pull request, or build it with `data.aws_iam_policy_document` in the same configuration so the plan carries it._ `PLAN-IAM-002`

### Routine changes (4)

- `aws_cloudwatch_log_group.app`: created
- `aws_iam_role.app`: created
- `aws_s3_bucket.assets`: created
- `aws_vpc.main`: created

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
