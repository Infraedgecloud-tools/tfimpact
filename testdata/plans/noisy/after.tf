locals {
  services = toset([
    "api", "auth", "billing", "catalog", "checkout", "email", "export", "feed",
    "gateway", "ingest", "inventory", "jobs", "ledger", "media", "notify", "orders",
    "payments", "pricing", "profile", "reports", "search", "shipping", "tax", "webhooks",
  ])
  log_groups = toset(["api", "auth", "billing", "catalog", "checkout", "gateway", "orders", "payments", "search", "shipping"])
}

resource "aws_ssm_parameter" "service_version" {
  for_each = local.services
  name     = "/fixture/${each.key}/version"
  type     = "String"
  value    = "1.0.0"
  tags     = { Service = each.key, Tier = "backend", Owner = "platform-team" }
}

resource "aws_cloudwatch_log_group" "service" {
  for_each          = local.log_groups
  name              = "/fixture/${each.key}"
  retention_in_days = 90
}

resource "aws_sqs_queue" "work" {
  for_each                   = toset(["orders", "payments", "notify"])
  name                       = "fixture-${each.key}"
  visibility_timeout_seconds = 120
}

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}

resource "aws_subnet" "public" {
  for_each          = { a = "10.0.1.0/24", b = "10.0.2.0/24", c = "10.0.3.0/24" }
  vpc_id            = aws_vpc.main.id
  cidr_block        = each.value
  availability_zone = "eu-west-2${each.key}"
}

resource "aws_security_group" "web" {
  name   = "fixture-web"
  vpc_id = aws_vpc.main.id
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "bastion" {
  name   = "fixture-bastion"
  vpc_id = aws_vpc.main.id
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_lb" "public" {
  name               = "fixture-public"
  load_balancer_type = "application"
  subnets            = [aws_subnet.public["b"].id, aws_subnet.public["c"].id]
  security_groups    = [aws_security_group.web.id]
}

resource "aws_instance" "bastion" {
  ami                    = "ami-12c6146b"
  instance_type          = "t3.micro"
  subnet_id              = aws_subnet.public["a"].id
  vpc_security_group_ids = [aws_security_group.bastion.id]
  user_data              = "#!/bin/bash\necho fixture v2\n"
  tags                   = { Name = "bastion" }
}

resource "aws_dynamodb_table" "sessions" {
  name         = "fixture-sessions"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "user_id"
  attribute {
    name = "user_id"
    type = "S"
  }
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
