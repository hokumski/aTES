package main

import (
	"ates/model"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamba/avro/v2"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"io"
	"math/rand"
	"net/http"
)

// checkAuth checks if current request contain authorization header, sends request to Auth service to check token,
// and ensures if user has one of the following roles
func (svc *accSvc) checkAuth(c echo.Context, availableFor []model.UserRole) (bool, uint) {
	authHeader := c.Request().Header["Authorization"][0]
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
func (svc *accSvc) checkUserRole(publicId string, availableFor []model.UserRole) (bool, uint) {
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
	err := avro.Unmarshal(model.UserSchema, avroPayload, &u)
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
			UserID:  u.ID,
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
func (svc *accSvc) createTask(avroPayload []byte) error {

	var t Task
	err := avro.Unmarshal(model.TaskSchema, avroPayload, &t)
	if err != nil {
		return err
	}

	var u User
	err = u.loadWithPublicId(svc, t.AssignedTo.PublicId)
	if err != nil {
		return err
	}

	t.AssignedToID = u.ID
	t.CostOfAssignment = uint(10 + rand.Intn(10)) // 10..20
	t.CompletionReward = uint(20 + rand.Intn(20)) // 20..40

	err = svc.accDb.Transaction(func(tx *gorm.DB) error {
		result := svc.accDb.Create(&t)
		if result.RowsAffected == 1 {
			svc.logger.Infof("Created task %s", t.PublicId)
		} else {
			svc.logger.Errorf("Failed to create task")
			return errors.New("failed to create task")
		}
		return svc.addOperation(u.ID, t.ID, model.CostOfAssignment, 0, int(t.CostOfAssignment),
			fmt.Sprintf("Deducted %d on assignment task %d", t.CostOfAssignment, t.ID))
	})

	return err
}

// completeTask finds Task with public identifier, marks as completed and
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
		task.StatusID = model.StatusCompleted
		result := svc.accDb.Save(&task)
		if result.RowsAffected != 1 {
			return errors.New("failed to complete task")
		}

		return svc.addOperation(task.AssignedToID, task.ID, model.CompletionReward, int(task.CompletionReward), 0,
			fmt.Sprintf("Added %d on completion task %d", task.CompletionReward, task.ID))
	})

	return err
}

// recordTaskLog adds log record to database, and updates Account balance
func (svc *accSvc) addOperation(userId, taskId uint, operationType model.AccountOperationType, debit, credit int, message string) error {

	a := Account{UserID: userId}
	err := a.load(svc)
	if err != nil {
		return err
	}

	newBalance := a.Balance + debit - credit
	entry := AccountLog{
		UserID:          userId,
		TaskID:          taskId,
		OperationTypeID: uint(operationType),
		Debit:           uint(debit),
		Credit:          uint(credit),
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
	return err
}