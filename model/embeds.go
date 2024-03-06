package model

import (
	_ "embed"
	"github.com/hamba/avro/v2"
)

//go:embed avro/user.avsc
var user []byte

//go:embed avro/task.avsc
var task []byte

var UserSchema, _ = avro.Parse(string(user))
var TaskSchema, _ = avro.Parse(string(task))

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
	return nil
}
