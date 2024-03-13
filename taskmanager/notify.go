package main

import (
	"ates/common"
	"ates/model"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/hamba/avro/v2"
	"sync"
	"time"
)

// getTaskForNotification returns new Task with public attributes, ready to be sent as notification
func getTaskForNotification(task *Task) Task {
	return Task{
		PublicId:    task.PublicId,
		JiraId:      task.JiraId,
		Title:       task.Title,
		Description: task.Description,
		StatusID:    task.StatusID,
		AssignedTo: User{
			PublicId: task.AssignedTo.PublicId,
		},
	}
}

// notifyAsync sends notification to Kafka
func (svc *tmSvc) notifyAsync(eventType string, e interface{}) {

	// Important: right now we are sending all events in a single topic,
	// not separating CUD (create-update-delete) and BE (business events).
	// That will be a part of future refactoring (maybe)

	// Also, library confluent-kafka-go has no publicly exposed batch methods.
	// In the contrast, segmentio/kafka-go does have, but we use Confluent to work with SchemaRegistry.

	topic := "task.lifecycle"

	msg := kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(common.GenerateRandomString(10)),
		Value:          nil,
	}

	common.AppendKafkaHeader(&msg, "event", eventType)
	common.AppendKafkaHeader(&msg, "producer", "TaskManager")

	switch e.(type) {
	case Task:

		switch eventType {
		case "Task.Created", "Task.Completed", "Task.Reassigned":
			common.AppendKafkaHeader(&msg, "eventVersion", "v2")

			t := e.(Task)
			t.load(svc)
			taskForNotification := getTaskForNotification(&t)
			b, err := taskForNotification.marshal()
			if err != nil {
				svc.logger.Errorf("failed to marshal Task %s to avro: %s", t.PublicId, err.Error())
				return
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

			switch eventType {
			case "User.Created":
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
