package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/segmentio/kafka-go"
	"github.com/znas-io/asyncapi-codegen/pkg/extensions"
	"github.com/znas-io/asyncapi-codegen/pkg/extensions/brokers"
)

// Check that it still fills the interface.
var _ extensions.BrokerController = (*Controller)(nil)

// Controller is the Kafka implementation for asyncapi-codegen.
type Controller struct {
	hosts     []string
	partition int
	maxBytes  int

	// Reception only
	groupID string

	logger extensions.Logger
}

// ControllerOption is a function that can be used to configure a Kafka controller
// Examples: WithGroupID(), WithPartition(), WithMaxBytes(), WithLogger().
type ControllerOption func(controller *Controller)

// NewController creates a new KafkaController that fulfill the BrokerLinker interface.
func NewController(hosts []string, options ...ControllerOption) *Controller {
	// Create default controller
	controller := &Controller{
		logger:    extensions.DummyLogger{},
		groupID:   brokers.DefaultQueueGroupID,
		hosts:     hosts,
		partition: 0,
		maxBytes:  10e6, // 10MB
	}

	// Execute options
	for _, option := range options {
		option(controller)
	}

	return controller
}

// WithGroupID set a custom group ID for channel subscription.
func WithGroupID(groupID string) ControllerOption {
	return func(controller *Controller) {
		controller.groupID = groupID
	}
}

// WithPartition set the partition to use for the topic.
func WithPartition(partition int) ControllerOption {
	return func(controller *Controller) {
		controller.partition = partition
	}
}

// WithMaxBytes set the maximum size of a message.
func WithMaxBytes(maxBytes int) ControllerOption {
	return func(controller *Controller) {
		controller.maxBytes = maxBytes
	}
}

// WithLogger set a custom logger that will log operations on broker controller.
func WithLogger(logger extensions.Logger) ControllerOption {
	return func(controller *Controller) {
		controller.logger = logger
	}
}

// Publish a message to the broker.
func (c *Controller) Publish(ctx context.Context, channel string, um extensions.BrokerMessage) error {
	// Create new writer
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  c.hosts,
		Topic:    channel,
		Balancer: &kafka.LeastBytes{},
	})

	// Create the message
	msg := kafka.Message{
		Headers: make([]kafka.Header, 0),
	}

	// Set message content and headers
	msg.Value = um.Payload
	for k, v := range um.Headers {
		msg.Headers = append(msg.Headers, kafka.Header{Key: k, Value: v})
	}

	for {
		// Publish message
		err := w.WriteMessages(ctx, msg)

		// If there is no error then return
		if err == nil {
			return nil
		}

		// Create topic if not exists, then it means that the topic is being
		// created, so let's retry
		if errors.Is(err, kafka.UnknownTopicOrPartition) {
			c.logger.Warning(ctx, fmt.Sprintf("Topic %s does not exists: request creation and retry", channel))
			if err := c.checkTopicExistOrCreateIt(ctx, channel); err != nil {
				return err
			}

			continue
		}

		// Unexpected error
		return err
	}
}

// Subscribe to messages from the broker.
func (c *Controller) Subscribe(ctx context.Context, channel string) (extensions.BrokerChannelSubscription, error) {
	// Check that topic exists before
	if err := c.checkTopicExistOrCreateIt(ctx, channel); err != nil {
		return extensions.BrokerChannelSubscription{}, err
	}

	// Create reader
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   c.hosts,
		Topic:     channel,
		Partition: c.partition,
		MaxBytes:  c.maxBytes,
		GroupID:   c.groupID,
	})

	// Create subscription
	sub := extensions.NewBrokerChannelSubscription(
		make(chan extensions.BrokerMessage, brokers.BrokerMessagesQueueSize),
		make(chan any, 1),
	)

	// Handle events
	go c.messagesHandler(ctx, r, sub)

	// Wait for cancellation and stop the kafka listener when it happens
	sub.WaitForCancellationAsync(func() {
		if err := r.Close(); err != nil {
			c.logger.Error(ctx, err.Error())
		}
	})

	return sub, nil
}

func (c *Controller) messagesHandler(ctx context.Context, r *kafka.Reader, sub extensions.BrokerChannelSubscription) {
	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			// If the error is not io.EOF, then it is a real error
			if !errors.Is(err, io.EOF) {
				c.logger.Warning(ctx, fmt.Sprintf("Error when reading message: %q", err.Error()))
			}

			return
		}

		// Get headers
		headers := make(map[string][]byte, len(msg.Headers))
		for _, header := range msg.Headers {
			headers[header.Key] = header.Value
		}

		// Send received message
		sub.TransmitReceivedMessage(extensions.BrokerMessage{
			Headers: headers,
			Payload: msg.Value,
		})
	}
}

func (c Controller) checkTopicExistOrCreateIt(ctx context.Context, topic string) error {
	// Get connection to first host
	conn, err := kafka.Dial("tcp", c.hosts[0])
	if err != nil {
		return err
	}
	defer conn.Close()

	for i := 0; ; i++ {
		// Create topic
		topicConfigs := []kafka.TopicConfig{{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		}}
		err = conn.CreateTopics(topicConfigs...)
		if err != nil {
			return err
		}

		// Read partitions
		partitions, err := conn.ReadPartitions()
		if err != nil {
			return err
		}

		// Get topic from partitions
		for _, p := range partitions {
			if topic == p.Topic {
				if i > 0 {
					c.logger.Warning(ctx, fmt.Sprintf("Topic %s has been created.", topic))
				}
				return nil
			}
		}

		c.logger.Warning(ctx, fmt.Sprintf("Topic %s doesn't exists yet, retrying (#%d)", topic, i))
	}
}
