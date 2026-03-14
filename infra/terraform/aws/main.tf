locals {
  name_prefix = "${var.project_name}-${var.environment}"

  runtime_secret_name = trimspace(var.runtime_secret_name) != "" ? var.runtime_secret_name : "${local.name_prefix}/runtime"
  aws_account_id      = trimspace(var.aws_account_id) != "" ? trimspace(var.aws_account_id) : data.aws_caller_identity.current[0].account_id
  cluster_name        = "${local.name_prefix}-eks"
  cluster_arn         = "arn:aws:eks:${var.aws_region}:${local.aws_account_id}:cluster/${local.cluster_name}"
  eks_admin_role_name = trimspace(var.eks_admin_role_name) != "" ? trimspace(var.eks_admin_role_name) : "${local.name_prefix}-eks-admin"
}

data "aws_caller_identity" "current" {
  count = trimspace(var.aws_account_id) == "" ? 1 : 0
}

data "aws_iam_policy_document" "eks_admin_assume_role" {
  count = length(var.eks_admin_role_trusted_principal_arns) > 0 ? 1 : 0

  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "AWS"
      identifiers = var.eks_admin_role_trusted_principal_arns
    }
  }
}

resource "aws_iam_role" "eks_admin" {
  count = length(var.eks_admin_role_trusted_principal_arns) > 0 ? 1 : 0

  name               = local.eks_admin_role_name
  assume_role_policy = data.aws_iam_policy_document.eks_admin_assume_role[0].json
}

