//go:generate go run ../../../../cmd/asyncapi-codegen -g application,types -p main -i ../../asyncapi.yaml -o ./app.gen.go

package main

import (
	"context"
	"log"

	"github.com/znas-io/asyncapi-codegen/examples"
	"github.com/znas-io/asyncapi-codegen/pkg/extensions/brokers/nats"
)

func main() {
	// Create a new broker adapter
	broker := nats.NewController("nats://nats:4222", nats.WithQueueGroup("helloworld-apps"))
	defer broker.Close()

	// Create a new application controller
	ctrl, err := NewAppController(broker)
	if err != nil {
		panic(err)
	}
	defer ctrl.Close(context.Background())

	// Subscribe to HelloWorld messages
	// Note: it will indefinitely wait for messages as context has no timeout
	log.Println("Subscribe to hello world...")
	ctrl.SubscribeHello(context.Background(), func(_ context.Context, msg HelloMessage) {
		log.Println("Received message:", msg.Payload)
	})

	// Listen on port to let know that app is ready
	examples.ListenLocalPort(1234)
}
