package main

import (
	"ates/common"
	"context"
	"encoding/json"
	"errors"
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

func buildNotification(data []byte) (Notification, error) {
	var n Notification
	err := json.Unmarshal(data, &n)
	return n, err
}

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

	msg := kafka.Message{
		Key:   []byte(common.GenerateRandomString(10)),
		Value: nil,
	}

	attributes := make(map[string]string)
	attributes["event"] = eventType

	switch e.(type) {
	case []Task:
		// ...
	case Task:
		attributes["entity"] = "Task"

		switch eventType {
		case "TaskCreated", "TaskCompleted", "TaskReassigned":

			t := e.(Task)
			taskForNotification := getTaskForNotification(&t)

			notification := Notification{
				Attributes: attributes,
				Payload:    taskForNotification.marshal(),
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

// startReadingNotification reads topics from Kafka, constructs Notification and sends to notification channel
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

			eventType := notification.Attributes["event"]
			eventEntity := notification.Attributes["entity"]

			switch eventType {
			case "UserCreated":
				if eventEntity != "User" {
					continue
				}
				var u User
				err := json.Unmarshal(notification.Payload, &u)
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

		case _ = <-abortCh:
			return
		}
	}
}
