resource "aws_s3_bucket" "logs" {
  bucket = "fixture-alb-access-logs-eu-west-2"
}

resource "aws_s3_bucket_versioning" "logs" {
  bucket = aws_s3_bucket.logs.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_sqs_queue" "events" {
  name                       = "fixture-events"
  visibility_timeout_seconds = 60
}

resource "aws_dynamodb_table" "orders" {
  name         = "fixture-orders"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "order_id"
  attribute {
    name = "order_id"
    type = "S"
  }
}
