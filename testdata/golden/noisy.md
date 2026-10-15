<!-- tfimpact -->
## tfimpact: 1 change destroys data, 1 change widens exposure, 1 change interrupts service, 38 routine

**Plan:** 1 to add, 40 to change, 1 to destroy (1 replacement, 6 unchanged) · Terraform/OpenTofu 1.15.5 · 47 resources

### 🔴 Critical (2)

- **&#96;aws\_dynamodb\_table.sessions&#96; will be destroyed and recreated, losing every item in the table.** The DynamoDB table is replaced because &#96;hash\_key&#96; \(&#34;session\_id&#34; → &#34;user\_id&#34;\) cannot be changed in place. Deletion protection is off; point-in-time recovery is off, so there is no automatic backup to restore from. _If the change is intended, take a snapshot or backup first and plan the cutover. If the replacement is a surprise, revert &#96;hash\_key&#96;. &#96;lifecycle { prevent\_destroy = true }&#96; makes Terraform refuse a plan like this._ `PLAN-DATA-002`
- **&#96;aws\_security\_group.bastion&#96; opens SSH to the internet.** The security group gains ingress for tcp port 22 from 0.0.0.0/0. _Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change._ `PLAN-NET-001`

### 🟠 High (1)

- **&#96;aws\_instance.bastion&#96;: changing &#96;user\_data&#96; in place has a service effect.** &#96;user\_data&#96; changes \(value hidden\): the instance is stopped and started to apply the new user data \(&#96;user\_data\_replace\_on\_change&#96; is false\); it keeps its ID but gets a new public IP unless it has an Elastic IP, and the script only re-runs if the AMI&#39;s cloud-init is configured to. _Apply in a maintenance window or confirm that clients tolerate the interruption._ `PLAN-DISRUPT-001`

### Routine changes (38)

- <details><summary>10 × `aws\_cloudwatch\_log\_group`: &#96;retention\_in\_days&#96; 30 → 90</summary>

  - `aws\_cloudwatch\_log\_group.service\[&#34;api&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;auth&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;billing&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;catalog&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;checkout&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;gateway&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;orders&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;payments&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;search&#34;\]`
  - `aws\_cloudwatch\_log\_group.service\[&#34;shipping&#34;\]`

  </details>
- `aws\_lb.public`: &#96;subnet\_mapping&#96;, &#96;subnets&#96;
- `aws\_sqs\_queue.work\[&#34;notify&#34;\]`, `aws\_sqs\_queue.work\[&#34;orders&#34;\]`, `aws\_sqs\_queue.work\[&#34;payments&#34;\]`: &#96;visibility\_timeout\_seconds&#96; 60 → 120
- <details><summary>24 × `aws\_ssm\_parameter`: tags only</summary>

  - `aws\_ssm\_parameter.service\_version\[&#34;api&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;auth&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;billing&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;catalog&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;checkout&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;email&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;export&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;feed&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;gateway&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;ingest&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;inventory&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;jobs&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;ledger&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;media&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;notify&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;orders&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;payments&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;pricing&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;profile&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;reports&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;search&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;shipping&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;tax&#34;\]`
  - `aws\_ssm\_parameter.service\_version\[&#34;webhooks&#34;\]`

  </details>

<sub>tfimpact test by [InfraEdge](https://infraedgecloud.com) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked</sub>
