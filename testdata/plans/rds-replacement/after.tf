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

resource "aws_db_subnet_group" "app" {
  name       = "app-db"
  subnet_ids = [aws_subnet.a.id, aws_subnet.b.id]
}

resource "aws_db_instance" "app" {
  identifier              = "app-db-eu"
  engine                  = "postgres"
  engine_version          = "15.4"
  instance_class          = "db.m6g.large"
  allocated_storage       = 100
  username                = "app"
  password                = "fixture-password-not-real"
  db_subnet_group_name    = aws_db_subnet_group.app.name
  storage_encrypted       = true
  backup_retention_period = 7
  deletion_protection     = false
  skip_final_snapshot     = true
  tags = {
    Name = "app-db"
  }
}
