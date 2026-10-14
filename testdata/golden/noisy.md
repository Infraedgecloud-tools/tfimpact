<!-- tfimpact -->
## tfimpact: 1 change destroys data, 1 change widens exposure, 1 change interrupts service, 38 routine

**Plan:** 1 to add, 40 to change, 1 to destroy (1 replacement, 6 unchanged) · Terraform/OpenTofu 1.15.5 · 47 resources

### 🔴 Critical (2)

- **`aws_dynamodb_table.sessions` will be destroyed and recreated, losing every item in the table.** The DynamoDB table is replaced because `hash_key` ("session_id" → "user_id") cannot be changed in place. Deletion protection is off; point-in-time recovery is off, so there is no automatic backup to restore from. _If the change is intended, take a snapshot or backup first and plan the cutover. If the replacement is a surprise, revert `hash_key`. `lifecycle { prevent_destroy = true }` makes Terraform refuse a plan like this._ `PLAN-DATA-002`
- **`aws_security_group.bastion` opens SSH to the internet.** The security group gains ingress for tcp port 22 from 0.0.0.0/0. _Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change._ `PLAN-NET-001`

### 🟠 High (1)

- **`aws_instance.bastion`: changing `user_data` in place has a service effect.** `user_data` changes (value hidden): the instance is stopped and started to apply the new user data (`user_data_replace_on_change` is false); it keeps its ID but gets a new public IP unless it has an Elastic IP, and the script only re-runs if the AMI's cloud-init is configured to. _Apply in a maintenance window or confirm that clients tolerate the interruption._ `PLAN-DISRUPT-001`

### Routine changes (38)

- <details><summary>10 × `aws_cloudwatch_log_group`: `retention_in_days` 30 → 90</summary>

  - `aws_cloudwatch_log_group.service["api"]`
  - `aws_cloudwatch_log_group.service["auth"]`
  - `aws_cloudwatch_log_group.service["billing"]`
  - `aws_cloudwatch_log_group.service["catalog"]`
  - `aws_cloudwatch_log_group.service["checkout"]`
  - `aws_cloudwatch_log_group.service["gateway"]`
  - `aws_cloudwatch_log_group.service["orders"]`
  - `aws_cloudwatch_log_group.service["payments"]`
  - `aws_cloudwatch_log_group.service["search"]`
  - `aws_cloudwatch_log_group.service["shipping"]`

  </details>
- `aws_lb.public`: `subnet_mapping`, `subnets`
- `aws_sqs_queue.work["notify"]`, `aws_sqs_queue.work["orders"]`, `aws_sqs_queue.work["payments"]`: `visibility_timeout_seconds` 60 → 120
- <details><summary>24 × `aws_ssm_parameter`: tags only</summary>

  - `aws_ssm_parameter.service_version["api"]`
  - `aws_ssm_parameter.service_version["auth"]`
  - `aws_ssm_parameter.service_version["billing"]`
  - `aws_ssm_parameter.service_version["catalog"]`
  - `aws_ssm_parameter.service_version["checkout"]`
  - `aws_ssm_parameter.service_version["email"]`
  - `aws_ssm_parameter.service_version["export"]`
  - `aws_ssm_parameter.service_version["feed"]`
  - `aws_ssm_parameter.service_version["gateway"]`
  - `aws_ssm_parameter.service_version["ingest"]`
  - `aws_ssm_parameter.service_version["inventory"]`
  - `aws_ssm_parameter.service_version["jobs"]`
  - `aws_ssm_parameter.service_version["ledger"]`
  - `aws_ssm_parameter.service_version["media"]`
  - `aws_ssm_parameter.service_version["notify"]`
  - `aws_ssm_parameter.service_version["orders"]`
  - `aws_ssm_parameter.service_version["payments"]`
  - `aws_ssm_parameter.service_version["pricing"]`
  - `aws_ssm_parameter.service_version["profile"]`
  - `aws_ssm_parameter.service_version["reports"]`
  - `aws_ssm_parameter.service_version["search"]`
  - `aws_ssm_parameter.service_version["shipping"]`
  - `aws_ssm_parameter.service_version["tax"]`
  - `aws_ssm_parameter.service_version["webhooks"]`

  </details>

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
