package model

import (
	_ "embed"
	"github.com/hamba/avro/v2"
)

//go:embed avro/user.avsc
var user []byte

//go:embed avro/task.avsc
var task []byte

//go:embed avro/accountlog.avsc
var accountLog []byte

var UserSchema, _ = avro.Parse(string(user))
var TaskSchema, _ = avro.Parse(string(task))
var AccountLog, _ = avro.Parse(string(accountLog))

func Validate() error {
	var err error
	UserSchema, err = avro.Parse(string(user))
	if err != nil {
		return err
	}
	TaskSchema, err = avro.Parse(string(task))
	if err != nil {
		return err
	}
	AccountLog, err = avro.Parse(string(accountLog))
	if err != nil {
		return err
	}
	return nil
}
