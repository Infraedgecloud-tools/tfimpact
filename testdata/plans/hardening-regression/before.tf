resource "aws_s3_bucket" "data" {
  bucket = "fixture-customer-exports-eu-west-2"
}

resource "aws_s3_bucket_public_access_block" "data" {
  bucket                  = aws_s3_bucket.data.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_cloudtrail" "main" {
  name                          = "fixture-trail"
  s3_bucket_name                = aws_s3_bucket.data.id
  enable_logging                = true
  is_multi_region_trail         = true
  enable_log_file_validation    = true
  include_global_service_events = true
}

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_subnet" "a" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.1.0/24"
  availability_zone = "eu-west-2a"
}

resource "aws_subnet" "b" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.2.0/24"
  availability_zone = "eu-west-2b"
}

resource "aws_security_group" "db" {
  name   = "fixture-db"
  vpc_id = aws_vpc.main.id
}

resource "aws_vpc_security_group_ingress_rule" "postgres" {
  security_group_id = aws_security_group.db.id
  ip_protocol       = "tcp"
  from_port         = 5432
  to_port           = 5432
  cidr_ipv4         = "10.0.0.0/8"
}

resource "aws_db_subnet_group" "app" {
  name       = "fixture-app"
  subnet_ids = [aws_subnet.a.id, aws_subnet.b.id]
}

resource "aws_db_instance" "app" {
  identifier              = "fixture-app"
  engine                  = "postgres"
  engine_version          = "15.4"
  instance_class          = "db.t3.medium"
  allocated_storage       = 50
  username                = "app"
  password                = "fixture-password-not-real"
  db_subnet_group_name    = aws_db_subnet_group.app.name
  vpc_security_group_ids  = [aws_security_group.db.id]
  backup_retention_period = 7
  deletion_protection     = true
  publicly_accessible     = false
  skip_final_snapshot     = true
}

resource "aws_secretsmanager_secret" "db" {
  name                    = "fixture/db/password"
  recovery_window_in_days = 30
}

resource "aws_kms_key" "data" {
  description             = "fixture data key"
  deletion_window_in_days = 7
  enable_key_rotation     = true
}

resource "aws_iam_role" "lambda" {
  name = "fixture-lambda"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}
