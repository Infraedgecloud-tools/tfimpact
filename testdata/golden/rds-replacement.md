<!-- tfimpact -->
## tfimpact: 1 change destroys data

**Plan:** 1 to add, 0 to change, 1 to destroy (1 replacement, 4 unchanged) · Terraform/OpenTofu 1.15.5 · 5 resources

### 🔴 Critical (1)

- **`aws_db_instance.app` will be destroyed and recreated; the database and everything stored in it is lost.** The RDS database instance is replaced because `storage_encrypted` (false → true) cannot be changed in place. No final snapshot will be taken (skip_final_snapshot = true); deletion protection is off. _If the change is intended, take a snapshot or backup first and plan the cutover. If the replacement is a surprise, revert `storage_encrypted`. `lifecycle { prevent_destroy = true }` makes Terraform refuse a plan like this._ `PLAN-DATA-002`

<sub>tfimpact test by [InfraEdge](https://infraedge.org) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
