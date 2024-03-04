package main

import (
	"ates/common"
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
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

	msg := kafka.Message{
		Key:   []byte(common.GenerateRandomString(10)),
		Value: nil,
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
		err := svc.kafkaWriter.WriteMessages(*ctx, msg)
		if err != nil {
			svc.logger.Errorf("Failed to send event notification on %s", eventType)
			svc.logger.Error(err)
		}
	}
}
