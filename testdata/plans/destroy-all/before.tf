resource "aws_vpc" "main" {
  cidr_block = "10.2.0.0/16"
}

resource "aws_subnet" "a" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.2.1.0/24"
  availability_zone = "eu-west-2a"
}

resource "aws_subnet" "b" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.2.2.0/24"
  availability_zone = "eu-west-2b"
}

resource "aws_db_subnet_group" "app" {
  name       = "fixture-destroy"
  subnet_ids = [aws_subnet.a.id, aws_subnet.b.id]
}

resource "aws_db_instance" "app" {
  identifier                = "fixture-destroy"
  engine                    = "mysql"
  engine_version            = "8.0.35"
  instance_class            = "db.t3.small"
  allocated_storage         = 20
  username                  = "app"
  password                  = "fixture-password-not-real"
  db_subnet_group_name      = aws_db_subnet_group.app.name
  deletion_protection       = true
  skip_final_snapshot       = false
  final_snapshot_identifier = "fixture-destroy-final"
}

resource "aws_dynamodb_table" "orders" {
  name         = "fixture-destroy-orders"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "order_id"
  attribute {
    name = "order_id"
    type = "S"
  }
  point_in_time_recovery {
    enabled = true
  }
}

resource "aws_cloudwatch_log_group" "keep" {
  name         = "/fixture/destroy/keep"
  skip_destroy = true
}

resource "aws_s3_bucket" "exports" {
  bucket        = "fixture-destroy-exports-eu-west-2"
  force_destroy = true
}

resource "aws_sns_topic" "alerts" {
  name = "fixture-destroy-alerts"
}

resource "aws_lb" "public" {
  name               = "fixture-destroy"
  load_balancer_type = "application"
  subnets            = [aws_subnet.a.id, aws_subnet.b.id]
}