data "aws_iam_policy_document" "eks_admin" {
  count = length(var.eks_admin_role_trusted_principal_arns) > 0 ? 1 : 0

  statement {
    effect = "Allow"
    actions = [
      "eks:DescribeCluster",
    ]
    resources = [local.cluster_arn]
  }

  statement {
    effect = "Allow"
    actions = [
      "eks:ListClusters",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "eks_admin" {
  count = length(var.eks_admin_role_trusted_principal_arns) > 0 ? 1 : 0

  name   = "${local.name_prefix}-eks-admin"
  role   = aws_iam_role.eks_admin[0].id
  policy = data.aws_iam_policy_document.eks_admin[0].json
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.21"

  name = "${local.name_prefix}-vpc"
  cidr = var.vpc_cidr

  azs             = var.azs
  private_subnets = var.private_subnets
  public_subnets  = var.public_subnets

  enable_nat_gateway     = true
  single_nat_gateway     = false
  one_nat_gateway_per_az = true

  enable_dns_hostnames = true
  enable_dns_support   = true

  public_subnet_tags = {
    "kubernetes.io/role/elb" = "1"
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = "1"
  }
}

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.36"

  cluster_name    = local.cluster_name
  cluster_version = var.eks_version

  cluster_endpoint_public_access           = true
  authentication_mode                      = "API"
  enable_cluster_creator_admin_permissions = false
  enable_irsa                              = true

  access_entries = merge(
    {
      for principal_arn in var.eks_cluster_admin_principal_arns :
      replace(replace(replace(principal_arn, ":", "-"), "/", "-"), "@", "-") => {
        principal_arn = principal_arn

        policy_associations = {
          cluster_admin = {
            policy_arn = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
            access_scope = {
              type = "cluster"
            }
          }
        }
      }
    },
    length(var.eks_admin_role_trusted_principal_arns) > 0 ? {
      eks_admin_role = {
        principal_arn = aws_iam_role.eks_admin[0].arn

        policy_associations = {
          cluster_admin = {
            policy_arn = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
            access_scope = {
              type = "cluster"
            }
          }
        }
      }
    } : {},
  )

  vpc_id                   = module.vpc.vpc_id
  subnet_ids               = module.vpc.private_subnets
  control_plane_subnet_ids = module.vpc.private_subnets

  node_security_group_additional_rules = {
    ingress_self_all = {
      description = "Node-to-node all traffic for pod networking"
      protocol    = "-1"
      from_port   = 0
      to_port     = 0
      type        = "ingress"
      self        = true
    }
  }

  eks_managed_node_groups = {
    app = {
      name                     = "${local.name_prefix}-ng-app"
      iam_role_name            = "${local.name_prefix}-ng-app-role"
      iam_role_use_name_prefix = false
      instance_types           = var.eks_node_instance_types
      min_size                 = var.eks_node_min_size
      max_size                 = var.eks_node_max_size
      desired_size             = var.eks_node_desired_size

      labels = {
        workload = "general"
      }
    }
  }
}

resource "aws_security_group" "rds" {
  name        = "${local.name_prefix}-rds-sg"
  description = "RDS access from EKS nodes"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "Postgres from EKS nodes"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [module.eks.node_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "rate_limit_cache" {
  name        = "${local.name_prefix}-rate-limit-cache-sg"
  description = "Rate-limit cache access from EKS nodes"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "Valkey/Redis from EKS nodes"
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [module.eks.node_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_elasticache_serverless_cache" "rate_limit" {
  name        = "${local.name_prefix}-rate-limit"
  description = var.rate_limit_cache_description
  engine      = var.rate_limit_cache_engine

  major_engine_version = trimspace(var.rate_limit_cache_major_engine_version) != "" ? var.rate_limit_cache_major_engine_version : null

  security_group_ids = [aws_security_group.rate_limit_cache.id]
  subnet_ids         = module.vpc.private_subnets
}

module "rds" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 6.13"

  identifier = "${replace(local.name_prefix, "-", "")}db"

  engine               = "postgres"
  engine_version       = "16.6"
  family               = "postgres16"
  major_engine_version = "16"
  instance_class       = var.db_instance_class

  allocated_storage     = var.db_allocated_storage
  max_allocated_storage = var.db_max_allocated_storage
  storage_encrypted     = true

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password
  port     = 5432

  multi_az                = var.db_multi_az
  backup_retention_period = 14
  backup_window           = "03:00-04:00"
  maintenance_window      = "Mon:04:00-Mon:05:00"

  subnet_ids             = module.vpc.private_subnets
  create_db_subnet_group = true
  vpc_security_group_ids = [aws_security_group.rds.id]

  deletion_protection              = true
  skip_final_snapshot              = false
  final_snapshot_identifier_prefix = "${replace(local.name_prefix, "-", "")}-final"
}

resource "aws_ecr_repository" "api" {
  name                 = "${local.name_prefix}/api"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "web" {
  name                 = "${local.name_prefix}/web"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_s3_bucket" "audit_exports" {
  bucket        = "${local.name_prefix}-audit-exports-${local.aws_account_id}"
  force_destroy = var.audit_exports_bucket_force_destroy
}

resource "aws_s3_bucket_versioning" "audit_exports" {
  bucket = aws_s3_bucket.audit_exports.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "audit_exports" {
  bucket = aws_s3_bucket.audit_exports.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "audit_exports" {
  bucket                  = aws_s3_bucket.audit_exports.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_secretsmanager_secret" "runtime" {
  count = var.create_runtime_secret ? 1 : 0

  name                    = local.runtime_secret_name
  recovery_window_in_days = var.runtime_secret_recovery_window_in_days
}

resource "aws_secretsmanager_secret_version" "runtime_bootstrap" {
  count = var.create_runtime_secret ? 1 : 0

  secret_id     = aws_secretsmanager_secret.runtime[0].id
  secret_string = "{}"
}

data "aws_secretsmanager_secret" "runtime" {
  count = var.create_runtime_secret ? 0 : 1

  name = local.runtime_secret_name
}

data "aws_iam_policy_document" "api_assume_role" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${replace(module.eks.oidc_provider, "https://", "")}:sub"
      values   = ["system:serviceaccount:lago-alpha:lago-alpha-api"]
    }
  }
}

resource "aws_iam_role" "api_irsa" {
  name               = "${local.name_prefix}-api-irsa"
  assume_role_policy = data.aws_iam_policy_document.api_assume_role.json
}

data "aws_iam_policy_document" "api_s3_policy" {
  statement {
    effect = "Allow"
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:AbortMultipartUpload",
      "s3:ListBucket",
    ]
    resources = [
      aws_s3_bucket.audit_exports.arn,
      "${aws_s3_bucket.audit_exports.arn}/*",
    ]
  }
}

resource "aws_iam_policy" "api_s3" {
  name   = "${local.name_prefix}-api-s3"
  policy = data.aws_iam_policy_document.api_s3_policy.json
}

resource "aws_iam_role_policy_attachment" "api_s3" {
  role       = aws_iam_role.api_irsa.name
  policy_arn = aws_iam_policy.api_s3.arn
}

data "aws_s3_bucket" "lago_uploads" {
  count  = trimspace(var.lago_uploads_bucket_name) == "" ? 0 : 1
  bucket = var.lago_uploads_bucket_name
}

data "aws_iam_policy_document" "lago_assume_role" {
  count = trimspace(var.lago_uploads_bucket_name) == "" ? 0 : 1

  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${replace(module.eks.oidc_provider, "https://", "")}:sub"
      values   = ["system:serviceaccount:${var.lago_service_account_namespace}:${var.lago_service_account_name}"]
    }
  }
}

resource "aws_iam_role" "lago_irsa" {
  count              = trimspace(var.lago_uploads_bucket_name) == "" ? 0 : 1
  name               = "${local.name_prefix}-lago-irsa"
  assume_role_policy = data.aws_iam_policy_document.lago_assume_role[0].json
}

data "aws_iam_policy_document" "lago_s3_policy" {
  count = trimspace(var.lago_uploads_bucket_name) == "" ? 0 : 1

  statement {
    effect = "Allow"
    actions = [
      "s3:GetBucketLocation",
      "s3:ListBucket",
      "s3:ListBucketMultipartUploads",
    ]
    resources = [
      data.aws_s3_bucket.lago_uploads[0].arn,
    ]
  }

  statement {
    effect = "Allow"
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
      "s3:AbortMultipartUpload",
      "s3:ListMultipartUploadParts",
    ]
    resources = [
      "${data.aws_s3_bucket.lago_uploads[0].arn}/*",
    ]
  }
}

