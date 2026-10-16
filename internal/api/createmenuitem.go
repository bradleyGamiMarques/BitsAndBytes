package api

import (
	"context"
	"encoding/json/v2"
	"errors"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/bradleyGamiMarques/BitsAndBytes/internal/menu"
)

// CreateMenuItemRoute is the API Gateway route key this handler serves.
const CreateMenuItemRoute = "POST /stores/{storeId}/menu"

type createMenuItemRequest struct {
	Name         string `json:"name"`
	PriceInCents int64  `json:"price_cents"`
}

type createMenuItemResponse struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	PriceInCents int64  `json:"price_in_cents"`
	Available    bool   `json:"available"`
}

// menuCreator is the one thing CreateMenuItem needs from storage.
// Production passes *menu.Repo tests pass a fake.
type menuCreator interface {
	Create(ctx context.Context, storeId string, item menu.Item) error
}

// CreateMenuItem serves POST /stores/{storeId}/menu. API Gateway has already
// checked IAM, so only roles allowed to call this route (the manager) reach it.
type CreateMenuItem struct {
	Menu menuCreator
	Log  *slog.Logger
}

// Handle validates the body, writes the item only if it does not already
// exist, and returns 201 with the new item.
func (h CreateMenuItem) Handle(ctx context.Context, e events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	storeId := e.PathParameters["storeId"]

	// With IAM authorization, API Gateway passes the Cognito identity behind
	// the signed request. It goes in every log line as the caller.
	actor := "unknown"
	if a := e.RequestContext.Authorizer; a != nil && a.IAM != nil {
		actor = a.IAM.CognitoIdentity.IdentityID
	}
	log := h.Log.With(
		"request_id", e.RequestContext.RequestID,
		"store_id", storeId,
		"actor", actor,
	)

	// respond marshals body into the JSON response the tablet recieves.
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
	body := []byte(e.Body)

	var req createMenuItemRequest
	if err := json.Unmarshal(body, &req, json.RejectUnknownMembers(true)); err != nil {
		log.Info("rejected request body", "error", err)
		return respond(http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()}), nil
	}

	item, err := menu.NewItem(req.Name, req.PriceInCents)
	if err != nil {
		log.Info("rejected menu item", "error", err)
		return respond(http.StatusUnprocessableEntity, map[string]string{"error": err.Error()}), nil
	}

	err = h.Menu.Create(ctx, storeId, item)
	switch {
	case errors.Is(err, menu.ErrMenuItemAlreadyExists):
		log.Info("menu item already exists", "item_id", item.Id)
		return respond(http.StatusConflict, map[string]string{"error": err.Error()}), nil
	case err != nil:
		log.Error("creating menu item", "item_id", item.Id, "error", err)
		return respond(http.StatusInternalServerError, map[string]string{"error": "could not create the menu item"}), nil
	}

	log.Info("menu item created", "item_id", item.Id, "price_in_cents", item.PriceInCents)

	resp := respond(http.StatusCreated, createMenuItemResponse{
		Id:           item.Id,
		Name:         item.Name,
		PriceInCents: item.PriceInCents,
		Available:    item.Available,
	})
	resp.Headers["Location"] = "/stores/" + storeId + "/menu/" + item.Id

	return resp, nil
}
