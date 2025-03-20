resource "aws_vpc" "benchmark_vpc" {
  count = var.vpc_enabled ? 1 : 0

  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name    = "lambda-gopher-benchmark-vpc-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

resource "aws_subnet" "private_subnets" {
  count = var.vpc_enabled ? length(var.private_subnet_cidrs) : 0

  vpc_id                  = aws_vpc.benchmark_vpc[0].id
  cidr_block              = var.private_subnet_cidrs[count.index]
  availability_zone       = var.availability_zones[count.index % length(var.availability_zones)]
  map_public_ip_on_launch = false

  tags = {
    Name    = "lambda-gopher-benchmark-private-subnet-${count.index}-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

resource "aws_subnet" "public_subnets" {
  count = var.vpc_enabled && var.nat_gateway_enabled ? length(var.public_subnet_cidrs) : 0

  vpc_id                  = aws_vpc.benchmark_vpc[0].id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = var.availability_zones[count.index % length(var.availability_zones)]
  map_public_ip_on_launch = true

  tags = {
    Name    = "lambda-gopher-benchmark-public-subnet-${count.index}-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

resource "aws_internet_gateway" "igw" {
  count = var.vpc_enabled && var.nat_gateway_enabled ? 1 : 0

  vpc_id = aws_vpc.benchmark_vpc[0].id

  tags = {
    Name    = "lambda-gopher-benchmark-igw-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

resource "aws_eip" "nat" {
  count = var.vpc_enabled && var.nat_gateway_enabled ? length(var.public_subnet_cidrs) : 0

  domain = "vpc"

  tags = {
    Name    = "lambda-gopher-benchmark-eip-${count.index}-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

resource "aws_nat_gateway" "nat" {
  count = var.vpc_enabled && var.nat_gateway_enabled ? length(var.public_subnet_cidrs) : 0

  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public_subnets[count.index].id

  tags = {
    Name    = "lambda-gopher-benchmark-nat-${count.index}-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }

  depends_on = [aws_internet_gateway.igw]
}

resource "aws_route_table" "private" {
  count = var.vpc_enabled ? 1 : 0

  vpc_id = aws_vpc.benchmark_vpc[0].id

  tags = {
    Name    = "lambda-gopher-benchmark-private-rt-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

resource "aws_route_table" "public" {
  count = var.vpc_enabled && var.nat_gateway_enabled ? 1 : 0

  vpc_id = aws_vpc.benchmark_vpc[0].id

  tags = {
    Name    = "lambda-gopher-benchmark-public-rt-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

resource "aws_route" "public_internet_gateway" {
  count = var.vpc_enabled && var.nat_gateway_enabled ? 1 : 0

  route_table_id         = aws_route_table.public[0].id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.igw[0].id
}

resource "aws_route" "private_nat_gateway" {
  count = var.vpc_enabled && var.nat_gateway_enabled ? length(var.private_subnet_cidrs) : 0

  route_table_id         = aws_route_table.private[0].id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.nat[count.index % length(var.public_subnet_cidrs)].id
}

resource "aws_route_table_association" "private" {
  count = var.vpc_enabled ? length(var.private_subnet_cidrs) : 0

  subnet_id      = aws_subnet.private_subnets[count.index].id
  route_table_id = aws_route_table.private[0].id
}

resource "aws_route_table_association" "public" {
  count = var.vpc_enabled && var.nat_gateway_enabled ? length(var.public_subnet_cidrs) : 0

  subnet_id      = aws_subnet.public_subnets[count.index].id
  route_table_id = aws_route_table.public[0].id
}

# VPC Endpoints for AWS services

resource "aws_security_group" "vpc_endpoints" {
  count = var.vpc_enabled && var.enable_vpc_endpoints ? 1 : 0

  name        = "lambda-gopher-benchmark-vpc-endpoints-sg-${var.environment}"
  description = "Security group for VPC endpoints"
  vpc_id      = aws_vpc.benchmark_vpc[0].id

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }

  tags = {
    Name    = "lambda-gopher-benchmark-vpc-endpoints-sg-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

# DynamoDB VPC Endpoint
resource "aws_vpc_endpoint" "dynamodb" {
  count = var.vpc_enabled && var.enable_vpc_endpoints ? 1 : 0

  vpc_id            = aws_vpc.benchmark_vpc[0].id
  service_name      = "com.amazonaws.${var.aws_region}.dynamodb"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.private[0].id]

  tags = {
    Name    = "lambda-gopher-benchmark-dynamodb-vpce-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

# S3 VPC Endpoint
resource "aws_vpc_endpoint" "s3" {
  count = var.vpc_enabled && var.enable_vpc_endpoints ? 1 : 0

  vpc_id            = aws_vpc.benchmark_vpc[0].id
  service_name      = "com.amazonaws.${var.aws_region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = [aws_route_table.private[0].id]

  tags = {
    Name    = "lambda-gopher-benchmark-s3-vpce-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

# Timestream VPC Endpoint
resource "aws_vpc_endpoint" "timestream" {
  count = var.vpc_enabled && var.enable_vpc_endpoints ? 1 : 0

  vpc_id              = aws_vpc.benchmark_vpc[0].id
  service_name        = "com.amazonaws.${var.aws_region}.timestream"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = aws_subnet.private_subnets[*].id
  security_group_ids  = [aws_security_group.vpc_endpoints[0].id]
  private_dns_enabled = true

  tags = {
    Name    = "lambda-gopher-benchmark-timestream-vpce-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

# Lambda VPC Endpoint
resource "aws_vpc_endpoint" "lambda" {
  count = var.vpc_enabled && var.enable_vpc_endpoints ? 1 : 0

  vpc_id              = aws_vpc.benchmark_vpc[0].id
  service_name        = "com.amazonaws.${var.aws_region}.lambda"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = aws_subnet.private_subnets[*].id
  security_group_ids  = [aws_security_group.vpc_endpoints[0].id]
  private_dns_enabled = true

  tags = {
    Name    = "lambda-gopher-benchmark-lambda-vpce-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
}

# CloudWatch VPC Endpoint
resource "aws_vpc_endpoint" "logs" {
  count = var.vpc_enabled && var.enable_vpc_endpoints ? 1 : 0

  vpc_id              = aws_vpc.benchmark_vpc[0].id
  service_name        = "com.amazonaws.${var.aws_region}.logs"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = aws_subnet.private_subnets[*].id
  security_group_ids  = [aws_security_group.vpc_endpoints[0].id]
  private_dns_enabled = true

  tags = {
    Name    = "lambda-gopher-benchmark-logs-vpce-${var.environment}"
    Project = "lambda-gopher-benchmark"
  }
} 