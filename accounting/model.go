package main

import "time"

type BillingCycle struct {
	Start time.Time
	End   time.Time
}

type AccountLog struct {
	Debit          int
	Credit         int
	BillingCycleID int
}

type User struct {
}

type Task struct {
}
