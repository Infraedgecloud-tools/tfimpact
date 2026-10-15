<!-- tfimpact -->
## tfimpact: 1 change destroys data, 1 minor, 1 routine

**Plan:** 2 to add, 0 to change, 2 to destroy (1 move, 2 unchanged) · Terraform/OpenTofu 1.15.5 · 6 resources

### 🔴 Critical (1)

- **&#96;aws\_s3\_bucket.logs&#96; and &#96;aws\_s3\_bucket.audit\_logs&#96; share an identity across a destroy/create pair: possible address rename.** &#96;aws\_s3\_bucket.logs&#96; will be destroyed because it left the configuration, and &#96;aws\_s3\_bucket.audit\_logs&#96; will be created with the same &#96;bucket&#96; \(&#34;fixture-alb-access-logs-eu-west-2&#34;\). A shared identity suggests a possible address rename; it does not establish that these are the same cloud object. The S3 bucket would be destroyed with it, losing every object and object version in the bucket. _Confirm that both entries refer to the same provider configuration, account, region and cloud object before considering a moved block \(&#96;from = aws\_s3\_bucket.logs&#96;, &#96;to = aws\_s3\_bucket.audit\_logs&#96;\). Otherwise review the intended migration and any uniqueness constraints on bucket._ `PLAN-RENAME-001`

### ⚪ Low (1)

- **&#96;aws\_s3\_bucket\_versioning.logs&#96; will be destroyed.** The S3 bucket versioning is destroyed because it is no longer in the configuration. _Confirm nothing outside this configuration still depends on it._ `PLAN-DELETE-001`

### Routine changes (1)

- `aws\_s3\_bucket\_versioning.audit\_logs`: created

### Moved (1)

- `aws\_sqs\_queue.events` → `aws\_sqs\_queue.event\_queue` (address change only, nothing is recreated)

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