resource "aws_iam_policy" "lago_s3" {
  count  = trimspace(var.lago_uploads_bucket_name) == "" ? 0 : 1
  name   = "${local.name_prefix}-lago-s3"
  policy = data.aws_iam_policy_document.lago_s3_policy[0].json
}

resource "aws_iam_role_policy_attachment" "lago_s3" {
  count      = trimspace(var.lago_uploads_bucket_name) == "" ? 0 : 1
  role       = aws_iam_role.lago_irsa[0].name
  policy_arn = aws_iam_policy.lago_s3[0].arn
}

locals {
  runtime_secret_arn = var.create_runtime_secret ? aws_secretsmanager_secret.runtime[0].arn : data.aws_secretsmanager_secret.runtime[0].arn
  external_secrets_allowed_secret_arns = [
    local.runtime_secret_arn,
    "arn:aws:secretsmanager:${var.aws_region}:${local.aws_account_id}:secret:lago/staging/*",
    "arn:aws:secretsmanager:${var.aws_region}:${local.aws_account_id}:secret:rds!db-*",
  ]
}

data "aws_iam_policy_document" "external_secrets_assume_role" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${replace(module.eks.oidc_provider, "https://", "")}:sub"
      values   = ["system:serviceaccount:${var.external_secrets_namespace}:${var.external_secrets_service_account}"]
    }
  }
}

resource "aws_iam_role" "external_secrets_irsa" {
  name               = "${local.name_prefix}-external-secrets-irsa"
  assume_role_policy = data.aws_iam_policy_document.external_secrets_assume_role.json
}

data "aws_iam_policy_document" "external_secrets_runtime_secret_policy" {
  statement {
    effect = "Allow"
    actions = [
      "secretsmanager:DescribeSecret",
      "secretsmanager:GetSecretValue",
    ]
    resources = local.external_secrets_allowed_secret_arns
  }
}

resource "aws_iam_policy" "external_secrets_runtime_secret" {
  name   = "${local.name_prefix}-external-secrets-runtime-secret"
  policy = data.aws_iam_policy_document.external_secrets_runtime_secret_policy.json
}

resource "aws_iam_role_policy_attachment" "external_secrets_runtime_secret" {
  role       = aws_iam_role.external_secrets_irsa.name
  policy_arn = aws_iam_policy.external_secrets_runtime_secret.arn
}

data "aws_iam_policy_document" "ebs_csi_assume_role" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    principals {
      type        = "Federated"
      identifiers = [module.eks.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${replace(module.eks.oidc_provider, "https://", "")}:sub"
      values   = ["system:serviceaccount:kube-system:ebs-csi-controller-sa"]
    }
  }
}

resource "aws_iam_role" "ebs_csi_irsa" {
  name               = "${local.name_prefix}-ebs-csi-irsa"
  assume_role_policy = data.aws_iam_policy_document.ebs_csi_assume_role.json
}

resource "aws_iam_role_policy_attachment" "ebs_csi_driver" {
  role       = aws_iam_role.ebs_csi_irsa.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonEBSCSIDriverPolicy"
}

resource "aws_eks_addon" "ebs_csi" {
  cluster_name                = module.eks.cluster_name
  addon_name                  = "aws-ebs-csi-driver"
  service_account_role_arn    = aws_iam_role.ebs_csi_irsa.arn
  resolve_conflicts_on_create = "OVERWRITE"
  resolve_conflicts_on_update = "OVERWRITE"

  depends_on = [aws_iam_role_policy_attachment.ebs_csi_driver]
}
