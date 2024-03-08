package main

import (
	"ates/common"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"sync"
	"time"
)

// startReadingNotification reads topics from Kafka
func (svc *anSvc) startReadingNotification(abortCh <-chan bool) {
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
				err = svc.createUser(msg.Value)
			case "AccountLog.Created":
				err = svc.createAccountLog(msg.Value)
			case "AccountLog.Updated":
				err = svc.updateAccountLog(msg.Value)
			}
			if err != nil {
				svc.logger.Errorf("Failed to process notification on %s: %s", eventType, err.Error())
				continue
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
