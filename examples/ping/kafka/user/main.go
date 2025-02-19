//go:generate go run ../../../../cmd/asyncapi-codegen -g user,types -p main -i ../../asyncapi.yaml -o ./user.gen.go

package main

import (
	"context"

	"github.com/znas-io/asyncapi-codegen/pkg/extensions/brokers/kafka"
	"github.com/znas-io/asyncapi-codegen/pkg/extensions/loggers"
	"github.com/znas-io/asyncapi-codegen/pkg/extensions/middlewares"
)

func main() {
	// Instanciate a Kafka controller with a logger
	logger := loggers.NewText()
	broker := kafka.NewController(
		[]string{"kafka:9092"},          // List of hosts
		kafka.WithLogger(logger),        // Attach an internal logger
		kafka.WithGroupID("ping-users"), // Change group id
	)

	// Create a new user controller
	ctrl, err := NewUserController(
		broker,             // Attach the kafka controller
		WithLogger(logger), // Attach an internal logger
		WithMiddlewares(middlewares.Logging(logger))) // Attach a middleware to log messages
	if err != nil {
		panic(err)
	}
	defer ctrl.Close(context.Background())

	// Make a new ping message
	req := NewPingMessage()
	req.Payload = "ping"

	// Create the publication function to send the message
	// Note: it will indefinitely wait to publish as context has no timeout
	publicationFunc := func(ctx context.Context) error {
		return ctrl.PublishPing(ctx, req)
	}

	// The following function will subscribe to the 'pong' channel, execute the publication
	// function and wait for a response. The response will be detected through its
	// correlation ID.
	//
	// This function is available only if the 'correlationId' field has been filled
	// for any channel in the AsyncAPI specification. You will then be able to use it
	// with the form WaitForXXX where XXX is the channel name.
	//
	// Note: it will indefinitely wait for messages as context has no timeout
	_, err = ctrl.WaitForPong(context.Background(), &req, publicationFunc)
	if err != nil {
		panic(err)
	}
}
