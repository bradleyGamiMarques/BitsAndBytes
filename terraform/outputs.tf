output "api_url" {
  value       = aws_apigatewayv2_stage.default.invoke_url
  description = "Base URL for the HTTP API. Append /products/{category}/{productId} to call the Lambda."
}

output "table_name" {
  value       = aws_dynamodb_table.coffee_products.name
  description = "Name of the DynamoDB table."
}

output "lambda_function_name" {
  value       = aws_lambda_function.get_product.function_name
  description = "Name of the Lambda function."
}
