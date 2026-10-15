<!-- tfimpact -->
## tfimpact: 1 change destroys data

**Plan:** 1 to add, 0 to change, 1 to destroy (1 replacement, 4 unchanged) · Terraform/OpenTofu 1.15.5 · 5 resources

### 🔴 Critical (1)

- **&#96;aws\_db\_instance.app&#96; will be destroyed and recreated, losing the database and everything stored in it.** The RDS database instance is replaced because &#96;storage\_encrypted&#96; \(false → true\) cannot be changed in place. No final snapshot will be taken \(&#96;skip\_final\_snapshot&#96; = true\); deletion protection is off. _If the change is intended, take a snapshot or backup first and plan the cutover. If the replacement is a surprise, revert &#96;storage\_encrypted&#96;. &#96;lifecycle { prevent\_destroy = true }&#96; makes Terraform refuse a plan like this._ `PLAN-DATA-002`

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
