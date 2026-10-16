// Package menu holds the rules for menu items and how they are stored.
package menu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	ErrInvalidMenuItem       = errors.New("menu: invalid menu item")
	ErrItemMissingId         = errors.New("menu: id is required")
	ErrItemMissingName       = errors.New("menu: item name is required")
	ErrItemNameTooLong       = errors.New("menu: item name too long")
	ErrItemNotFound          = errors.New("menu: item not found")
	ErrItemPriceOutOfRange   = errors.New("menu: price_in_cents is out of range")
	ErrItemRetiredMismatch   = errors.New("menu: retired, retired_at, and retired_by must be set together")
	ErrMenuItemAlreadyExists = errors.New("menu: menu item already exists")
)

const maxNameLen int = 64

// Item is a menu item as the business rules and storage see it. It has no JSON
// tags on purpose: each API route defines its own response shape.
//
// Retired is a soft delete. A retired item stays in the table so past orders,
// reports and audits can still name it, but it is hidden from the menu and new// orders reject it. RetiredAt and RetiredBy record when and by whom.
type Item struct {
	Id           string     `dynamodbav:"id"`
	Name         string     `dynamodbav:"name"`
	PriceInCents int64      `dynamodbav:"price_in_cents"`
	Available    bool       `dynamodbav:"available"`
	Retired      bool       `dynamodbav:"retired"`
	RetiredAt    *time.Time `dynamodbav:"retired_at,omitempty"`
	RetiredBy    string     `dynamodbav:"retired_by,omitempty"`
}

// Repo stores menu items in the orders table under the store's partition.
type Repo struct {
	db    *dynamodb.Client
	table string
}

func NewRepo(db *dynamodb.Client, table string) *Repo {
	return &Repo{
		db:    db,
		table: table,
	}
}

// Create writes the item only if no item with the same ID exists, retired or not.
func (r *Repo) Create(ctx context.Context, storeId string, item Item) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshalling menu item: %w", err)
	}
	for k, v := range key(storeId, item.Id) {
		av[k] = v
	}

	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.table),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(sk)"),
	})
	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return fmt.Errorf("%w: %s", ErrMenuItemAlreadyExists, item.Id)
	}
	return err
}

// NewItem builds a new menu item and validates it. New items start in stock.
func NewItem(name string, priceInCents int64) (Item, error) {
	name = strings.TrimSpace(name)
	id := slug(name)
	if name != "" && id == "" {
		return Item{}, fmt.Errorf("%w: name needs at least one letter or digit", ErrInvalidMenuItem)
	}
	item := Item{
		Id:           id,
		Name:         name,
		PriceInCents: priceInCents,
		Available:    true,
	}
	if err := item.Validate(); err != nil {
		return Item{}, err
	}
	return item, nil
}

// Retire soft-deletes a menu item for DELETE /stores/{storeId}/menu/{itemId}.
// It sets retired, retired_at and retired_by in one write and never removes
// the item. Retiring an item that does not exist, or is already retired,
// returns ErrItemNotFound: from the menu's point of view it is already gone.
func (r *Repo) Retire(ctx context.Context, storeId, itemId, actor string, at time.Time) error {
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(r.table),
		Key:                 key(storeId, itemId),
		UpdateExpression:    aws.String("SET #retired = :true, #retired_at = :at, #retired_by = :by"),
		ConditionExpression: aws.String("attribute_exists(sk) AND #retired = :false"),
		ExpressionAttributeNames: map[string]string{
			"#retired":    "retired",
			"#retired_at": "retired_at",
			"#retired_by": "retired_by",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":true":  &types.AttributeValueMemberBOOL{Value: true},
			":false": &types.AttributeValueMemberBOOL{Value: false},
			":at":    &types.AttributeValueMemberS{Value: at.UTC().Format(time.RFC3339)},
			":by":    &types.AttributeValueMemberS{Value: actor},
		},
	})
	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return fmt.Errorf("%w: %s", ErrItemNotFound, itemId)
	}
	return err
}

// Validate reports every rule i breaks, not just the first.
// It returns nil if i is valid.
func (i Item) Validate() error {
	var errs []error
	if i.Id == "" {
		errs = append(errs, ErrItemMissingId)
	}
	if strings.TrimSpace(i.Name) == "" {
		errs = append(errs, ErrItemMissingName)
	}
	if utf8.RuneCountInString(i.Name) > maxNameLen {
		errs = append(errs, fmt.Errorf("%w: more than %d characters", ErrItemNameTooLong, maxNameLen))
	}
	if i.PriceInCents <= 0 {
		errs = append(errs, fmt.Errorf("%w: got %d, want greater than 0", ErrItemPriceOutOfRange, i.PriceInCents))
	}
	if i.Retired != (i.RetiredAt != nil) || i.Retired != (i.RetiredBy != "") {
		errs = append(errs, ErrItemRetiredMismatch)
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %w", ErrInvalidMenuItem, errors.Join(errs...))
}

// key takes a storeId and itemId and converts it into a DynamoDB key.
func key(storeId, itemId string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "STORE#" + storeId},
		"sk": &types.AttributeValueMemberS{Value: "MENU#" + itemId},
	}
}

// slug takes a string and converts it to url safe Id used in URLs and keys.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
