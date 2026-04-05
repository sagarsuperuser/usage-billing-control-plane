# CloudFront CDN for static web assets
# Serves Vite build output with long-lived cache for /assets/ and
# fallback to origin for SPA routing.
#
# Usage: add to infra/terraform/aws/ and reference the web ALB as origin.

# resource "aws_cloudfront_distribution" "web" {
#   enabled             = true
#   default_root_object = "index.html"
#   price_class         = "PriceClass_100" # US + Europe
#   aliases             = [var.web_domain]
#
#   origin {
#     domain_name = var.web_alb_dns_name
#     origin_id   = "web-alb"
#
#     custom_origin_config {
#       http_port              = 80
#       https_port             = 443
#       origin_protocol_policy = "https-only"
#       origin_ssl_protocols   = ["TLSv1.2"]
#     }
#   }
#
#   # Static assets — immutable, cache for 1 year
#   ordered_cache_behavior {
#     path_pattern     = "/assets/*"
#     allowed_methods  = ["GET", "HEAD"]
#     cached_methods   = ["GET", "HEAD"]
#     target_origin_id = "web-alb"
#
#     forwarded_values {
#       query_string = false
#       cookies { forward = "none" }
#     }
#
#     min_ttl     = 31536000
#     default_ttl = 31536000
#     max_ttl     = 31536000
#     compress    = true
#
#     viewer_protocol_policy = "redirect-to-https"
#   }
#
#   # SPA fallback — short cache, forward to origin
#   default_cache_behavior {
#     allowed_methods  = ["GET", "HEAD"]
#     cached_methods   = ["GET", "HEAD"]
#     target_origin_id = "web-alb"
#
#     forwarded_values {
#       query_string = true
#       cookies { forward = "all" }
#     }
#
#     min_ttl     = 0
#     default_ttl = 60
#     max_ttl     = 300
#     compress    = true
#
#     viewer_protocol_policy = "redirect-to-https"
#   }
#
#   # Custom error page for SPA routing (404 → index.html)
#   custom_error_response {
#     error_code         = 404
#     response_code      = 200
#     response_page_path = "/index.html"
#     error_caching_min_ttl = 0
#   }
#
#   viewer_certificate {
#     acm_certificate_arn      = var.acm_certificate_arn
#     ssl_support_method       = "sni-only"
#     minimum_protocol_version = "TLSv1.2_2021"
#   }
#
#   restrictions {
#     geo_restriction { restriction_type = "none" }
#   }
# }
