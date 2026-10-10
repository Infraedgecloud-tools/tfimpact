<!-- tfimpact -->
## tfimpact: 1 change destroys data, 1 minor, 1 routine

**Plan:** 2 to add, 0 to change, 2 to destroy (1 move, 2 unchanged) · Terraform/OpenTofu 1.15.5 · 6 resources

### 🔴 Critical (1)

- **`aws_s3_bucket.logs` is renamed to `aws_s3_bucket.audit_logs` without a moved block, so it will be destroyed and recreated.** `aws_s3_bucket.logs` will be destroyed because it left the configuration, and `aws_s3_bucket.audit_logs` will be created with the same `bucket` ("fixture-alb-access-logs-eu-west-2"). Terraform does not know they are the same object. Every object and object version in the bucket would be lost with the S3 bucket. _Add a moved block (`from = aws_s3_bucket.logs`, `to = aws_s3_bucket.audit_logs`) so Terraform changes the address in state instead. Applying as planned also tries to create a resource with the same bucket, which fails for globally named resources._ `PLAN-RENAME-001`

### ⚪ Low (1)

- **`aws_s3_bucket_versioning.logs` will be destroyed.** The S3 bucket versioning is destroyed because it is no longer in the configuration. _Confirm nothing outside this configuration still depends on it._ `PLAN-DELETE-001`

### Routine changes (1)

- `aws_s3_bucket_versioning.audit_logs`: created

### Moved (1)

- `aws_sqs_queue.events` → `aws_sqs_queue.event_queue` (address change only, nothing is recreated)

<sub>tfimpact test by [InfraEdge](https://infraedge.org) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
