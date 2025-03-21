terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 4.0"
    }
  }
}

# Configure the AWS Provider
provider "aws" {
  region = var.aws_region
}

# Local variables
locals {
  lambda_memory_sizes   = [128, 512, 1024, 2048, 3008]
  operation_types       = ["read", "write"]
  function_types        = ["sequential", "parallel"]
  database_types        = ["dynamodb", "immudb", "timestream"]
  lambda_function_names = {
    for config in setproduct(local.database_types, local.operation_types, local.function_types) :
    join("-", config) => {
      database_type = config[0],
      operation_type = config[1],
      function_type = config[2]
    }
  }
}

# IAM Role for Lambda functions
resource "aws_iam_role" "lambda_role" {
  name = "lambda_benchmark_role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

# IAM Policy for Lambda functions
resource "aws_iam_policy" "lambda_policy" {
  name        = "lambda_benchmark_policy"
  description = "Policy for Lambda benchmark functions"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ],
        Effect   = "Allow",
        Resource = "arn:aws:logs:*:*:*"
      },
      {
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan",
          "dynamodb:BatchGetItem",
          "dynamodb:BatchWriteItem",
          "dynamodb:TransactGetItems",
          "dynamodb:TransactWriteItems"
        ],
        Effect   = "Allow",
        Resource = "arn:aws:dynamodb:*:*:table/*"
      },
      {
        Action = [
          "timestream:DescribeEndpoints",
          "timestream:SelectValues",
          "timestream:WriteRecords"
        ],
        Effect   = "Allow",
        Resource = "*"
      },
      {
        Action = [
          "ec2:DescribeInstances"
        ],
        Effect   = "Allow",
        Resource = "*"
      }
    ]
  })
}

# Attach the policy to the role
resource "aws_iam_role_policy_attachment" "lambda_policy_attachment" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.lambda_policy.arn
}

# VPC for ImmuDB
resource "aws_vpc" "immudb_vpc" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name    = "immudb-vpc"
    Project = "lambda-gopher-benchmark"
  }
}

# Subnet for ImmuDB
resource "aws_subnet" "immudb_subnet" {
  vpc_id                  = aws_vpc.immudb_vpc.id
  cidr_block              = "10.0.1.0/24"
  map_public_ip_on_launch = true
  availability_zone       = "${var.aws_region}a"

  tags = {
    Name    = "immudb-subnet"
    Project = "lambda-gopher-benchmark"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "immudb_igw" {
  vpc_id = aws_vpc.immudb_vpc.id

  tags = {
    Name    = "immudb-igw"
    Project = "lambda-gopher-benchmark"
  }
}

# Route Table
resource "aws_route_table" "immudb_route_table" {
  vpc_id = aws_vpc.immudb_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.immudb_igw.id
  }

  tags = {
    Name    = "immudb-route-table"
    Project = "lambda-gopher-benchmark"
  }
}

# Route Table Association
resource "aws_route_table_association" "immudb_route_table_assoc" {
  subnet_id      = aws_subnet.immudb_subnet.id
  route_table_id = aws_route_table.immudb_route_table.id
}

# Security Group for ImmuDB
resource "aws_security_group" "immudb_sg" {
  name        = "immudb-security-group"
  description = "Security group for ImmuDB EC2 instance"
  vpc_id      = aws_vpc.immudb_vpc.id

  # ImmuDB default ports
  ingress {
    from_port   = 3322
    to_port     = 3322
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]  # Allow from anywhere - in production limit this
    description = "ImmuDB API port"
  }

  ingress {
    from_port   = 9497
    to_port     = 9497
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "ImmuDB metrics port"
  }

  # SSH access for management - only if SSH public key is provided
  dynamic "ingress" {
    for_each = var.ssh_public_key != "" ? [1] : []
    content {
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      cidr_blocks = ["0.0.0.0/0"]  # In production, restrict to your IP
      description = "SSH access"
    }
  }

  # Allow all outbound traffic
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound traffic"
  }

  tags = {
    Name    = "immudb-sg"
    Project = "lambda-gopher-benchmark"
  }
}

