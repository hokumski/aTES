package main

import (
	"ates/schema"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamba/avro/v2"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"io"
	"math/rand"
	"net/http"
	"time"
)

// checkAuth checks if current request contain authorization header, sends request to Auth service to check token,
// and ensures if user has one of the following roles
func (svc *accSvc) checkAuth(c echo.Context, availableFor []schema.UserRole) (bool, uint) {
	var authHeader string
	if c.Request().Header["Authorization"] != nil {
		authHeader = c.Request().Header["Authorization"][0]
	}
	if authHeader == "" {
		return false, 0
	}
	sub, err := svc.verifyAuth(authHeader)
	if err != nil {
		svc.logger.Infof("Auth failed: %s", err)
		return false, 0
	}
	return svc.checkUserRole(sub, availableFor)
}

// verifyAuth sends request to Auth service to check token, returns public identifier of authenticated user
func (svc *accSvc) verifyAuth(authz string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/verify", svc.authServer), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", authz)
	resp, err := svc.authHttpClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("authentication failed")
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	var ver AuthVerification
	err = json.Unmarshal(body, &ver)
	if err != nil || ver.PublicId == "" {
		return "", errors.New("bad answer from auth service")
	}
	return ver.PublicId, nil
}

// checkUserRole checks if user with given public identifier belongs to one of the following roles
func (svc *accSvc) checkUserRole(publicId string, availableFor []schema.UserRole) (bool, uint) {
	// could be cached in memory, with invalidation on notification
	var userFromDb User
	result := svc.accDb.First(&userFromDb, "public_id = ?", publicId)
	if result.RowsAffected == 1 {
		for _, availableRoleId := range availableFor {
			if userFromDb.RoleID == availableRoleId {
				return true, userFromDb.ID
			}
		}
	}
	return false, 0
}

// createUser creates User basing on Avro payload, and Account for this user
func (svc *accSvc) createUser(avroPayload []byte) error {
	var u User
	err := avro.Unmarshal(schema.UserSchema, avroPayload, &u)
	if err != nil {
		return err
	}

	err = svc.accDb.Transaction(func(tx *gorm.DB) error {
		result := svc.accDb.Create(&u)
		u.load(svc)
		if result.RowsAffected == 1 {
			svc.logger.Infof("Created user %s", u.PublicId)
		} else {
			svc.logger.Errorf("Failed to create user")
			return errors.New("failed to create user")
		}
		a := Account{
			UserID:  int(u.ID),
			Balance: 0,
		}
		result = svc.accDb.Create(&a)
		if result.RowsAffected == 1 {
			svc.logger.Infof("Created account for user %s", u.PublicId)
		} else {
			svc.logger.Errorf("Failed to create account for user")
			return errors.New("failed to create account for user")
		}
		return nil
	})

	return err
}

// createTask creates Task basing on Avro payload, sets costs, and deducts cost of assignment from user
func (svc *accSvc) createTask(avroPayload []byte, version string) error {

	var t Task
	taskSchema := schema.TaskSchema
	if version == "v1" {
		taskSchema = schema.TaskSchemaV1
	}

	err := avro.Unmarshal(taskSchema, avroPayload, &t)
	if err != nil {
		return err
	}

	var u User
	err = u.loadWithPublicId(svc, t.AssignedTo.PublicId)
	if err != nil {
		return err
	}

	t.AssignedToID = int(u.ID)
	t.CostOfAssignment = 10 + rand.Intn(10) // 10..20
	t.CompletionReward = 20 + rand.Intn(20) // 20..40

	err = svc.accDb.Transaction(func(tx *gorm.DB) error {
		result := svc.accDb.Create(&t)
		if result.RowsAffected == 1 {
			svc.logger.Infof("Created task %s", t.PublicId)
		} else {
			svc.logger.Errorf("Failed to create task")
			return errors.New("failed to create task")
		}
		return svc.addOperation(int(u.ID), int(t.ID), schema.CostOfAssignment, 0, int(t.CostOfAssignment),
			fmt.Sprintf("Deducted %d on assignment task %d", t.CostOfAssignment, t.ID))
	})

	return err
}

