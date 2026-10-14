<!-- tfimpact -->
## tfimpact: 3 changes destroy data, 5 changes interrupt service, 2 minor

**Plan:** 0 to add, 0 to change, 10 to destroy · Terraform/OpenTofu 1.15.5 · 10 resources

### 🔴 Critical (1)

- **`aws_s3_bucket.exports` will be destroyed, losing every object and object version in the bucket.** The S3 bucket is destroyed because it is no longer in the configuration. `force_destroy` = true, so all objects and versions are deleted first without asking. _If removing it is intended, take a backup or snapshot first and keep it until the change has settled. If it was renamed or moved in code, add a `moved` block instead of recreating it._ `PLAN-DATA-001`

### 🟠 High (2)

- **`aws_db_instance.app` will be destroyed, losing the database and everything stored in it.** The RDS database instance is destroyed because it is no longer in the configuration. Automated backups are disabled (`backup_retention_period` = 0). Deletion protection is on, so AWS will refuse the delete and the apply will fail before anything is lost; a final snapshot will be taken first, so the data can be restored. _If removing it is intended, take a backup or snapshot first and keep it until the change has settled. If it was renamed or moved in code, add a `moved` block instead of recreating it._ `PLAN-DATA-001`
- **`aws_dynamodb_table.orders` will be destroyed, losing every item in the table.** The DynamoDB table is destroyed because it is no longer in the configuration. Deletion protection is off. Point-in-time recovery is on, so AWS keeps a system backup for 35 days after deletion that can be restored to a new table. _If removing it is intended, take a backup or snapshot first and keep it until the change has settled. If it was renamed or moved in code, add a `moved` block instead of recreating it._ `PLAN-DATA-001`

### 🟡 Medium (5)

- **`aws_lb.public` will be destroyed.** The load balancer is destroyed because it is no longer in the configuration. Anything still pointing at it (DNS names, clients, attachments, dependants in other configurations) stops working. _Confirm nothing outside this configuration still depends on it._ `PLAN-DELETE-001`
- **`aws_sns_topic.alerts` will be destroyed.** The SNS topic is destroyed because it is no longer in the configuration. Anything still pointing at it (DNS names, clients, attachments, dependants in other configurations) stops working. _Confirm nothing outside this configuration still depends on it._ `PLAN-DELETE-001`
- **`aws_subnet.a` will be destroyed.** The subnet is destroyed because it is no longer in the configuration. Anything still pointing at it (DNS names, clients, attachments, dependants in other configurations) stops working. _Confirm nothing outside this configuration still depends on it._ `PLAN-DELETE-001`
- **`aws_subnet.b` will be destroyed.** The subnet is destroyed because it is no longer in the configuration. Anything still pointing at it (DNS names, clients, attachments, dependants in other configurations) stops working. _Confirm nothing outside this configuration still depends on it._ `PLAN-DELETE-001`
- **`aws_vpc.main` will be destroyed.** The VPC is destroyed because it is no longer in the configuration. Anything still pointing at it (DNS names, clients, attachments, dependants in other configurations) stops working. _Confirm nothing outside this configuration still depends on it._ `PLAN-DELETE-001`

### ⚪ Low (2)

- **`aws_cloudwatch_log_group.keep` is removed from Terraform state; every stored log event stays in AWS.** The CloudWatch log group leaves this configuration because it is no longer in the configuration. `skip_destroy` = true, so the log group is only removed from state and its events stay in AWS. _Nothing is deleted, but the resource is no longer managed here. Import it elsewhere or delete it by hand when it is no longer needed._ `PLAN-DATA-001`
- **`aws_db_subnet_group.app` will be destroyed.** The RDS subnet group is destroyed because it is no longer in the configuration. _Confirm nothing outside this configuration still depends on it._ `PLAN-DELETE-001`

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
