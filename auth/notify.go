package main

import (
	"ates/common"
	"context"
	"encoding/json"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type Notification struct {
	Attributes map[string]string `json:"attributes"`
	Payload    []byte            `json:"payload"`
}

func (n *Notification) marshal() []byte {
	body, _ := json.Marshal(n)
	return body
}

// notifyAsync sends notification to Kafka
func (svc *authSvc) notifyAsync(ctx *context.Context, eventType string, e interface{}) {

	topic := "user_events"

	msg := kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(common.GenerateRandomString(10)),
		Value:          nil,
	}

	attributes := make(map[string]string)
	attributes["event"] = eventType

	switch e.(type) {
	case User:
		attributes["entity"] = "User"

		switch eventType {
		case "UserCreated":
			u := e.(User)
			userForNotify := User{
				PublicId: u.PublicId,
				Login:    u.Login,
				RoleID:   u.RoleID,
			}

			notification := Notification{
				Attributes: attributes,
				Payload:    userForNotify.marshal(),
			}
			msg.Value = notification.marshal()

		}
	}

	if msg.Value != nil {
		err := svc.kafkaProducer.Produce(&msg, nil)
		if err != nil {
			svc.logger.Errorf("Failed to send event notification on %s", eventType)
			svc.logger.Error(err)
		}
	}
}
