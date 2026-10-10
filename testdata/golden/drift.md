<!-- tfimpact -->
## tfimpact: no consequences found, 3 routine changes

**Plan:** 1 to add, 2 to change, 0 to destroy · Terraform/OpenTofu 1.15.5 · 3 resources

### Routine changes (3)

- `aws_cloudwatch_log_group.app`: `retention_in_days` 30 → 90
- `aws_sqs_queue.events`: `visibility_timeout_seconds` 120 → 30
- `aws_ssm_parameter.flag`: created

### Changed outside Terraform (2)

- `aws_sqs_queue.events`: changed outside Terraform: `visibility_timeout_seconds`
- `aws_ssm_parameter.flag`: deleted outside Terraform

<sub>tfimpact test by [InfraEdge](https://infraedge.org) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
