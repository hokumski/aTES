package main

import (
	"ates/common"
	"ates/schema"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/hamba/avro/v2"
	"sync"
	"time"
)

// notifyAsync sends notification to Kafka
func (svc *accSvc) notifyAsync(eventType string, e interface{}) {

	switch e.(type) {
	case AccountLog:

		topic := "accountlog.lifecycle"
		msg := kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Key:            []byte(common.GenerateRandomString(10)),
			Value:          nil,
		}

		common.AppendKafkaHeader(&msg, "event", eventType)
		common.AppendKafkaHeader(&msg, "producer", "Accounting")

		switch eventType {
		case "AccountLog.Created", "AccountLog.Updated":
			common.AppendKafkaHeader(&msg, "eventVersion", "v1")
			a := e.(AccountLog)
			b, err := a.marshal()
			if err != nil {
				svc.logger.Errorf("failed to marshal AccountLog#%d to avro: %s", a.ID, err.Error())
				return
			}
			msg.Value = b
		}

		if msg.Value != nil {
			err := svc.kafkaProducer.Produce(&msg, nil)
			if err != nil {
				svc.logger.Errorf("Failed to send event notification on %s", eventType)
				svc.logger.Error(err)
			}
		}
	}

}

// startReadingNotification reads topics from Kafka
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
			eventVersion, _ := common.GetKafkaHeader(msg, "eventVersion")

			switch eventType {
			case "User.Created":
				err := svc.createUser(msg.Value)
				if err != nil {
					svc.logger.Errorf("Failed to process notification on %s: %s", eventType, err.Error())
					continue
				}

			case "Task.Created", "Task.Completed", "Task.Reassigned":
				var t Task
				err := avro.Unmarshal(schema.TaskSchema, msg.Value, &t)

				if err != nil {
					svc.logger.Errorf("Failed to process notification on %s: bad payload, %s", eventType, err.Error())
					continue
				}

				switch eventType {
				case "Task.Created":
					err = svc.createTask(msg.Value, eventVersion)
				case "Task.Completed":
					err = svc.completeTask(t.PublicId, t.AssignedTo.PublicId)
				case "Task.Reassigned":
					err = svc.reassignTask(t.PublicId, t.AssignedTo.PublicId)
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
