package main

import (
	"ates/common"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// notifyAsync sends notification to Kafka
func (svc *authSvc) notifyAsync(eventType string, e interface{}) {

	topic := "user.lifecycle"

	msg := kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(common.GenerateRandomString(10)),
		Value:          nil,
	}

	common.AppendKafkaHeader(&msg, "event", eventType)
	common.AppendKafkaHeader(&msg, "producer", "Auth")

	switch e.(type) {
	case User:
		switch eventType {
		case "User.Created":
			u := e.(User)
			userForNotify := User{
				PublicId: u.PublicId,
				Login:    u.Login,
				RoleID:   u.RoleID,
			}
			b, err := userForNotify.marshal()
			if err != nil {
				svc.logger.Errorf("failed to marshal User %s to avro", u.PublicId)
			}
			msg.Value = b
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
