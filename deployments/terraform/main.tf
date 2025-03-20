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
      }
    ]
  })
}

# Attach the policy to the role
resource "aws_iam_role_policy_attachment" "lambda_policy_attachment" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.lambda_policy.arn
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
      { for config_key, config in local.lambda_function_names : config_key => config if config.database_type != "immudb" },
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