package main

import (
	"ates/common"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
)

func forbidden(c echo.Context) error {
	return c.JSON(http.StatusForbidden, common.FromKeysAndValues("error", "forbidden"))
}

// newTask creates new task, and assigns it to random user
func (svc *tmSvc) newTask(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []UserRole{RoleAdmin, RoleUser, RoleAccountant, RoleManager})
	if !userIsAllowed {
		return forbidden(c)
	}

	task, err := getTaskFromRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", err.Error()))
	}

	randomUserId, err := svc.getRandomUser()
	if err != nil {
		svc.logger.Errorf("Failed to find random user to assign new task")
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", "failed to choose user to assign new task"))
	}
	task.AuthorID = userId
	task.AssignedToID = randomUserId

	err = task.validate()
	if err != nil {
		svc.logger.Errorf("Failed to create new task: %s", err.Error())
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", err.Error()))
	}

	result := svc.tmDb.Create(&task)
	if result.RowsAffected == 1 {
		svc.logger.Infof("New task created by user#%d", userId)
		return c.JSON(http.StatusOK, common.FromKeysAndValues("result", "task created"))
	}
	svc.logger.Errorf("Failed to create task on db request")
	svc.logger.Error(task)

	return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", "failed to create task"))
}

// getTasks renders tasks of current user
func (svc *tmSvc) getTasks(c echo.Context) error {
	return c.JSON(http.StatusOK, nil)
}

// getTask renders task of current user with additional information by id
func (svc *tmSvc) getTask(c echo.Context) error {
	return c.JSON(http.StatusOK, nil)
}

// completeTask sets task status to Complete
func (svc *tmSvc) completeTask(c echo.Context) error {
	return c.JSON(http.StatusOK, nil)
}

// reassignTasks reassign all tasks with status=Open to users
func (svc *tmSvc) reassignTasks(c echo.Context) error {
	return c.JSON(http.StatusOK, nil)
}

// checkAuth if current request contain authorization header, sends request to Auth service to check token,
// and checks if user has one of the following roles
func (svc *tmSvc) checkAuth(c echo.Context, availableFor []UserRole) (bool, int) {
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

// verifyAuth performs request to Auth service to check token, returns public identifier of authenticated user
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
	var ver Verification
	err = json.Unmarshal(body, &ver)
	if err != nil || ver.PublicId == "" {
		return "", errors.New("bad answer from auth service")
	}
	return ver.PublicId, nil
}

// checkUserRole checks if user with given public identifier belongs to one of the following roles
func (svc *tmSvc) checkUserRole(publicId string, availableFor []UserRole) (bool, int) {
	var userFromDb User
	result := svc.tmDb.First(&userFromDb, "public_id = ?", publicId)
	if result.RowsAffected == 1 {
		// User found, checking role
		for _, availableRoleId := range availableFor {
			if userFromDb.RoleID == int(availableRoleId) {
				return true, int(userFromDb.ID)
			}
		}
	}
	return false, 0
}

func (svc *tmSvc) getRandomUser() (int, error) {

	return 0, nil
}

// getTaskFromRequest constructs Task based on the payload of request
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
