//go:generate go run ../../../cmd/asyncapi-codegen -p v1 -i ./asyncapi-1.0.0.yaml -o ./v1/asyncapi.gen.go
//go:generate go run ../../../cmd/asyncapi-codegen -p v2 -i ./asyncapi-2.0.0.yaml -o ./v2/asyncapi.gen.go

package issue73

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/znas-io/asyncapi-codegen/pkg/extensions"
	"github.com/znas-io/asyncapi-codegen/pkg/extensions/middlewares"
	"github.com/znas-io/asyncapi-codegen/pkg/extensions/versioning"
	"github.com/znas-io/asyncapi-codegen/pkg/utils"
	asyncapi_test "github.com/znas-io/asyncapi-codegen/test"
	v1 "github.com/znas-io/asyncapi-codegen/test/issues/73/v1"
	v2 "github.com/znas-io/asyncapi-codegen/test/issues/73/v2"
)

func TestSuite(t *testing.T) {
	brokers, cleanup := asyncapi_test.BrokerControllers(t)
	defer cleanup()

	for _, b := range brokers {
		suite.Run(t, NewSuite(b))
	}
}

type Suite struct {
	broker extensions.BrokerController
	v1     struct {
		app  *v1.AppController
		user *v1.UserController
	}
	v2 struct {
		app  *v2.AppController
		user *v2.UserController
	}
	interceptor chan extensions.BrokerMessage

	suite.Suite
}

func NewSuite(broker extensions.BrokerController) *Suite {
	return &Suite{
		broker: broker,
	}
}

func (suite *Suite) SetupTest() {
	// Create a channel to intercept message before sending to broker and after
	// reception from broker
	suite.interceptor = make(chan extensions.BrokerMessage, 8)

	// Add a version wrapper to the broker
	vw := versioning.NewWrapper(suite.broker)

	// Create v1 appV1
	appV1, err := v1.NewAppController(vw, v1.WithMiddlewares(middlewares.Intercepter(suite.interceptor)))
	suite.Require().NoError(err)
	suite.v1.app = appV1

	// Create v1 userV1
	userV1, err := v1.NewUserController(vw, v1.WithMiddlewares(middlewares.Intercepter(suite.interceptor)))
	suite.Require().NoError(err)
	suite.v1.user = userV1

	// Create v2 app
	appV2, err := v2.NewAppController(vw, v2.WithMiddlewares(middlewares.Intercepter(suite.interceptor)))
	suite.Require().NoError(err)
	suite.v2.app = appV2

	// Create v2 user
	userV2, err := v2.NewUserController(vw, v2.WithMiddlewares(middlewares.Intercepter(suite.interceptor)))
	suite.Require().NoError(err)
	suite.v2.user = userV2
}

func (suite *Suite) TearDownTest() {
	suite.v1.app.Close(context.Background())
	suite.v1.user.Close(context.Background())
	suite.v2.app.Close(context.Background())
	suite.v2.user.Close(context.Background())
	close(suite.interceptor)
}

func (suite *Suite) TestV1Reception() {
	var wg sync.WaitGroup

	// Expected message
	sent := v1.HelloMessage{
		Payload: "HelloWord!",
	}

	// Check what the app receive and translate
	var recvMsg v1.HelloMessage
	wg.Add(1)
	err := suite.v1.app.SubscribeHello(context.Background(), func(_ context.Context, msg v1.HelloMessage) {
		recvMsg = msg
		wg.Done()
	})
	suite.Require().NoError(err)

	// Check that the other app doesn't receive
	err = suite.v2.app.SubscribeHello(context.Background(), func(_ context.Context, _ v2.HelloMessage) {
		suite.Require().FailNow("this should not happen")
	})
	suite.Require().NoError(err)

	// Publish the message
	err = suite.v1.user.PublishHello(context.Background(), sent)
	suite.Require().NoError(err)

	// Wait for the message to be received by the app
	wg.Wait()

	// Check received message
	suite.Require().Equal(sent, recvMsg)

	// Check sent message to broker
	bMsg := <-suite.interceptor

	// Check payload
	suite.Require().Equal([]byte("HelloWord!"), bMsg.Payload)
}

func (suite *Suite) TestV2Reception() {
	var wg sync.WaitGroup

	// Expected message
	sent := v2.HelloMessage{
		Payload: struct {
			Message   string    `json:"message"`
			Timestamp time.Time `json:"timestamp"`
		}{
			Message:   "HelloWord!",
			Timestamp: utils.Must(time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")).UTC(),
		},
	}

	// Check that the other app doesn't receive
	err := suite.v1.app.SubscribeHello(context.Background(), func(_ context.Context, _ v1.HelloMessage) {
		suite.Require().FailNow("this should not happen")
	})
	suite.Require().NoError(err)

	// Check what the app receive and translate
	var recvMsg v2.HelloMessage
	wg.Add(1)
	err = suite.v2.app.SubscribeHello(context.Background(), func(_ context.Context, msg v2.HelloMessage) {
		recvMsg = msg
		wg.Done()
	})
	suite.Require().NoError(err)

	// Publish the message
	err = suite.v2.user.PublishHello(context.Background(), sent)
	suite.Require().NoError(err)

	// Wait for the message to be received by the app
	wg.Wait()

	// Check received message
	suite.Require().Equal(sent, recvMsg)

	// Check sent message to broker
	bMsg := <-suite.interceptor

	// Check payload
	var p struct {
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
	}
	suite.Require().NoError(json.Unmarshal(bMsg.Payload, &p))
	suite.Require().Equal("HelloWord!", p.Message)

	expected := utils.Must(time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")).UTC()
	suite.Require().WithinDuration(expected, p.Timestamp, time.Millisecond)
}
