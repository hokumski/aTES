package main

import (
	"ates/common"
	"ates/model"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/hamba/avro/v2"
	"sync"
	"time"
)

// startReadingNotification reads topics from Kafka, constructs Notification and sends to notification channel
func (svc *accSvc) startReadingNotification(abortCh <-chan bool) {
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
				err := svc.createUser(msg.Value)
				if err != nil {
					svc.logger.Errorf("Failed to process notification on %s: %s", eventType, err.Error())
					continue
				}

			case "Task.Created", "Task.Completed", "Task.Reassigned":
				var t Task
				err := avro.Unmarshal(model.TaskSchema, msg.Value, &t)

				if err != nil {
					svc.logger.Errorf("Failed to process notification on %s: bad payload, %s", eventType, err.Error())
					continue
				}

				switch eventType {
				case "Task.Created":
					err = svc.createTask(msg.Value)
				case "Task.Completed":
					err = svc.completeTask(t.PublicId, t.AssignedTo.PublicId)
				case "Task.Reassigned":

				}

				if err != nil {
					svc.logger.Errorf("Failed to process notification on %s: %s", eventType, err.Error())
					continue
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
