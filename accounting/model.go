package main

import (
	"ates/model"
	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/hamba/avro/v2"
	"gorm.io/gorm"
	"time"
)

type AuthVerification struct {
	PublicId string `json:"sub"`
}

type BillingCycle struct {
	gorm.Model `json:"-"`
	Day        time.Time
}

type Account struct {
	gorm.Model `json:"-"`
	UserID     int `json:"-"`
	User       User
	Balance    int
}

func (a *Account) load(svc *accSvc) error {
	result := svc.accDb.
		Preload("User").
		Where("user_id = ?", a.UserID).
		Find(&a)
	if result.RowsAffected == 1 {
		return nil
	}
	result = svc.accDb.Create(&a)
	if result.RowsAffected == 1 {
		svc.logger.Infof("Created account for user %s", a.UserID)
		return nil
	}
	svc.logger.Errorf("Failed to create account for user")
	return errors.New("failed to create account for user")
}

type AccountLog struct {
	gorm.Model      `json:"-"`
	LogID           int                        `gorm:"-" avro:"logId" json:"-"`
	UserID          int                        `json:"-" avro:"userId"`
	User            User                       `json:"-" avro:"-"`
	TaskID          int                        `json:"-" avro:"taskId"`
	Task            *Task                      `avro:"-"`
	BillingCycleID  int                        `json:"-" avro:"billingCycleId"`
	OperationTypeID model.AccountOperationType `avro:"operationId"`
	Debit           int                        `avro:"debit"`
	Credit          int                        `avro:"credit"`
	Message         string                     `avro:"-"`
	Balance         int                        `avro:"balance"`
}

func (a *AccountLog) marshal() ([]byte, error) {
	a.LogID = int(a.ID)
	return avro.Marshal(model.AccountLog, a)
}

type OperationType struct {
	gorm.Model `json:"-"`
	Name       string
}

func createDefaultOperations(db *gorm.DB) {
	db.Create(&OperationType{
		Model: gorm.Model{
			ID: 1,
		},
		Name: "CostOfAssigment",
	})
	db.Create(&OperationType{
		Model: gorm.Model{
			ID: 2,
		},
		Name: "CompletionReward",
	})
	db.Create(&OperationType{
		Model: gorm.Model{
			ID: 3,
		},
		Name: "WagePayment",
	})
}

// User is synced, source is "auth"
type User struct {
	gorm.Model `json:"-"`
	PublicId   string         `json:"uid" avro:"uid"`
	Login      string         `json:"login" avro:"login"`
	RoleID     model.UserRole `json:"roleId" avro:"roleId"`
}

func (u *User) load(svc *accSvc) {
	svc.accDb.Where("id = ?", u.ID).Find(&u)
}

func (u *User) loadWithPublicId(svc *accSvc, publicId string) error {
	result := svc.accDb.Where("public_id = ?", publicId).Find(&u)
	if result.RowsAffected == 1 {
		return nil
	}
	return errors.New("user not found")
}

// Task is synced, source is "taskmanager", additional fields here
type Task struct {
	gorm.Model       `json:"-"`
	PublicId         string           `gorm:"default:(uuid());unique" json:"tid" avro:"tid"`
	Title            string           `json:"title" avro:"title"`
	Description      string           `json:"description" avro:"description"`
	StatusID         model.TaskStatus `json:"statusId" avro:"statusId"`
	AssignedToID     int              `json:"-"`
	AssignedTo       User             `gorm:"-" json:"-" avro:"assignedTo"`
	CostOfAssignment int              // set in Accounting
	CompletionReward int              // set in Accounting
}

func (t *Task) loadWithPublicId(svc *accSvc, publicId string) error {
	result := svc.accDb.
		Where("public_id = ?", publicId).Find(&t)
	if result.RowsAffected == 1 {
		return nil
	}
	return errors.New("task not found")
}
