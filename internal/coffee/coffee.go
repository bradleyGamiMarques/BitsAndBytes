package coffee

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var ErrProductNotFound = errors.New("coffee: product not found")

type Product struct {
	Category     string `dynamodbav:"category"`
	ProductId    string `dynamodbav:"product_id"`
	Name         string `dynamodbav:"name"`
	PriceInCents int64  `dynamodbav:"price_in_cents"`
	Description  string `dynamodbav:"description"`
	Size         string `dynamodbav:"size"`
}

type Repo struct {
	db    *dynamodb.Client
	table string
}

func NewRepo(db *dynamodb.Client, table string) *Repo {
	return &Repo{db: db, table: table}
}

func (r *Repo) GetProduct(ctx context.Context, category, productId string) (Product, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			"category":   &types.AttributeValueMemberS{Value: category},
			"product_id": &types.AttributeValueMemberS{Value: productId},
		},
	})
	if err != nil {
		return Product{}, fmt.Errorf("getting product: %w", err)
	}
	if out.Item == nil {
		return Product{}, ErrProductNotFound
	}
	var p Product
	if err := attributevalue.UnmarshalMap(out.Item, &p); err != nil {
		return Product{}, fmt.Errorf("unmarshalling product: %w", err)
	}
	return p, nil
}
