<!-- tfimpact -->
## tfimpact: 3 changes widen exposure, 1 minor, 4 routine

**Plan:** 8 to add, 0 to change, 0 to destroy · Terraform/OpenTofu 1.15.5 · 8 resources

### 🔴 Critical (1)

- **&#96;aws\_security\_group.web&#96; opens RDP to the internet.** The security group is created with ingress for tcp port 3389 from 0.0.0.0/0; tcp port 443 from 0.0.0.0/0. _Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change._ `PLAN-NET-001`

### 🟠 High (1)

- **&#96;aws\_iam\_role\_policy.app&#96;: the policy is created with broad access.** &#96;policy&#96; grants &#96;\*&#96; on all resources. _Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change._ `PLAN-IAM-001`

### 🟡 Medium (1)

- **&#96;aws\_subnet.public&#96; is created with &#96;map\_public\_ip\_on\_launch&#96; = true.** &#96;map\_public\_ip\_on\_launch&#96; is set to true on creation; on an existing subnet this is what the flag means: instances launched in this subnet get public IP addresses by default. _Confirm this is intended and recorded in the change; if not, revert it._ `PLAN-EXPOSE-001`

### ⚪ Low (1)

- **&#96;aws\_sqs\_queue.jobs&#96;: &#96;policy&#96; is not known until apply, so it was not assessed.** The SQS queue&#39;s &#96;policy&#96; comes from a value the plan cannot show \(a data source, a template or an attribute of another resource\). _Review the policy document in the pull request, or build it with &#96;data.aws\_iam\_policy\_document&#96; in the same configuration so the plan carries it._ `PLAN-IAM-002`

### Routine changes (4)

- `aws\_cloudwatch\_log\_group.app`: created
- `aws\_iam\_role.app`: created
- `aws\_s3\_bucket.assets`: created
- `aws\_vpc.main`: created

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
