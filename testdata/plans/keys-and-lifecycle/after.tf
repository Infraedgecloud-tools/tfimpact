# The for_each keys are renamed: Terraform plans a delete of work["orders"] and a
# create of work["order_events"] with the same queue name. The role is renamed with
# create_before_destroy, so it is a replacement that creates first. The log group
# is removed from state with a removed block but not destroyed (forget).
resource "aws_sqs_queue" "work" {
  for_each = { order_events = "orders", payments = "payments" }
  name     = "fixture-lifecycle-${each.value}"
}

resource "aws_iam_role" "deploy" {
  name = "fixture-lifecycle-deploy-v2"
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

removed {
  from = aws_cloudwatch_log_group.legacy
  lifecycle {
    destroy = false
  }
}

resource "aws_ssm_parameter" "flag" {
  name  = "/fixture/lifecycle/flag"
  type  = "String"
  value = "on"
}
