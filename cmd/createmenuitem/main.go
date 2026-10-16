// Command createmenuitem is the Lambda entry point for POST /stores/{storeId}/menu.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/bradleyGamiMarques/BitsAndBytes/internal/api"
	"github.com/bradleyGamiMarques/BitsAndBytes/internal/menu"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("route", api.CreateMenuItemRoute)

	table := os.Getenv("TABLE_NAME")
	if table == "" {
		log.Error("TABLE_NAME is not set")
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Error("loading AWS config", "error", err)
		os.Exit(1)
	}

	h := api.CreateMenuItem{
		Menu: menu.NewRepo(dynamodb.NewFromConfig(cfg), table),
		Log:  log,
	}
	lambda.Start(h.Handle)
}
