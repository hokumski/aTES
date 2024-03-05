package main

import (
	"ates/common"
	"ates/model"
	"context"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/hamba/avro/v2"
	"sync"
	"time"
)

// getTaskForNotification returns new Task with public attributes, ready to be sent as notification
func getTaskForNotification(task *Task) Task {
	return Task{
		PublicId:    task.PublicId,
		Title:       task.Title,
		Description: task.Description,
		StatusID:    task.StatusID,
		AssignedTo: User{
			PublicId: task.AssignedTo.PublicId,
		},
	}
}

// notifyAsync sends notification to Kafka
func (svc *tmSvc) notifyAsync(ctx *context.Context, eventType string, e interface{}) {

	topic := "taskmanager_events"

	msg := kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(common.GenerateRandomString(10)),
		Value:          nil,
	}

	common.AppendKafkaHeader(&msg, "event", eventType)

	switch e.(type) {
	case []Task:
		// ...
	case Task:
		common.AppendKafkaHeader(&msg, "entity", "Task")

		switch eventType {
		case "TaskCreated", "TaskCompleted", "TaskReassigned":

			t := e.(Task)
			taskForNotification := getTaskForNotification(&t)
			b, err := taskForNotification.marshal()
			if err != nil {
				svc.logger.Errorf("failed to marshal Task %s to avro", t.PublicId)
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

// startReadingNotification reads topics from Kafka, constructs Notification and sends to notification channel
func (svc *tmSvc) startReadingNotification(abortCh <-chan bool) {
	defer func() {
		_ = svc.kafkaConsumer.Close()
	}()

	run := true
	runMx := &sync.Mutex{}

	go func() {
		for run {
			msg, err := svc.kafkaConsumer.ReadMessage(time.Second)
			if err != nil {
				if err.(kafka.Error).IsTimeout() {
					continue
				}
				svc.logger.Error(err)
				continue
				//return
			}

			eventType, err := common.GetKafkaHeader(msg, "event")
			if err != nil {
				svc.logger.Infof("missing event header in the message %s, skipping", msg.Key)
				continue
			}
			eventEntity, err := common.GetKafkaHeader(msg, "entity")
			if err != nil {
				svc.logger.Infof("missing entity header in the message %s, skipping", msg.Key)
				continue
			}

			switch eventType {
			case "UserCreated":
				if eventEntity != "User" {
					continue
				}
				var u User
				err := avro.Unmarshal(model.UserSchema, msg.Value, &u)
				if err != nil {
					svc.logger.Errorf("Failed to process notification on %s: bad payload", eventType)
					continue
				}

				result := svc.tmDb.Create(&u)
				if result.RowsAffected == 1 {
					svc.logger.Infof("Created user %s based on notification %s", u.PublicId, eventType)
				} else {
					svc.logger.Errorf("Failed to create user %s based on notification %s", u.PublicId, eventType)
				}

			}
		}
	}()

	for {
		select {
		case _ = <-abortCh:
			runMx.Lock()
			run = false
			runMx.Unlock()
			return
		}
	}

}
