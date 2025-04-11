variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "us-east-1"
}

variable "ssh_public_key" {
  description = "SSH public key for EC2 instance access (if empty, SSH access will be disabled)"
  type        = string
  default     = ""
}

variable "environment" {
  description = "Environment name for resource naming and tagging"
  type        = string
  default     = "dev"
}

variable "dynamodb_read_capacity" {
  description = "Provisioned read capacity units for DynamoDB"
  type        = number
  default     = 20
}

variable "dynamodb_write_capacity" {
  description = "Provisioned write capacity units for DynamoDB"
  type        = number
  default     = 20
}

variable "lambda_timeout" {
  description = "Timeout in seconds for Lambda functions"
  type        = number
  default     = 30
}

variable "lambda_memory_configurations" {
  description = "List of memory configurations to test with Lambda functions"
  type        = list(number)
  default     = [128, 512, 1024, 2048, 3008]
}

variable "vpc_enabled" {
  description = "Whether to deploy Lambda functions in a VPC"
  type        = bool
  default     = false
}

variable "enable_api_gateway" {
  description = "Whether to deploy API Gateway for Lambda invocation"
  type        = bool
  default     = true
}

variable "enable_cloudwatch_dashboard" {
  description = "Whether to create CloudWatch dashboard"
  type        = bool
  default     = true
}

variable "enable_alarm_notifications" {
  description = "Whether to enable CloudWatch alarm notifications"
  type        = bool
  default     = false
}

variable "notification_email" {
  description = "Email address for CloudWatch alarm notifications"
  type        = string
  default     = ""
}

variable "enable_x_ray_tracing" {
  description = "Whether to enable AWS X-Ray tracing for Lambda functions"
  type        = bool
  default     = true
}

variable "dynamodb_billing_mode" {
  description = "Billing mode for DynamoDB (PROVISIONED or PAY_PER_REQUEST)"
  type        = string
  default     = "PROVISIONED"
  validation {
    condition     = contains(["PROVISIONED", "PAY_PER_REQUEST"], var.dynamodb_billing_mode)
    error_message = "DynamoDB billing mode must be either PROVISIONED or PAY_PER_REQUEST."
  }
}

variable "dynamodb_point_in_time_recovery" {
  description = "Whether to enable point-in-time recovery for DynamoDB"
  type        = bool
  default     = true
}

variable "timestream_magnetic_retention_days" {
  description = "Number of days to retain data in Timestream magnetic store"
  type        = number
  default     = 365
}

variable "timestream_memory_retention_hours" {
  description = "Number of hours to retain data in Timestream memory store"
  type        = number
  default     = 24
}

variable "test_duration_seconds" {
  description = "Duration of benchmark tests in seconds"
  type        = number
  default     = 300 # 5 minutes
}

variable "test_concurrency_levels" {
  description = "List of concurrency levels for testing"
  type        = list(number)
  default     = [1, 10, 50, 100, 250, 500, 1000]
}

variable "test_data_sizes" {
  description = "List of data sizes for testing in bytes"
  type        = list(number)
  default     = [256, 1024, 10240, 102400, 1048576] # 256B, 1KB, 10KB, 100KB, 1MB
}

variable "regional_testing" {
  description = "Whether to deploy and test across multiple AWS regions"
  type        = bool
  default     = false
}

variable "testing_regions" {
  description = "List of AWS regions to deploy for regional testing"
  type        = list(string)
  default     = ["us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"]
}

variable "enable_cost_optimization" {
  description = "Whether to enable cost optimization features"
  type        = bool
  default     = true
}

variable "cost_optimization_threshold_percent" {
  description = "Threshold percentage for cost optimization alerts"
  type        = number
  default     = 80
}

variable "enable_vpc_endpoints" {
  description = "Whether to create VPC endpoints for AWS services"
  type        = bool
  default     = false
}

variable "nat_gateway_enabled" {
  description = "Whether to deploy NAT Gateway for VPC"
  type        = bool
  default     = false
}

variable "lambda_container_images" {
  description = "Whether to use container images for Lambda functions"
  type        = bool
  default     = false
}

variable "ec2_ami_id" {
  description = "AMI ID for the ImmuDB EC2 instance"
  type        = string
  default     = "ami-0f34c5ae932e6f0e4" # Amazon Linux 2 AMI in us-east-1, update for other regions
} 