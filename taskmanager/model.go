package main

import (
	"ates/model"
	"errors"
	"github.com/hamba/avro/v2"
	"gorm.io/gorm"
)

type AuthVerification struct {
	PublicId string `json:"sub"`
}

// User is synced, source is "auth"
type User struct {
	gorm.Model `json:"-"`
	PublicId   string         `json:"uid" avro:"uid"`
	Login      string         `json:"login" avro:"login"`
	RoleID     model.UserRole `json:"roleId" avro:"roleId"` // uint
}

type Task struct {
	gorm.Model   `json:"-"`
	PublicId     string           `gorm:"default:(uuid());unique" json:"tid" avro:"tid"`
	Title        string           `json:"title" avro:"title"`
	Description  string           `json:"description" avro:"description"`
	StatusID     model.TaskStatus `json:"statusId" avro:"statusId"` // uint
	Status       Status           `json:"-"`
	AuthorID     uint             `json:"-"`
	Author       User             `json:"-"`
	AssignedToID uint             `json:"-"`
	AssignedTo   User             `json:"assignedTo" avro:"assignedTo"`
}

func (t *Task) validate() error {
	if t.Title == "" {
		return errors.New("task must contain title")
	}
	if t.Description == "" {
		return errors.New("task must contain description")
	}
	if t.AuthorID == 0 {
		return errors.New("author must be set")
	}
	if t.AssignedToID == 0 {
		return errors.New("task must be assigned")
	}
	if t.StatusID == 0 {
		return errors.New("status must be set")
	}
	return nil
}

func (t *Task) marshal() ([]byte, error) {
	return avro.Marshal(model.TaskSchema, t)
}

func (t *Task) unmarshal(b []byte) error {
	return avro.Unmarshal(model.TaskSchema, b, t)
}

type Status struct {
	gorm.Model
	Name string
}

// TaskLog contains log of status changes
type TaskLog struct {
	gorm.Model
	AssignedToId uint
	TaskId       uint
	StatusId     model.TaskStatus // uint
	Message      string           // commit message
}

func createDefaultStatuses(db *gorm.DB) {
	db.Create(&Status{
		Model: gorm.Model{
			ID: 1,
		},
		Name: "Open",
	})
	db.Create(&Status{
		Model: gorm.Model{
			ID: 2,
		},
		Name: "Closed",
	})
}
