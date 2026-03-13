output "vpc_id" {
  value       = module.vpc.vpc_id
  description = "VPC ID."
}

output "private_subnet_ids" {
  value       = module.vpc.private_subnets
  description = "Private subnets used by workloads."
}

output "eks_cluster_name" {
  value       = module.eks.cluster_name
  description = "EKS cluster name."
}

output "eks_cluster_endpoint" {
  value       = module.eks.cluster_endpoint
  description = "EKS API endpoint."
}

output "eks_oidc_provider_arn" {
  value       = module.eks.oidc_provider_arn
  description = "EKS OIDC provider ARN for IRSA."
}

output "rds_endpoint" {
  value       = module.rds.db_instance_endpoint
  description = "RDS endpoint."
}

output "audit_exports_bucket" {
  value       = aws_s3_bucket.audit_exports.bucket
  description = "S3 bucket used for audit export artifacts."
}

output "api_ecr_repository_url" {
  value       = aws_ecr_repository.api.repository_url
  description = "ECR repository URL for alpha API image."
}

output "web_ecr_repository_url" {
  value       = aws_ecr_repository.web.repository_url
  description = "ECR repository URL for alpha web image."
}

output "api_irsa_role_arn" {
  value       = aws_iam_role.api_irsa.arn
  description = "IRSA role ARN for API service account."
}

output "external_secrets_irsa_role_arn" {
  value       = aws_iam_role.external_secrets_irsa.arn
  description = "IRSA role ARN for external-secrets operator service account."
}

output "runtime_secret_name" {
  value       = local.runtime_secret_name
  description = "AWS Secrets Manager secret name containing runtime env."
}

output "runtime_secret_arn" {
  value       = local.runtime_secret_arn
  description = "AWS Secrets Manager secret ARN containing runtime env."
}

output "eks_admin_role_arn" {
  value       = try(aws_iam_role.eks_admin[0].arn, null)
  description = "Dedicated IAM role ARN for EKS admin access."
}
