# Rules

Every finding carries one of these rule IDs. Suppressions in `.tfimpact.toml` refer to
them, and they never change meaning; a rule that needs different semantics gets a new ID.
`tfimpact rules` prints the same table.

| Rule | Domain | Severity | What it means |
|---|---|---|---|
| `PLAN-DATA-001` | data | high–critical | A resource that holds data is destroyed. The finding names what is lost and which mitigations apply or are missing: final snapshot, deletion protection, point-in-time recovery, recovery window, backups. Critical unless a safeguard is present, then high. |
| `PLAN-DATA-002` | data | high–critical | The same resource is destroyed and recreated. The finding names the attribute that forced the replacement (`replace_paths`) and its before and after values. |
| `PLAN-RENAME-001` | change | high–critical | A resource was renamed in code without a `moved` block. The plan destroys one address and creates another with the same identity attribute (`bucket`, `name`, `identifier`, ...). Critical when the type holds data. Also fires when a `for_each` key or `count` index changes. |
| `PLAN-REPLACE-001` | availability | medium–high | A resource without data is destroyed and recreated. High when the knowledge file says replacement interrupts service: a load balancer's DNS name, a NAT gateway's IP, a security group's attachments, a cluster. Not raised when the knowledge file says the replacement changes nothing outside Terraform (a certificate validation wait, an underscore-named DNS record); those are listed as routine. |
| `PLAN-DELETE-001` | change | low–high | A resource is destroyed. Low by default, since Terraform already says so; raised when the resource is protective (CloudTrail, flow log, public access block, GuardDuty, Config recorder, OIDC provider) or its deletion interrupts service. |
| `PLAN-DISRUPT-001` | availability | low–high | An in-place update that still interrupts service: `instance_class` restarts an RDS instance, `user_data` stops and starts an EC2 instance, `identifier` changes an RDS endpoint hostname, `version` upgrades an EKS control plane. The severity comes from the knowledge file. |
| `PLAN-IAM-002` | security | low | An IAM policy document is not known until apply, because it comes from a data source, a template or another resource's attribute. Nothing was assessed, and the finding says so rather than letting a routine line stand in for a check that did not run. |
| `PLAN-IAM-001` | security | low–critical | A policy, trust policy or resource policy document grants more than before: a new wildcard action, `Resource = "*"`, `Principal = "*"` (critical without a restricting condition), a removed condition, a condition widened to a wildcard or from `StringEquals` to `StringLike`, a federated trust with no `sub` condition, a removed `Deny`, or added actions (low). New policies report only high and critical widenings. |
| `PLAN-NET-001` | security | high–critical | A security group, security group rule or VPC ingress rule opens ingress to `0.0.0.0/0` or `::/0` that was not open before. Critical when the port range covers SSH, RDP, a database port, NFS, SMB, or all ports. |
| `PLAN-EXPOSE-001` | security | low–critical | A single attribute moves to an unsafe value or a resource is created with one: public access block flags, `publicly_accessible`, encryption off, deletion protection off, logging off, IMDSv1 allowed, a load balancer made internet-facing, a Lambda invokable by anyone. |

## What is not a finding

- Tag-only updates, no-ops and data source reads are counted and listed as routine.
- Moves (`moved` blocks) are listed under their own heading as address changes.
- Drift (`resource_drift`) is listed under "Changed outside Terraform".
- Creates without an exposure problem are listed as routine, grouped by type.

## Severity ladder

`low` < `medium` < `high` < `critical`. `--fail-on LEVEL` exits 1 when any unsuppressed
finding is at or above LEVEL. Suppressed findings never trip it.

## Knowledge

The rules are generic; what makes them say something useful about AWS is the knowledge
under [`internal/knowledge/aws/`](../internal/knowledge/aws/), one TOML file per resource
type. `tfimpact knowledge` lists the types; `tfimpact knowledge aws_db_instance` shows
one. A resource type without a knowledge file still gets `PLAN-REPLACE-001` and
`PLAN-DELETE-001` with the generic wording.
