package api

import (
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/bradleyGamiMarques/BitsAndBytes/internal/coffee"
)

const GetProductRoute = "GET /products/{category}/{productId}"

type getProductResponse struct {
	Category     string `json:"category"`
	ProductId    string `json:"product_id"`
	Name         string `json:"name"`
	PriceInCents int64  `json:"price_in_cents"`
	Description  string `json:"description"`
	Size         string `json:"size"`
}

type productGetter interface {
	GetProduct(ctx context.Context, category, productId string) (coffee.Product, error)
}

type GetProduct struct {
	Products productGetter
	Log      *slog.Logger
}

func (h GetProduct) Handle(ctx context.Context, e events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	category := e.PathParameters["category"]
	productId := e.PathParameters["productId"]

	log := h.Log.With(
		"request_id", e.RequestContext.RequestID,
		"category", category,
		"product_id", productId,
	)

	respond := func(status int, body any) events.APIGatewayV2HTTPResponse {
		b, err := json.Marshal(body)
		if err != nil {
			log.Error("marshalling response", "error", err)
			status, b = http.StatusInternalServerError, []byte(`{"error":"could not encode the response"}`)
		}
		return events.APIGatewayV2HTTPResponse{
			StatusCode: status,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(b),
		}
	}

	product, err := h.Products.GetProduct(ctx, category, productId)
	switch {
	case errors.Is(err, coffee.ErrProductNotFound):
		log.Info("product not found")
		return respond(http.StatusNotFound, map[string]string{"error": "product not found"}), nil
	case err != nil:
		log.Error("getting product", "error", err)
		return respond(http.StatusInternalServerError, map[string]string{"error": "could not get the product"}), nil
	}

	log.Info("product found", "name", product.Name)
	return respond(http.StatusOK, getProductResponse{
		Category:     product.Category,
		ProductId:    product.ProductId,
		Name:         product.Name,
		PriceInCents: product.PriceInCents,
		Description:  product.Description,
		Size:         product.Size,
	}), nil
}
