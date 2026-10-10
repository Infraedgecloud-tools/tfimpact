resource "aws_sqs_queue" "events" {
  name                       = "fixture-drift-events"
  visibility_timeout_seconds = 30
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/fixture/drift/app"
  retention_in_days = 30
}

resource "aws_ssm_parameter" "flag" {
  name  = "/fixture/drift/flag"
  type  = "String"
  value = "on"
}
