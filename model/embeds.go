package model

import (
	_ "embed"
	"github.com/hamba/avro/v2"
)

//go:embed user.avsc
var user []byte

//go:embed task.avsc
var task []byte

var UserSchema, _ = avro.Parse(string(user))
var TaskSchema, _ = avro.Parse(string(task))

func validate() error {
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
