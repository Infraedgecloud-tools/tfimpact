resource "aws_sqs_queue" "work" {
  for_each = toset(["orders", "payments"])
  name     = "fixture-lifecycle-${each.key}"
}

resource "aws_iam_role" "deploy" {
  name = "fixture-lifecycle-deploy-v1"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "codebuild.amazonaws.com" }
    }]
  })
  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_cloudwatch_log_group" "legacy" {
  name = "/fixture/lifecycle/legacy"
}

resource "aws_ssm_parameter" "flag" {
  name  = "/fixture/lifecycle/flag"
  type  = "String"
  value = "on"
}
