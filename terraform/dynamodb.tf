resource "aws_dynamodb_table" "coffee_products" {
  name         = "${var.project_name}-products"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "category"
  range_key    = "product_id"

  attribute {
    name = "category"
    type = "S"
  }

  attribute {
    name = "product_id"
    type = "S"
  }
}
