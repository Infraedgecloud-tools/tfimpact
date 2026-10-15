<!-- tfimpact -->
## tfimpact: no consequences found, 3 routine changes

**Plan:** 1 to add, 2 to change, 0 to destroy · Terraform/OpenTofu 1.15.5 · 3 resources

### Routine changes (3)

- `aws\_cloudwatch\_log\_group.app`: &#96;retention\_in\_days&#96; 30 → 90
- `aws\_sqs\_queue.events`: &#96;visibility\_timeout\_seconds&#96; 120 → 30
- `aws\_ssm\_parameter.flag`: created

### Changed outside Terraform (2)

- `aws\_sqs\_queue.events`: changed outside Terraform: &#96;visibility\\\_timeout\\\_seconds&#96;
- `aws\_ssm\_parameter.flag`: deleted outside Terraform

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
