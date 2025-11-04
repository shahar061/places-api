package services

import (
	"encoding/json"
	"fmt"
	"places_api/internal/config"
	"places_api/internal/types"
	"time"

	"github.com/nats-io/nats.go"
)

// NATSService handles all NATS JetStream operations
type NATSService struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// NewNATSService creates a new NATS service instance
func NewNATSService(cfg *config.NATSConfig) (*NATSService, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("NATS URL is required")
	}

	// Connect to NATS with retry options for better reliability
	// This helps with network issues in containerized environments
	opts := []nats.Option{
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(10),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				fmt.Printf("NATS disconnected: %v\n", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			fmt.Printf("NATS reconnected to %s\n", nc.ConnectedUrl())
		}),
	}

	nc, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %v", err)
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %v", err)
	}

	service := &NATSService{
		conn: nc,
		js:   js,
	}

	// Initialize streams
	if err := service.initializeStreams(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to initialize streams: %v", err)
	}

	fmt.Printf("Connected to NATS at %s\n", cfg.URL)
	return service, nil
}

// initializeStreams creates the necessary JetStream streams
func (n *NATSService) initializeStreams() error {
	// Create JOBS stream
	streamConfig := &nats.StreamConfig{
		Name:     types.JobsStreamName,
		Subjects: []string{"jobs.*"},
		Storage:  nats.FileStorage,
		MaxAge:   24 * time.Hour, // Keep jobs for 24 hours
		Replicas: 1,
	}

	// Try to add stream, ignore if it already exists
	_, err := n.js.AddStream(streamConfig)
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return fmt.Errorf("failed to create JOBS stream: %v", err)
	}

	fmt.Printf("NATS JetStream initialized with stream: %s\n", types.JobsStreamName)
	return nil
}

// PublishFetchAttractionsJob publishes a job to fetch attractions for an area
func (n *NATSService) PublishFetchAttractionsJob(msg *types.FetchAttractionsMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	_, err = n.js.Publish(types.SubjectFetchAttractions, data)
	if err != nil {
		return fmt.Errorf("failed to publish fetch attractions job: %v", err)
	}

	fmt.Printf("Published fetch attractions job for area: %s (job ID: %s)\n", msg.Area.AreaKey, msg.JobID)
	return nil
}

// PublishJobStatus publishes a job status update
func (n *NATSService) PublishJobStatus(msg *types.JobStatusMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	_, err = n.js.Publish(types.SubjectJobStatus, data)
	if err != nil {
		return fmt.Errorf("failed to publish job status: %v", err)
	}

	return nil
}

// SubscribeFetchAttractions creates a durable consumer for fetch attractions jobs
func (n *NATSService) SubscribeFetchAttractions(handler func(*types.FetchAttractionsMessage) error) error {
	// Subscribe with durable consumer configuration
	sub, err := n.js.PullSubscribe(types.SubjectFetchAttractions, types.ConsumerFetchAttractions,
		nats.AckExplicit(),
		nats.MaxDeliver(3),
		nats.AckWait(30*time.Second))
	if err != nil {
		return fmt.Errorf("failed to create pull subscription: %v", err)
	}

	// Start processing messages in a goroutine
	go func() {
		for {
			// Fetch messages
			msgs, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					continue // No messages available, continue polling
				}
				fmt.Printf("Error fetching messages: %v\n", err)
				continue
			}

			for _, msg := range msgs {
				// Parse message
				var fetchMsg types.FetchAttractionsMessage
				if err := json.Unmarshal(msg.Data, &fetchMsg); err != nil {
					fmt.Printf("Error unmarshaling message: %v\n", err)
					msg.Nak()
					continue
				}

				// Process message
				if err := handler(&fetchMsg); err != nil {
					fmt.Printf("Error processing fetch attractions job: %v\n", err)
					msg.Nak()
					continue
				}

				// Acknowledge successful processing
				msg.Ack()
			}
		}
	}()

	fmt.Printf("Subscribed to fetch attractions jobs\n")
	return nil
}

// SubscribeJobStatus creates a subscription for job status updates
func (n *NATSService) SubscribeJobStatus(handler func(*types.JobStatusMessage) error) error {
	// Subscribe to job status updates
	sub, err := n.js.Subscribe(types.SubjectJobStatus, func(msg *nats.Msg) {
		// Parse message
		var statusMsg types.JobStatusMessage
		if err := json.Unmarshal(msg.Data, &statusMsg); err != nil {
			fmt.Printf("Error unmarshaling job status message: %v\n", err)
			return
		}

		// Process message
		if err := handler(&statusMsg); err != nil {
			fmt.Printf("Error processing job status update: %v\n", err)
			return
		}

		// Acknowledge
		msg.Ack()
	}, nats.Durable("job-status-processor"))

	if err != nil {
		return fmt.Errorf("failed to subscribe to job status: %v", err)
	}

	fmt.Printf("Subscribed to job status updates\n")
	_ = sub // Keep reference to prevent garbage collection
	return nil
}

// GetStreamInfo returns information about the jobs stream
func (n *NATSService) GetStreamInfo() (*nats.StreamInfo, error) {
	return n.js.StreamInfo(types.JobsStreamName)
}

// Close closes the NATS connection
func (n *NATSService) Close() {
	if n.conn != nil {
		n.conn.Close()
		fmt.Printf("NATS connection closed\n")
	}
}

// IsConnected checks if the NATS connection is still active
func (n *NATSService) IsConnected() bool {
	return n.conn != nil && n.conn.IsConnected()
}
