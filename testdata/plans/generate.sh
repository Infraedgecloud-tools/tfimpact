#!/usr/bin/env bash
# Regenerates plan.json / plan.txt for every fixture.
#
# Requires: terraform (>= 1.5), docker, curl. Starts a local moto server
# (https://github.com/getmoto/moto) as a fake AWS so the "before" state is
# created by a real apply. Nothing contacts a real AWS account: the provider is
# pointed at http://localhost:5000 via AWS_ENDPOINT_URL with dummy keys.
#
#   ./generate.sh            # all fixtures
#   ./generate.sh rename     # one fixture
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
work="${TMPDIR:-/tmp}/plan-explainer-fixtures-work"
export TF_PLUGIN_CACHE_DIR="${TF_PLUGIN_CACHE_DIR:-$work/plugin-cache}"
export TF_IN_AUTOMATION=1
export AWS_ENDPOINT_URL="http://localhost:5000"
export AWS_ACCESS_KEY_ID=testing AWS_SECRET_ACCESS_KEY=testing AWS_DEFAULT_REGION=eu-west-2
mkdir -p "$TF_PLUGIN_CACHE_DIR"

if ! curl -fs -o /dev/null http://localhost:5000/moto-api/; then
  docker rm -f plan-explainer-moto >/dev/null 2>&1 || true
  docker run -d --name plan-explainer-moto -p 5000:5000 motoserver/moto:latest >/dev/null
  for _ in $(seq 1 30); do curl -fs -o /dev/null http://localhost:5000/moto-api/ && break; sleep 1; done
fi

fixtures=("$@")
if [ ${#fixtures[@]} -eq 0 ]; then
  fixtures=(rds-replacement rename iam-widening tag-only noisy)
fi

for f in "${fixtures[@]}"; do
  echo "== $f"
  curl -fs -X POST -o /dev/null http://localhost:5000/moto-api/reset
  d="$work/$f"; rm -rf "$d"; mkdir -p "$d"
  cp "$here/provider.tf" "$d/"; cp "$here/$f/before.tf" "$d/main.tf"
  (cd "$d" && terraform init -input=false -no-color >/dev/null && terraform apply -input=false -auto-approve -no-color >/dev/null)
  cp "$here/$f/after.tf" "$d/main.tf"
  varfile=(); [ -f "$here/$f/after.tfvars" ] && varfile=(-var-file="$here/$f/after.tfvars")
  (cd "$d" && terraform plan -input=false -no-color -refresh=false -out=tfplan "${varfile[@]}" > "$here/$f/plan.txt" 2>&1 || true)
  (cd "$d" && terraform show -json tfplan | python3 -m json.tool > "$here/$f/plan.json")
  (cd "$d" && terraform version -json | python3 -c 'import json,sys; v=json.load(sys.stdin); print("terraform", v["terraform_version"], "aws provider", v["provider_selections"].get("registry.terraform.io/hashicorp/aws"))') > "$here/$f/versions.txt"
  python3 - "$here/$f/plan.json" <<'PY'
import json,sys,collections
p=json.load(open(sys.argv[1]))
c=collections.Counter("/".join(r["change"]["actions"]) for r in p.get("resource_changes",[]))
print("  format", p["format_version"], dict(c))
PY
done
