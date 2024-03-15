package main

import (
	"ates/common"
	"ates/schema"
	"errors"
	"github.com/hamba/avro/v2"
	"gorm.io/gorm"
)

type AuthVerification struct {
	PublicId string `json:"sub"`
}

type User struct {
	gorm.Model   `json:"-"`
	PublicId     string          `gorm:"default:(uuid());unique" json:"uid" avro:"uid"`
	Login        string          `gorm:"unique" json:"login" avro:"login"`
	Password     string          `gorm:"-" json:"password,omitempty"`
	PasswordHash string          `json:"-"`
	PasswordSalt string          `json:"-"`
	RoleID       schema.UserRole `json:"roleId" avro:"roleId"`
	Role         Role            `json:"-"`
}

func (u *User) calculatePasswordHash() error {
	if u.Password == "" {
		return errors.New("password must be set")
	}
	u.PasswordSalt = common.GenerateRandomString(10)
	u.PasswordHash = common.HashSHA256([]byte(u.Password + u.PasswordSalt))
	return nil
}

func (u *User) checkPassword(password string) bool {
	if u.PasswordHash == "" {
		return false
	}
	hash := common.HashSHA256([]byte(password + u.PasswordSalt))
	return hash == u.PasswordHash
}

func (u *User) marshal() ([]byte, error) {
	return avro.Marshal(schema.UserSchema, u)
}

func (u *User) unmarshal(b []byte) error {
	return avro.Unmarshal(schema.UserSchema, b, u)
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