# EC2 Key Pair - only created if an SSH public key is provided
resource "aws_key_pair" "immudb_key_pair" {
  count      = var.ssh_public_key != "" ? 1 : 0
  key_name   = "immudb-key-pair-${var.environment}"
  public_key = var.ssh_public_key

  tags = {
    Name    = "immudb-key-pair-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

# User data script to install and configure ImmuDB
locals {
  user_data = <<-EOF
#!/bin/bash
set -e

# Update and install dependencies
apt-get update
apt-get install -y wget gnupg2 apt-transport-https ca-certificates

# Install ImmuDB
wget https://github.com/codenotary/immudb/releases/download/v1.9.5/immudb-v1.9.5-linux-amd64-installer.bin
chmod +x immudb-v1.9.5-linux-amd64-installer.bin
./immudb-v1.9.5-linux-amd64-installer.bin --non-interactive

# Create benchmark database
sleep 10 # Wait for ImmuDB to start
immuadmin login immudb
immuclient database create benchmark

# Configure ImmuDB service to allow connections from anywhere
cat > /etc/immudb/immudb.service << 'EOL'
[Unit]
Description=ImmuDB is a lightweight, high-speed immutable database
Documentation=https://docs.immudb.io/
After=network.target

[Service]
User=immudb
Group=immudb
ExecStart=/usr/local/bin/immudb --auth --port=3322 --address=0.0.0.0 --dir=/var/lib/immudb --metrics-port=9497 --pgsql-server=true --pgsql-server-port=5432 --pidfile=/var/lib/immudb/immudb.pid
LimitNOFILE=10000
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOL

# Restart the service to apply new settings
systemctl daemon-reload
systemctl restart immudb

echo "ImmuDB setup completed!"
EOF
}

# EC2 Instance for ImmuDB
resource "aws_instance" "immudb_instance" {
  ami                    = var.ec2_ami_id # Amazon Linux 2 AMI, defined in variables.tf
  instance_type          = "t2.micro"     # Free tier eligible
  key_name               = var.ssh_public_key != "" ? aws_key_pair.immudb_key_pair[0].key_name : null
  vpc_security_group_ids = [aws_security_group.immudb_sg.id]
  subnet_id              = aws_subnet.immudb_subnet.id
  user_data              = local.user_data

  root_block_device {
    volume_size = 20 # 20GB for ImmuDB data
    volume_type = "gp2"
  }

  tags = {
    Name    = "immudb-server"
    Project = "lambda-gopher-benchmark"
  }
}

# DynamoDB Table
resource "aws_dynamodb_table" "transactions_table" {
  name           = "Transactions"
  billing_mode   = "PROVISIONED"
  read_capacity  = var.dynamodb_read_capacity
  write_capacity = var.dynamodb_write_capacity
  hash_key       = "accountId"
  range_key      = "uuid"

  attribute {
    name = "accountId"
    type = "S"
  }

  attribute {
    name = "uuid"
    type = "S"
  }

  attribute {
    name = "timestamp"
    type = "S"
  }

  global_secondary_index {
    name               = "TimestampIndex"
    hash_key           = "accountId"
    range_key          = "timestamp"
    projection_type    = "ALL"
    read_capacity      = var.dynamodb_read_capacity
    write_capacity     = var.dynamodb_write_capacity
  }

  point_in_time_recovery {
    enabled = true
  }

  tags = {
    Project = "lambda-gopher-benchmark"
  }
}

# AWS Timestream Database
resource "aws_timestreamwrite_database" "timestream_db" {
  database_name = "TransactionsDB"

  tags = {
    Project = "lambda-gopher-benchmark"
  }
}

# AWS Timestream Table
resource "aws_timestreamwrite_table" "timestream_table" {
  database_name = aws_timestreamwrite_database.timestream_db.database_name
  table_name    = "Transactions"

  retention_properties {
    magnetic_store_retention_period_in_days = 365
    memory_store_retention_period_in_hours  = 24
  }

  tags = {
    Project = "lambda-gopher-benchmark"
  }
}

# S3 bucket for Lambda function code
resource "aws_s3_bucket" "lambda_bucket" {
  bucket = "lambda-gopher-benchmark-${var.environment}-${var.aws_region}"

  tags = {
    Project = "lambda-gopher-benchmark"
  }
}

# Lambda functions for each database and operation type
resource "aws_lambda_function" "benchmark_lambda" {
  for_each = {
    for pair in setproduct(
      local.lambda_function_names,
      var.lambda_memory_configurations
    ) : "${pair[0].key}_${pair[1]}" => {
      config      = pair[0].value
      memory_size = pair[1]
    }
  }

  function_name = "lambda-gopher-benchmark-${split("_", each.key)[0]}-${each.value.memory_size}"
  role          = aws_iam_role.lambda_role.arn
  handler       = "bootstrap"
  runtime       = "provided.al2"
  timeout       = var.lambda_timeout
  memory_size   = each.value.memory_size

  s3_bucket = aws_s3_bucket.lambda_bucket.bucket
  s3_key    = "lambda/lambda-function.zip"

  environment {
    variables = {
      DATABASE_TYPE     = each.value.config.database_type
      OPERATION_TYPE    = each.value.config.operation_type
      FUNCTION_TYPE     = each.value.config.function_type
      DYNAMODB_TABLE    = each.value.config.database_type == "dynamodb" ? aws_dynamodb_table.transactions_table.name : ""
      TIMESTREAM_DATABASE = each.value.config.database_type == "timestream" ? aws_timestreamwrite_database.timestream_db.database_name : ""
      TIMESTREAM_TABLE  = each.value.config.database_type == "timestream" ? aws_timestreamwrite_table.timestream_table.table_name : ""
      IMMUDB_ADDRESS    = each.value.config.database_type == "immudb" ? aws_instance.immudb_instance.public_ip : ""
      IMMUDB_PORT       = "3322"
      IMMUDB_DATABASE   = "benchmark"
      MEMORY_SIZE       = tostring(each.value.memory_size)
    }
  }

  tags = {
    Project     = "lambda-gopher-benchmark"
    Environment = var.environment
    MemorySize  = each.value.memory_size
    Database    = each.value.config.database_type
    Operation   = each.value.config.operation_type
    Function    = each.value.config.function_type
  }
}

# Create Lambda function URLs for invoking the functions
resource "aws_lambda_function_url" "benchmark_lambda_url" {
  for_each = aws_lambda_function.benchmark_lambda

  function_name      = each.value.function_name
  authorization_type = "NONE"  # For benchmark purposes, using NONE, but in production should use AWS_IAM

  cors {
    allow_credentials = true
    allow_origins     = ["*"]
    allow_methods     = ["*"]
    allow_headers     = ["*"]
    expose_headers    = ["*"]
    max_age           = 3600
  }
}

# Output the Lambda function URLs
output "lambda_function_urls" {
  description = "URLs for invoking the Lambda functions"
  value = {
    for key, function_url in aws_lambda_function_url.benchmark_lambda_url : 
    aws_lambda_function.benchmark_lambda[key].function_name => function_url.function_url
  }
}

# Output the Lambda bucket name
output "lambda_bucket_name" {
  description = "Name of the S3 bucket for Lambda function code"
  value       = aws_s3_bucket.lambda_bucket.bucket
}

# Output the Lambda function names
output "lambda_function_names" {
  description = "Names of the created Lambda functions"
  value = {
    for key, function in aws_lambda_function.benchmark_lambda : 
    key => function.function_name
  }
}

# Output the ImmuDB instance details
output "immudb_instance_ip" {
  description = "Public IP address of the ImmuDB EC2 instance"
  value       = aws_instance.immudb_instance.public_ip
}

output "immudb_connection_string" {
  description = "Connection string for ImmuDB"
  value       = "immudb://${aws_instance.immudb_instance.public_ip}:3322"
}

# CloudWatch Dashboard for metrics visualization
resource "aws_cloudwatch_dashboard" "benchmark_dashboard" {
  dashboard_name = "lambda-gopher-benchmark"

  dashboard_body = jsonencode({
    widgets = [
      # Dynamic widgets would be generated here based on benchmark configurations
      # This is a placeholder for illustration
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          metrics = [
            ["AWS/Lambda", "Duration", "FunctionName", "lambda-gopher-benchmark-dynamodb-read-sequential"]
          ]
          view    = "timeSeries"
          stacked = false
          region  = var.aws_region
          title   = "Lambda Duration - DynamoDB Read Sequential"
          period  = 60
        }
      }
    ]
  })
}

# CloudWatch Alarms for benchmark monitoring
resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  for_each = local.lambda_function_names

  alarm_name          = "lambda-errors-${each.key}"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = "60"
  statistic           = "Sum"
  threshold           = "0"
  alarm_description   = "Monitor Lambda function errors for benchmark tests"
  alarm_actions       = []

  dimensions = {
    FunctionName = "lambda-gopher-benchmark-${each.key}"
  }

  tags = {
    Project = "lambda-gopher-benchmark"
  }
} 