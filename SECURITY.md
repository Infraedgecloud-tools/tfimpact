# Security policy

## Supported versions

Only the latest minor release receives security fixes.

## Reporting a vulnerability

Report privately through GitHub private vulnerability reporting:
<https://github.com/infraedge-tools/tfimpact/security/advisories/new>

Please do not open a public issue for a vulnerability. We acknowledge reports within 14 days.

## What counts as a security issue

- tfimpact prints a value that the plan marks as sensitive.
- tfimpact classifies a destructive change as safe, for example a data-loss replacement reported as low impact.
- Anything that lets a crafted plan file run code, write outside the requested output path, or exfiltrate data.
- A flaw in the release artefacts, container image or GitHub Action that lets a third party tamper with what users run.

A rule that is noisy on a safe change is a bug, not a security issue. Use the false-positive issue template for those.

## Redact before you attach

Plan JSON from `terraform show -json` or `tofu show -json` contains secrets in plain text, including values that the plan marks as sensitive. Remove secrets, account IDs, ARNs and other real identifiers from any plan, log or fixture before you attach it to a report.
