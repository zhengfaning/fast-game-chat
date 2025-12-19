package mq

import (
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type RobustMQ struct {
	client mqtt.Client
}

type RobustMQConfig struct {
	Broker   string
	ClientID string
	Username string
	Password string
}

func NewRobustMQ(cfg *RobustMQConfig) *RobustMQ {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)

	// Default handler
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		// Log unexpected messages
		// log.Printf("[RobustMQ] Received unexpected message: %s from topic: %s", msg.Payload(), msg.Topic())
	})

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Println("✅ Connected to RobustMQ Broker")
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("⚠️ RobustMQ Connection Lost: %v", err)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to RobustMQ: %v", token.Error())
	}

	return &RobustMQ{client: client}
}

// Publish sends data to a MQTT topic
func (r *RobustMQ) Publish(topic string, payload []byte) error {
	// QoS 1: At least once
	token := r.client.Publish(topic, 1, false, payload)
	token.Wait()
	if token.Error() != nil {
		return fmt.Errorf("robustmq publish error: %w", token.Error())
	}
	return nil
}

// Subscribe listens to a MQTT topic and returns a read-only channel of messages
func (r *RobustMQ) Subscribe(topic string) (<-chan *Message, error) {
	msgChan := make(chan *Message, 100)

	// NOTE: In robustmq/mqtt, if you want shared subscription (load balancing),
	// you typically use specific syntax like $share/group/topic.
	// We will pass the topic as is, assuming the caller handles the group prefix if needed.

	token := r.client.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
		msgChan <- &Message{
			Topic:   msg.Topic(),
			Payload: msg.Payload(),
		}
	})

	token.Wait()
	if token.Error() != nil {
		return nil, fmt.Errorf("robustmq subscribe error: %w", token.Error())
	}

	return msgChan, nil
}

func (r *RobustMQ) Close() error {
	r.client.Disconnect(250)
	return nil
}
