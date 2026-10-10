# The bucket is renamed in code with no moved block: Terraform plans a destroy
# of aws_s3_bucket.logs and a create of aws_s3_bucket.audit_logs with the same
# bucket name. The queue is renamed WITH a moved block and should be a no-op
# carrying previous_address. The table is untouched.
resource "aws_s3_bucket" "audit_logs" {
  bucket = "fixture-alb-access-logs-eu-west-2"
}

resource "aws_s3_bucket_versioning" "audit_logs" {
  bucket = aws_s3_bucket.audit_logs.id
  versioning_configuration {
    status = "Enabled"
  }
}

moved {
  from = aws_sqs_queue.events
  to   = aws_sqs_queue.event_queue
}

resource "aws_sqs_queue" "event_queue" {
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