// completeTask finds Task with public identifier, marks as completed
func (svc *accSvc) completeTask(tid, uid string) error {

	var task Task
	err := task.loadWithPublicId(svc, tid)
	if err != nil {
		return err
	}

	var u User
	svc.accDb.First(&u, task.AssignedToID)
	if u.PublicId != uid {
		return errors.New(
			fmt.Sprintf("task %s is not assigned to %s", tid, uid),
		)
	}

	err = svc.accDb.Transaction(func(tx *gorm.DB) error {
		task.StatusID = schema.StatusCompleted
		result := svc.accDb.Save(&task)
		if result.RowsAffected != 1 {
			return errors.New("failed to complete task")
		}

		return svc.addOperation(task.AssignedToID, int(task.ID), schema.CompletionReward, task.CompletionReward, 0,
			fmt.Sprintf("Added %d on completion task %d", task.CompletionReward, task.ID))
	})

	return err
}

// reassignTask finds Task with public identifier, and reassigns it to another user
func (svc *accSvc) reassignTask(tid, uid string) error {

	var task Task
	err := task.loadWithPublicId(svc, tid)
	if err != nil {
		return err
	}

	var user User
	err = user.loadWithPublicId(svc, uid)
	if err != nil {
		return err
	}

	err = svc.accDb.Transaction(func(tx *gorm.DB) error {
		task.AssignedToID = int(user.ID)
		result := svc.accDb.Save(&task)
		if result.RowsAffected != 1 {
			return errors.New("failed to update task assignee")
		}

		return svc.addOperation(task.AssignedToID, int(task.ID), schema.CostOfAssignment, 0, task.CostOfAssignment,
			fmt.Sprintf("Deducted %d on reassignment task %d", task.CostOfAssignment, task.ID))
	})

	return err
}

// recordTaskLog adds log record to database, and updates Account balance
func (svc *accSvc) addOperation(userId, taskId int, operationType schema.AccountOperationType, debit, credit int, message string) error {

	a := Account{UserID: userId}
	err := a.load(svc)
	if err != nil {
		return err
	}

	newBalance := a.Balance + debit - credit
	entry := AccountLog{
		UserID:          userId,
		TaskID:          taskId,
		OperationTypeID: operationType,
		Debit:           debit,
		Credit:          credit,
		Message:         message,
		Balance:         newBalance,
	}

	err = svc.accDb.Transaction(func(tx *gorm.DB) error {
		result := svc.accDb.Create(&entry)
		if result.RowsAffected == 1 {
			svc.logger.Infof("Created account log record on %s", message)
		} else {
			svc.logger.Errorf("Failed to create account log")
			return errors.New("failed to create account log")
		}
		a.Balance = newBalance
		result = svc.accDb.Save(&a)
		if result.RowsAffected == 1 {
			svc.logger.Infof("Updated account balance on %s", message)
		} else {
			svc.logger.Errorf("Failed to update account balance")
			return errors.New("failed to update account balance")
		}
		return nil
	})
	if err == nil {
		go svc.notifyAsync("AccountLog.Created", entry)
	}

	return err
}

func (svc *accSvc) createBillingCycle() error {

	bc := BillingCycle{
		Day: time.Now().UTC(),
	}

	// Actually, GORM transaction doesn't work like transaction
	// Rollback after error on addOperation doesn't work!
	err := svc.accDb.Transaction(func(tx *gorm.DB) error {
		result := svc.accDb.Create(&bc)
		if result.RowsAffected == 1 {
			svc.logger.Infof("Created billing cycle record for %s", bc.Day)
		} else {
			svc.logger.Errorf("Failed to create billing cycle")
			return errors.New("failed to create billing cycle")
		}

		// selecting all account with positive balance
		var accs []Account
		result = svc.accDb.Where("balance > 0").Find(&accs)
		if result.RowsAffected > 0 {
			// create WagePayment for every
			for _, acc := range accs {
				// this also modifies balance
				_ = svc.payWage(acc.UserID, acc.Balance) // does nothing
				// 1 is hardcode
				err := svc.addOperation(acc.UserID, 0, schema.WagePayment, 0, acc.Balance, fmt.Sprintf("Wage %d is paid", acc.Balance))
				if err != nil {
					tx.Rollback()
					return err
				}
			}
		}

		// set current billing cycle for account logs without it (including WagePayment created right before)
		// error here, now we use " = 0 " instead of "is null", because GORM can't create appropriate tables
		result = svc.accDb.Model(&AccountLog{}).Where("billing_cycle_id = ?", 0).Update("billing_cycle_id", bc.ID)
		var log []AccountLog
		svc.accDb.Where("billing_cycle_id = ?", bc.ID).Find(&log)
		for _, a := range log {
			go svc.notifyAsync("AccountLog.Updated", a)
		}

		return nil
	})
	return err
}

func (svc *accSvc) payWage(userId int, balance int) error {
	return nil
}
