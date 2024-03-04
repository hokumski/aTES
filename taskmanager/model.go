package main

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
)

type AuthVerification struct {
	PublicId string `json:"sub"`
}

// UserRole copies values from Auth.Role
type UserRole uint

const (
	RoleAdmin UserRole = iota + 1
	RoleUser
	RoleManager
	RoleAccountant
)

// User is synced, source is "auth"
type User struct {
	gorm.Model `json:"-"`
	PublicId   string   `json:"uid"`
	Login      string   `json:"login"`
	RoleID     UserRole `json:"roleId"` // uint
}

type Task struct {
	gorm.Model   `json:"-"`
	PublicId     string     `gorm:"default:(uuid());unique" json:"tid"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	StatusID     TaskStatus `json:"statusId"` // uint
	Status       Status     `json:"-"`
	AuthorID     uint       `json:"-"`
	Author       User       `json:"-"`
	AssignedToID uint       `json:"-"`
	AssignedTo   User       `json:"assignedTo"`
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

func (t *Task) marshal() []byte {
	b, _ := json.Marshal(t)
	return b
}

type Status struct {
	gorm.Model
	Name string
}

// TaskStatus copies values from TaskManager.Status
type TaskStatus uint

const (
	StatusOpen TaskStatus = iota + 1
	StatusCompleted
)

// TaskLog contains log of status changes
type TaskLog struct {
	gorm.Model
	AssignedToId uint
	TaskId       uint
	StatusId     TaskStatus // uint
	Message      string     // commit message
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
