package main

import (
	"errors"
	"gorm.io/gorm"
)

type Verification struct {
	PublicId string `json:"sub"`
}

// UserRole copies values from Auth.Role
type UserRole int

const (
	RoleAdmin UserRole = iota + 1
	RoleUser
	RoleManager
	RoleAccountant
)

// User is synced, source is "auth"
type User struct {
	gorm.Model `json:"-"`
	PublicId   string `json:"uid"`
	Login      string `json:"login"`
	RoleID     int    `json:"roleId"`
}

type Task struct {
	gorm.Model   `json:"-"`
	PublicId     string `gorm:"default:(uuid());unique" json:"tid"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	StatusID     int    `json:"statusId"`
	Status       Status `json:"-"`
	AuthorID     int    `json:"authorId"`
	Author       User   `json:"-"`
	AssignedToID int    `json:"assignedId"`
	AssignedTo   User   `json:"-"`
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

type Status struct {
	gorm.Model
	Name string
}

// TaskStatus copies values from TaskManager.Status
type TaskStatus int

const (
	StatusOpen TaskStatus = iota + 1
	StatusClosed
)

// TaskLog contains log of status changes
type TaskLog struct {
	gorm.Model
	OwnerId  int
	TaskId   int
	Task     Task
	StatusId int
	Status   Status
	Message  string // commit message
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
