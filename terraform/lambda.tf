resource "null_resource" "lambda_build" {
  triggers = {
    source_hash = sha256(join("", [
      file("${path.module}/../cmd/getproduct/main.go"),
      file("${path.module}/../internal/coffee/coffee.go"),
      file("${path.module}/../internal/api/getproduct.go"),
    ]))
  }

  provisioner "local-exec" {
    command     = "GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o ${path.module}/bootstrap ../cmd/getproduct"
    working_dir = path.module
  }
}

data "archive_file" "lambda_zip" {
  depends_on  = [null_resource.lambda_build]
  type        = "zip"
  source_file = "${path.module}/bootstrap"
  output_path = "${path.module}/getproduct.zip"
}

resource "aws_iam_role" "lambda_exec" {
  name = "${var.project_name}-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "lambda.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy" "lambda_dynamodb" {
  name = "${var.project_name}-dynamodb-read"
  role = aws_iam_role.lambda_exec.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "dynamodb:GetItem",
      ]
      Resource = aws_dynamodb_table.coffee_products.arn
    }]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "get_product" {
  function_name = "${var.project_name}-get-product"
  role          = aws_iam_role.lambda_exec.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]

  filename         = data.archive_file.lambda_zip.output_path
  source_code_hash = data.archive_file.lambda_zip.output_base64sha256

  environment {
    variables = {
      TABLE_NAME = aws_dynamodb_table.coffee_products.name
    }
  }
}
