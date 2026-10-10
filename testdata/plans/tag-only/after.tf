resource "aws_s3_bucket" "artifacts" {
  bucket = "fixture-artifacts-eu-west-2"
  tags   = { Name = "artifacts", Owner = "platform-team" }
}

resource "aws_sqs_queue" "jobs" {
  name = "fixture-jobs"
  tags = { Name = "jobs", Owner = "platform-team" }
}

resource "aws_dynamodb_table" "sessions" {
  name         = "fixture-sessions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "session_id"
  attribute {
    name = "session_id"
    type = "S"
  }
  tags = { Name = "sessions", Owner = "platform-team" }
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/fixture/app"
  retention_in_days = 30
  tags              = { Name = "app-logs", Owner = "platform-team" }
}

resource "aws_ssm_parameter" "feature_flag" {
  name  = "/fixture/feature-flag"
  type  = "String"
  value = "on"
  tags  = { Name = "feature-flag", Owner = "platform-team" }
}
