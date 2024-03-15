package main

import (
	"ates/schema"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hamba/avro/v2"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
)

// checkAuth checks if current request contain authorization header, sends request to Auth service to check token,
// and ensures if user has one of the following roles
func (svc *anSvc) checkAuth(c echo.Context, availableFor []schema.UserRole) (bool, uint) {
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
func (svc *anSvc) verifyAuth(authz string) (string, error) {
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
func (svc *anSvc) checkUserRole(publicId string, availableFor []schema.UserRole) (bool, uint) {
	// could be cached in memory, with invalidation on notification
	var userFromDb User
	result := svc.anDb.First(&userFromDb, "public_id = ?", publicId)
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
func (svc *anSvc) createUser(avroPayload []byte) error {
	var u User
	err := avro.Unmarshal(schema.UserSchema, avroPayload, &u)
	if err != nil {
		svc.logger.Errorf("Failed to unmarshal avro payload of User")
		return err
	}

	result := svc.anDb.Create(&u)
	if result.RowsAffected == 1 {
		svc.logger.Infof("Created user %s", u.PublicId)
	} else {
		svc.logger.Errorf("Failed to create user %s", u.PublicId)
	}
	return nil
}

func (svc *anSvc) createAccountLog(avroPayload []byte) error {
	var a AccountLog
	err := avro.Unmarshal(schema.AccountLog, avroPayload, &a)
	if err != nil {
		svc.logger.Errorf("Failed to unmarshal avro payload of AccountLog")
		return err
	}

	result := svc.anDb.Create(&a)
	if result.RowsAffected == 1 {
		svc.logger.Infof("Created account log %d", a.ID)
	} else {
		svc.logger.Errorf("Failed to create account log %d", a.ID)
	}
	return nil
}

func (svc *anSvc) updateAccountLog(avroPayload []byte) error {
	var a, adb AccountLog
	err := avro.Unmarshal(schema.AccountLog, avroPayload, &a)
	if err != nil {
		svc.logger.Errorf("Failed to unmarshal avro payload of AccountLog")
		return err
	}
	logId := a.LogID
	if logId == 0 {
		svc.logger.Errorf("Missing LogId in payload of AccountLog")
		return errors.New("missing LogId")
	}

	result := svc.anDb.Where("log_id = ?", logId).First(&adb)
	if result.RowsAffected == 1 {
		_ = avro.Unmarshal(schema.AccountLog, avroPayload, &adb)
		svc.anDb.Save(&adb)
	} else {
		svc.logger.Errorf("Record for LogId=%d not found", logId)
		return errors.New("record for LogId not found")
	}

	return nil
}
