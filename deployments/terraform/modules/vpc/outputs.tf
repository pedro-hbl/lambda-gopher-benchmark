output "vpc_id" {
  description = "The ID of the VPC"
  value       = var.vpc_enabled ? aws_vpc.benchmark_vpc[0].id : null
}

output "private_subnet_ids" {
  description = "List of private subnet IDs"
  value       = var.vpc_enabled ? aws_subnet.private_subnets[*].id : []
}

output "public_subnet_ids" {
  description = "List of public subnet IDs"
  value       = var.vpc_enabled && var.nat_gateway_enabled ? aws_subnet.public_subnets[*].id : []
}

output "vpc_cidr_block" {
  description = "The CIDR block of the VPC"
  value       = var.vpc_enabled ? aws_vpc.benchmark_vpc[0].cidr_block : null
}

output "dynamodb_vpc_endpoint_id" {
  description = "The ID of the DynamoDB VPC endpoint"
  value       = var.vpc_enabled && var.enable_vpc_endpoints ? aws_vpc_endpoint.dynamodb[0].id : null
}

output "s3_vpc_endpoint_id" {
  description = "The ID of the S3 VPC endpoint"
  value       = var.vpc_enabled && var.enable_vpc_endpoints ? aws_vpc_endpoint.s3[0].id : null
}

output "lambda_vpc_endpoint_id" {
  description = "The ID of the Lambda VPC endpoint"
  value       = var.vpc_enabled && var.enable_vpc_endpoints ? aws_vpc_endpoint.lambda[0].id : null
}

output "timestream_vpc_endpoint_id" {
  description = "The ID of the Timestream VPC endpoint"
  value       = var.vpc_enabled && var.enable_vpc_endpoints ? aws_vpc_endpoint.timestream[0].id : null
}

output "cloudwatch_logs_vpc_endpoint_id" {
  description = "The ID of the CloudWatch Logs VPC endpoint"
  value       = var.vpc_enabled && var.enable_vpc_endpoints ? aws_vpc_endpoint.logs[0].id : null
}

output "vpc_endpoint_security_group_id" {
  description = "The ID of the security group for VPC endpoints"
  value       = var.vpc_enabled && var.enable_vpc_endpoints ? aws_security_group.vpc_endpoints[0].id : null
} 