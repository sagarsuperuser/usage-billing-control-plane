# AWS Terraform Stack

Production baseline for `usage-billing-control-plane` on AWS.

## Provisions

- VPC across 3 AZs
- EKS cluster + managed node group
- RDS Postgres (Multi-AZ)
- S3 audit exports bucket
- ECR repositories for API and web images
- IRSA role for API service account to access S3 bucket
- IRSA role for external-secrets operator to read runtime secret
- AWS Secrets Manager runtime secret bootstrap

## Usage

Prepare env files once:

```bash
cd infra/terraform/aws
cp environments/staging.tfvars.example environments/staging.tfvars
cp environments/prod.tfvars.example environments/prod.tfvars
cp backends/staging.hcl.example backends/staging.hcl
cp backends/prod.hcl.example backends/prod.hcl
```

Run environment plan/apply from repo root:

```bash
ENVIRONMENT=staging ./scripts/terraform_plan.sh
ENVIRONMENT=staging ./scripts/terraform_apply.sh

ENVIRONMENT=prod ./scripts/terraform_plan.sh
ENVIRONMENT=prod ./scripts/terraform_apply.sh
```

Or use Make targets:

```bash
make tf-plan-staging
make tf-apply-staging
make tf-plan-prod
make tf-apply-prod
```

Legacy direct Terraform flow:

```bash
cd infra/terraform/aws
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform fmt -recursive
terraform validate
terraform plan -out=tfplan
terraform apply tfplan
```

## Notes

- Configure a remote state backend (S3 + DynamoDB lock table) before shared-team usage.
- Do not commit real `terraform.tfvars` files containing credentials.
- Never commit `environments/*.tfvars` or `backends/*.hcl` with real secrets/account identifiers.
- API IRSA subject is pinned to `system:serviceaccount:lago-alpha:lago-alpha-api`.
- External secrets IRSA subject is configurable with `external_secrets_namespace` and `external_secrets_service_account`.
