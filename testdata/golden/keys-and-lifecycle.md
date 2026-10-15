<!-- tfimpact -->
## tfimpact: 1 change destroys data, 1 replaced, 1 minor, 1 routine

**Plan:** 2 to add, 0 to change, 2 to destroy (1 replacement, 1 forgotten, 2 unchanged) · Terraform/OpenTofu 1.15.5 · 6 resources

### 🔴 Critical (1)

- **&#96;aws\_sqs\_queue.work\[&#34;orders&#34;\]&#96; and &#96;aws\_sqs\_queue.work\[&#34;order\_events&#34;\]&#96; share an identity across a destroy/create pair: possible address rename.** &#96;aws\_sqs\_queue.work\[&#34;orders&#34;\]&#96; will be destroyed because its for\_each key changed, and &#96;aws\_sqs\_queue.work\[&#34;order\_events&#34;\]&#96; will be created with the same &#96;name&#96; \(&#34;fixture-lifecycle-orders&#34;\). A shared identity suggests a possible address rename; it does not establish that these are the same cloud object. The SQS queue would be destroyed with it, losing every message still in the queue. _Confirm that both entries refer to the same provider configuration, account, region and cloud object before considering a moved block \(&#96;from = aws\_sqs\_queue.work\[&#34;orders&#34;\]&#96;, &#96;to = aws\_sqs\_queue.work\[&#34;order\_events&#34;\]&#96;\). Otherwise review the intended migration and any uniqueness constraints on name._ `PLAN-RENAME-001`

### 🟡 Medium (1)

- **&#96;aws\_iam\_role.deploy&#96; will be destroyed and recreated.** The IAM role is replaced because &#96;name&#96; \(&#34;fixture-lifecycle-deploy-v1&#34; → &#34;fixture-lifecycle-deploy-v2&#34;\) cannot be changed in place. The new object is created before the old one is destroyed \(create\_before\_destroy\). _Check that the identifiers other systems hold for this resource \(ARN, ID, DNS name, IP\) are not referenced outside Terraform, or plan a cutover._ `PLAN-REPLACE-001`

### ⚪ Low (1)

- **&#96;aws\_sqs\_queue.work\[&#34;order\_events&#34;\]&#96;: &#96;policy&#96; is not known until apply, so it was not assessed.** The SQS queue&#39;s &#96;policy&#96; comes from a value the plan cannot show \(a data source, a template or an attribute of another resource\). _Review the policy document in the pull request, or build it with &#96;data.aws\_iam\_policy\_document&#96; in the same configuration so the plan carries it._ `PLAN-IAM-002`

### Routine changes (1)

- `aws\_cloudwatch\_log\_group.legacy`: removed from state but not destroyed

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
