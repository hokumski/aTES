package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"io"
	"net/http"
)

// recordTaskLog adds log record to database
func (svc *tmSvc) recordTaskLog(task *Task, message string) error {
	record := TaskLog{
		Model:        gorm.Model{},
		AssignedToId: task.AssignedToID,
		TaskId:       task.ID,
		StatusId:     task.StatusID,
		Message:      message,
	}
	result := svc.tmDb.Create(&record)
	if result.RowsAffected != 1 {
		return errors.New("failed to created TaskLog record")
	}
	return nil
}

// checkAuth checks if current request contain authorization header, sends request to Auth service to check token,
// and ensures if user has one of the following roles
func (svc *tmSvc) checkAuth(c echo.Context, availableFor []UserRole) (bool, uint) {
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
func (svc *tmSvc) verifyAuth(authz string) (string, error) {
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
func (svc *tmSvc) checkUserRole(publicId string, availableFor []UserRole) (bool, uint) {
	// could be cached in memory, with invalidation on notification
	var userFromDb User
	result := svc.tmDb.First(&userFromDb, "public_id = ?", publicId)
	if result.RowsAffected == 1 {
		for _, availableRoleId := range availableFor {
			if userFromDb.RoleID == availableRoleId {
				return true, userFromDb.ID
			}
		}
	}
	return false, 0
}

// getUserIds return identifiers of all users with Role=User
func (svc *tmSvc) getUserIds() []uint {
	// could be cached in memory, with invalidation on notification
	var users []User
	svc.tmDb.Where("role_id = ?", RoleUser).Find(&users)
	result := make([]uint, len(users))
	for i, u := range users {
		result[i] = u.ID
	}
	return result
}

// getTaskFromRequest constructs Task based on the request payload
func getTaskFromRequest(c echo.Context) (Task, error) {
	var task Task
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		c.Logger().Errorf("Failed to get body of newTask request\n%s", err.Error())
		return Task{}, errors.New("failed to get body of newTask request")
	}
	err = json.Unmarshal(body, &task)
	if err != nil {
		c.Logger().Errorf("Failed to process body of newTask request\n%s", err.Error())
		return Task{}, errors.New("failed to process body of newTask request")
	}
	return task, nil
}
