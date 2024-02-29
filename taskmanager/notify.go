package main

import (
	"context"
	"encoding/json"
	"errors"
)

type Notification struct {
	Attributes map[string]string `json:"attributes"`
	Payload    []byte            `json:"payload"`
}

func (n *Notification) marshal() []byte {
	body, _ := json.Marshal(n)
	return body
}

func buildNotification(data []byte) (Notification, error) {
	var n Notification
	err := json.Unmarshal(data, &n)
	return n, err
}

// startReadingNotification reads topics of Kafka, constructs Notification and sends to notification channel
func (svc *tmSvc) startReadingNotification(notifyCh chan<- Notification, abortCh <-chan bool) {
	defer func() {
		_ = svc.kafkaReader.Close()
	}()

	cctx, cancelReader := context.WithCancel(context.Background())
	go func() {
		for {
			msg, err := svc.kafkaReader.ReadMessage(cctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				svc.logger.Error(err)
			}
			n, err := buildNotification(msg.Value)
			if err == nil {
				svc.logger.Infof("Incoming notification from %s: %s", msg.Topic, n.Attributes["event"])
				notifyCh <- n
			} else {
				svc.logger.Errorf("Got notification from %s with wrong format", msg.Topic)
			}
		}
	}()

	for {
		select {
		case _ = <-abortCh:
			cancelReader()
			return
		}
	}

}

// processNotifications reads notification channel, and performs CUD operations with incoming events from other services
func (svc *tmSvc) processNotifications(notifyCh <-chan Notification, abortCh <-chan bool) {
	for {
		select {
		case notification := <-notifyCh:

			eventSource := notification.Attributes["source"]
			eventType := notification.Attributes["event"]
			eventEntity := notification.Attributes["entity"]

			switch eventSource {
			case "auth":

				switch eventType {
				case "UserCreated":
					if eventEntity != "User" {
						continue
					}
					var u User
					err := json.Unmarshal(notification.Payload, &u)
					if err != nil {
						svc.logger.Errorf("Failed to process notification on %s with source %s: bad payload",
							eventType, eventSource)
						continue
					}

					result := svc.tmDb.Create(&u)
					if result.RowsAffected == 1 {
						svc.logger.Infof("Created user %s based on notification %s from %s",
							u.PublicId, eventType, eventSource)
					} else {
						svc.logger.Errorf("Failed to create user %s based on notification %s from %s",
							u.PublicId, eventType, eventSource)
					}

				}

			}

		case _ = <-abortCh:
			return
		}
	}
}
