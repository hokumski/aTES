package main

type Notification struct {
	ty string
}

func (app *atesAuthSvc) notifyAsync(eventType string, e interface{}) {

	switch e.(type) {
	case User:

		switch eventType {
		case "UserCreated":

			//u := e.(User)

			// todo

		}

	}
}
