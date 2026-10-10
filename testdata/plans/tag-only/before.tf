resource "aws_s3_bucket" "artifacts" {
  bucket = "fixture-artifacts-eu-west-2"
  tags   = { Name = "artifacts" }
}

resource "aws_sqs_queue" "jobs" {
  name = "fixture-jobs"
  tags = { Name = "jobs" }
}

resource "aws_dynamodb_table" "sessions" {
  name         = "fixture-sessions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "session_id"
  attribute {
    name = "session_id"
    type = "S"
  }
  tags = { Name = "sessions" }
}

resource "aws_cloudwatch_log_group" "app" {
  name              = "/fixture/app"
  retention_in_days = 30
  tags              = { Name = "app-logs" }
}

resource "aws_ssm_parameter" "feature_flag" {
  name  = "/fixture/feature-flag"
  type  = "String"
  value = "on"
  tags  = { Name = "feature-flag" }
}
