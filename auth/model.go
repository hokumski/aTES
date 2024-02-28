package main

import (
	"ates/common"
	"errors"
	"gorm.io/gorm"
)

type Verification struct {
	PublicId string `json:"sub"`
}

type User struct {
	gorm.Model   `json:"-"`
	PublicId     string `gorm:"default:(uuid())" json:"uid"`
	Login        string `gorm:"unique" json:"login"`
	Password     string `gorm:"-" json:"password,omitempty"`
	PasswordHash string `json:"-"`
	RoleID       int    `json:"roleId"`
	Role         Role   `json:"-"`
}

func (u *User) calculatePasswordHash() error {
	if u.Password == "" {
		return errors.New("password must be set")
	}
	u.PasswordHash = common.HashSHA256([]byte(u.Password))
	return nil
}

func (u *User) checkPassword(password string) bool {
	if u.PasswordHash == "" {
		return false
	}
	hash := common.HashSHA256([]byte(password))
	return hash == u.PasswordHash
}

type Role struct {
	gorm.Model
	Name string
}

func createDefaultRoles(db *gorm.DB) {
	db.Create(&Role{
		Model: gorm.Model{
			ID: 1,
		},
		Name: "Admin",
	})
	db.Create(&Role{
		Model: gorm.Model{
			ID: 2,
		},
		Name: "User",
	})
	db.Create(&Role{
		Model: gorm.Model{
			ID: 3,
		},
		Name: "Manager",
	})
	db.Create(&Role{
		Model: gorm.Model{
			ID: 4,
		},
		Name: "Accountant",
	})
}
