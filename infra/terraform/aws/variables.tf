variable "project_name" {
  description = "Logical project/app name."
  type        = string
  default     = "lago-alpha"
}

variable "environment" {
  description = "Environment name, for example staging or prod."
  type        = string
  validation {
    condition     = contains(["staging", "prod"], var.environment)
    error_message = "environment must be one of: staging, prod."
  }
}

variable "aws_region" {
  description = "AWS region."
  type        = string
}

variable "aws_account_id" {
  description = "Optional AWS account id override. Leave empty to resolve dynamically via aws_caller_identity."
  type        = string
  default     = ""
  validation {
    condition     = trimspace(var.aws_account_id) == "" || can(regex("^[0-9]{12}$", var.aws_account_id))
    error_message = "aws_account_id must be empty or a 12-digit AWS account id."
  }
}

variable "vpc_cidr" {
  description = "VPC CIDR block."
  type        = string
  default     = "10.40.0.0/16"
}

variable "azs" {
  description = "Availability zones."
  type        = list(string)
}

variable "private_subnets" {
  description = "Private subnets for EKS and databases."
  type        = list(string)
}

variable "public_subnets" {
  description = "Public subnets for internet-facing load balancers."
  type        = list(string)
}

variable "eks_version" {
  description = "EKS Kubernetes version."
  type        = string
  default     = "1.30"
}

variable "eks_node_instance_types" {
  description = "Instance types for EKS managed node groups."
  type        = list(string)
  default     = ["m6i.large"]
}

variable "eks_node_desired_size" {
  description = "Desired number of nodes."
  type        = number
  default     = 3
}

variable "eks_node_min_size" {
  description = "Minimum number of nodes."
  type        = number
  default     = 3
}

variable "eks_node_max_size" {
  description = "Maximum number of nodes."
  type        = number
  default     = 12
}

variable "db_name" {
  description = "RDS database name."
  type        = string
  default     = "lago_alpha"
}

variable "db_username" {
  description = "RDS master username."
  type        = string
}

variable "db_password" {
  description = "RDS master password."
  type        = string
  sensitive   = true
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.r6g.large"
}

variable "db_allocated_storage" {
  description = "RDS allocated storage (GB)."
  type        = number
  default     = 100
}

variable "db_max_allocated_storage" {
  description = "RDS autoscaling max storage (GB)."
  type        = number
  default     = 1000
}

variable "db_multi_az" {
  description = "Whether to enable Multi-AZ deployment for RDS."
  type        = bool
  default     = true
  validation {
    condition     = var.environment != "prod" || var.db_multi_az
    error_message = "db_multi_az must be true when environment is prod."
  }
}

variable "audit_exports_bucket_force_destroy" {
  description = "Whether audit exports S3 bucket can be force-destroyed."
  type        = bool
  default     = false
  validation {
    condition     = var.environment != "prod" || !var.audit_exports_bucket_force_destroy
    error_message = "audit_exports_bucket_force_destroy must be false when environment is prod."
  }
}

variable "external_secrets_namespace" {
  description = "Namespace where external-secrets operator runs."
  type        = string
  default     = "external-secrets"
}

variable "external_secrets_service_account" {
  description = "Service account name used by external-secrets operator."
  type        = string
  default     = "external-secrets"
}

variable "runtime_secret_name" {
  description = "AWS Secrets Manager secret name for alpha runtime configuration."
  type        = string
  default     = ""
}

variable "create_runtime_secret" {
  description = "Whether to create runtime secrets manager secret."
  type        = bool
  default     = true
}

variable "runtime_secret_recovery_window_in_days" {
  description = "Recovery window for deleting runtime secret."
  type        = number
  default     = 30
}

variable "tags" {
  description = "Additional tags to apply to resources."
  type        = map(string)
  default     = {}
}
