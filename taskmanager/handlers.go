package main

import (
	"ates/common"
	"ates/model"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"math/rand"
	"net/http"
)

func forbidden(c echo.Context) error {
	return c.JSON(http.StatusForbidden, common.FromKeysAndValues("error", "forbidden"))
}

// newTask creates new task, and assigns it to random user
func (svc *tmSvc) newTask(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []model.UserRole{model.RoleAdmin, model.RoleUser, model.RoleAccountant, model.RoleManager})
	if !userIsAllowed {
		return forbidden(c)
	}

	task, err := getTaskFromRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", err.Error()))
	}

	userIds := svc.getUserIds()
	randomUserId := userIds[rand.Intn(len(userIds))]

	task.AuthorID = userId
	task.AssignedToID = randomUserId
	task.StatusID = model.StatusOpen

	err = task.validate()
	if err != nil {
		svc.logger.Errorf("Failed to create new task: %s", err.Error())
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", err.Error()))
	}

	err = svc.tmDb.Transaction(func(tx *gorm.DB) error {
		result := svc.tmDb.Create(&task)
		if result.RowsAffected != 1 {
			return errors.New("failed to create task on db request")
		}
		return svc.recordTaskLog(&task, fmt.Sprintf("created by user#%d", task.AuthorID))
	})

	if err == nil {
		svc.logger.Infof("New task created by user#%d", userId)
		//ctx := context.Background()
		//go svc.notifyAsync(&ctx, "TaskCreated", task)
		return c.JSON(http.StatusOK, common.FromKeysAndValues("result", "task created"))
	}

	svc.logger.Errorf(err.Error())
	svc.logger.Error(task)
	return c.JSON(http.StatusInternalServerError, common.FromKeysAndValues("error", "failed to create task"))
}

// getOpenTasks renders tasks of current user with status=Open
func (svc *tmSvc) getOpenTasks(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []model.UserRole{model.RoleUser})
	if !userIsAllowed {
		return forbidden(c)
	}

	var tasks []Task
	svc.tmDb.
		Preload("AssignedTo").
		Where("assigned_to_id = ? and status_id = ?", userId, model.StatusOpen).
		Find(&tasks)

	return c.JSON(http.StatusOK, tasks)
}

// getTask renders task of current user with additional information by id
func (svc *tmSvc) getTask(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []model.UserRole{model.RoleUser})
	if !userIsAllowed {
		return forbidden(c)
	}

	tid := c.Param("tid")
	if !common.IsUUID(tid) {
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", "bad id"))
	}

	var task Task
	result := svc.tmDb.
		Preload("AssignedTo").
		Where("public_id = ? AND assigned_to_id = ?", tid, userId).
		Find(&task)
	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, nil)
	}

	return c.JSON(http.StatusOK, task)
}

// completeTask sets task status to Complete
func (svc *tmSvc) completeTask(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []model.UserRole{model.RoleUser})
	if !userIsAllowed {
		return forbidden(c)
	}

	tid := c.Param("tid")
	if !common.IsUUID(tid) {
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", "bad id"))
	}

	var task Task
	result := svc.tmDb.
		Preload("AssignedTo").
		Where("public_id = ? AND assigned_to_id = ? AND status_id = ?", tid, userId, model.StatusOpen).
		Find(&task)

	if result.RowsAffected == 0 {
		return c.JSON(http.StatusNotFound, nil)
	}

	err := svc.tmDb.Transaction(func(tx *gorm.DB) error {
		task.StatusID = model.StatusCompleted
		result = svc.tmDb.Save(&task)
		if result.RowsAffected != 1 {
			return errors.New("failed to complete task")
		}
		return svc.recordTaskLog(&task, "completed")
	})

	if err == nil {
		svc.logger.Infof("task %s is set completed", tid)
		//ctx := context.Background()
		//go svc.notifyAsync(&ctx, "TaskCompleted", task)
		return c.JSON(http.StatusOK, common.FromKeysAndValues("result", "task completed"))
	}

	svc.logger.Errorf(err.Error())
	svc.logger.Error(task)
	return c.JSON(http.StatusInternalServerError, common.FromKeysAndValues("error", "failed to complete task"))
}

// reassignTasks reassign all tasks with status=Open to users
func (svc *tmSvc) reassignTasks(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []model.UserRole{model.RoleManager, model.RoleAdmin})
	if !userIsAllowed {
		return forbidden(c)
	}

	var tasks []Task
	svc.tmDb.Where("status_id = ?", model.StatusOpen).Find(&tasks)
	if len(tasks) == 0 {
		return c.JSON(http.StatusOK, common.FromKeysAndValues("result", "no open tasks to reassign"))
	}

	allUsers := svc.getUserIds()
	if len(allUsers) == 0 {
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", "failed to assign to users"))
	}

	err := svc.tmDb.Transaction(func(tx *gorm.DB) error {
		for _, task := range tasks {
			task.AssignedToID = allUsers[rand.Intn(len(allUsers))] // random user
			result := svc.tmDb.Save(&task)
			if result.RowsAffected != 1 {
				return errors.New(fmt.Sprintf("failed to reassign task %s", task.PublicId))
			}
			err := svc.recordTaskLog(&task, fmt.Sprintf("reassigned by user#%d", userId))
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err == nil {
		svc.logger.Infof("%d task are reassigned", len(tasks))
		//ctx := context.Background()
		// todo: choose one of:
		//for _, task := range tasks {
		//	go svc.notifyAsync(&ctx, "TaskReassigned", task)
		//}
		// OR (need to implement msg... on notifyAsync)
		//go svc.notifyAsync(&ctx, "TaskReassigned", tasks)

		return c.JSON(http.StatusOK, common.FromKeysAndValues("result", "tasks reassigned"))
	}

	svc.logger.Errorf(err.Error())
	return c.JSON(http.StatusInternalServerError, common.FromKeysAndValues("error", "failed to reassign tasks"))
}
