# Shared provider block for every fixture. Points the AWS provider at a local
# moto mock (http://localhost:5000) so "before" state is created by a real
# `terraform apply` and the plan JSON is produced by Terraform itself. Nothing
# here touches a real AWS account.
terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region                      = "eu-west-2"
  access_key                  = "testing"
  secret_key                  = "testing"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true

  default_tags {
    tags = {
      Project    = "fixture"
      CostCentre = var.cost_centre
    }
  }
}

variable "cost_centre" {
  type    = string
  default = "platform"
}
