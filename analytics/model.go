package main

import (
	"ates/model"
	"gorm.io/gorm"
)

type AuthVerification struct {
	PublicId string `json:"sub"`
}

type AccountLog struct {
	gorm.Model
	LogID           int                        `gorm:"logId" avro:"logId"`
	UserID          int                        `avro:"userId"`
	TaskID          int                        `avro:"taskId"`
	BillingCycleID  int                        `avro:"billingCycleId"`
	OperationTypeID model.AccountOperationType `avro:"operationId"`
	Debit           int                        `avro:"debit"`
	Credit          int                        `avro:"credit"`
	Balance         int                        `avro:"balance"`
}

// User is synced, source is "auth"
type User struct {
	gorm.Model `json:"-"`
	PublicId   string         `json:"uid" avro:"uid"`
	Login      string         `json:"login" avro:"login"`
	RoleID     model.UserRole `json:"roleId" avro:"roleId"`
}
