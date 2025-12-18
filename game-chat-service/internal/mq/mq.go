package mq

// Message represents a message in the queue
type Message struct {
	Topic   string
	Payload []byte
}

// Producer defines interface for publishing messages
type Producer interface {
	Publish(topic string, payload []byte) error
}

// Consumer defines interface for subscribing to topics
type Consumer interface {
	Subscribe(topic string) (<-chan *Message, error)
	Close() error
}
