package mq

import (
	"context"
	"fmt"

	"game-chat-service/internal/logger"

	"github.com/go-redis/redis/v8"
)

type RedisMQ struct {
	client *redis.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func NewRedisMQ(client *redis.Client) *RedisMQ {
	ctx, cancel := context.WithCancel(context.Background())
	return &RedisMQ{
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Publish sends data to a Redis channel
func (r *RedisMQ) Publish(topic string, payload []byte) error {
	err := r.client.Publish(r.ctx, topic, payload).Err()
	if err != nil {
		return fmt.Errorf("redis publish error: %w", err)
	}
	return nil
}

// Subscribe listens to a Redis channel and returns a read-only channel of messages
func (r *RedisMQ) Subscribe(topic string) (<-chan *Message, error) {
	pubsub := r.client.Subscribe(r.ctx, topic)

	// Check connection
	_, err := pubsub.Receive(r.ctx)
	if err != nil {
		return nil, fmt.Errorf("redis subscribe error: %w", err)
	}

	msgChan := make(chan *Message, 100) // Buffer for safety

	// Start a goroutine to bridge Redis PubSub to our channel
	go func() {
		defer close(msgChan)
		defer pubsub.Close()

		ch := pubsub.Channel()
		for {
			select {
			case redisMsg, ok := <-ch:
				if !ok {
					logger.Debug(logger.TagMQ, "Redis PubSub channel closed for topic: %s", topic)
					return
				}
				msgChan <- &Message{
					Topic:   topic,
					Payload: []byte(redisMsg.Payload),
				}
			case <-r.ctx.Done():
				return
			}
		}
	}()

	return msgChan, nil
}

func (r *RedisMQ) Close() error {
	r.cancel()
	return nil
}
