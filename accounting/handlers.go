package main

import (
	"ates/common"
	"ates/model"
	"github.com/labstack/echo/v4"
	"net/http"
	"time"
)

func forbidden(c echo.Context) error {
	return c.JSON(http.StatusForbidden, common.FromKeysAndValues("error", "forbidden"))
}

// getBalance renders current balance of user
func (svc *accSvc) getBalance(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []model.UserRole{model.RoleUser})
	if !userIsAllowed {
		return forbidden(c)
	}

	var account Account
	svc.accDb.Preload("User").Where("user_id = ?", userId).First(&account)

	return c.JSON(http.StatusOK, account)
}

// getLog renders log of operations on user's account for unfinished billing cycle
func (svc *accSvc) getLog(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []model.UserRole{model.RoleUser})
	if !userIsAllowed {
		return forbidden(c)
	}

	log := svc.queryLogOnDay(userId, "")
	return c.JSON(http.StatusOK, log)
}

// getLog renders log of operations on user's account for billing cycle of certain day
func (svc *accSvc) getLogOnDay(c echo.Context) error {
	userIsAllowed, userId := svc.checkAuth(c, []model.UserRole{model.RoleUser})
	if !userIsAllowed {
		return forbidden(c)
	}

	dayParam := c.Param("day") // must be YYYY-MM-DD
	_, err := time.Parse("2006-01-02", dayParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", "bad date format, must be YYYY-MM-DD"))
	}

	log := svc.queryLogOnDay(userId, dayParam)
	return c.JSON(http.StatusOK, log)
}

// getLog renders log of operations on user's account for unfinished billing cycle
func (svc *accSvc) getIncome(c echo.Context) error {
	userIsAllowed, _ := svc.checkAuth(c, []model.UserRole{model.RoleAdmin, model.RoleAccountant})
	if !userIsAllowed {
		return forbidden(c)
	}

	income, _ := svc.queryIncomeOnDay("")
	return c.JSON(http.StatusOK, common.FromKeysAndValues("income", income))
}

// getLog renders log of operations on user's account for billing cycle of certain day
func (svc *accSvc) getIncomeOnDay(c echo.Context) error {
	userIsAllowed, _ := svc.checkAuth(c, []model.UserRole{model.RoleAdmin, model.RoleAccountant})
	if !userIsAllowed {
		return forbidden(c)
	}

	dayParam := c.Param("day") // must be YYYY-MM-DD
	_, err := time.Parse("2006-01-02", dayParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, common.FromKeysAndValues("error", "bad date format, must be YYYY-MM-DD"))
	}

	income, _ := svc.queryIncomeOnDay(dayParam)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, common.FromKeysAndValues("income", income))
}

func (svc *accSvc) getBillingCycleIds(day string) ([]int, error) {
	bcIds := make([]int, 0)
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		return nil, err
	}
	var bcs []BillingCycle
	svc.accDb.Where("day > ? and day < ?",
		d.Format("2006-01-02"), d.AddDate(0, 0, 1).Format("2006-01-02")).
		Find(&bcs)

	for _, bc := range bcs {
		bcIds = append(bcIds, int(bc.ID))
	}
	return bcIds, nil
}

func (svc *accSvc) queryLogOnDay(userId uint, day string) []AccountLog {
	var log []AccountLog
	if day == "" {
		svc.accDb.Preload("Task").Where("user_id = ?", userId).Find(&log)
		return log
	}

	// select billing cycles of that day and query
	bcIds, err := svc.getBillingCycleIds(day)
	if err != nil {
		return log
	}

	svc.accDb.Preload("Task").Where("user_id = ? and billing_cycle_id in ?",
		userId, bcIds).
		Find(&log)

	return log
}

type NResult struct {
	N int64 //or int ,or some else
}

func (svc *accSvc) queryIncomeOnDay(day string) (int, error) {
	var credits, debits int
	var n NResult

	if day == "" {
		svc.accDb.Table("account_logs").
			Where("billing_cycle_id = ? and operation_type_id = ?", 0, model.CostOfAssignment).
			Select("sum(credit) as n").Scan(&n)
		credits = int(n.N)
		svc.accDb.Table("account_logs").
			Where("billing_cycle_id = ? and operation_type_id = ?", 0, model.CompletionReward).
			Select("sum(debit) as n").Scan(&n)
		debits = int(n.N)
	} else {
		// select billing cycles of that day and query
		bcIds, err := svc.getBillingCycleIds(day)
		if err != nil {
			return 0, err
		}

		svc.accDb.Table("account_logs").
			Where("billing_cycle_id IN ? and operation_type_id = ?", bcIds, model.CostOfAssignment).
			Select("sum(credit) as n").Scan(&n)
		credits = int(n.N)
		svc.accDb.Table("account_logs").
			Where("billing_cycle_id IN ? and operation_type_id = ?", bcIds, model.CompletionReward).
			Select("sum(debit) as n").Scan(&n)
		debits = int(n.N)
	}

	return credits - debits, nil
}

// closeDay creates billing cycle, sets today as the day of BC, and creates WagePayment operations
func (svc *accSvc) closeDay(c echo.Context) error {
	userIsAllowed, _ := svc.checkAuth(c, []model.UserRole{model.RoleAdmin})
	if !userIsAllowed {
		return forbidden(c)
	}

	err := svc.createBillingCycle()
	if err == nil {
		return c.JSON(http.StatusOK, nil)
	}
	return c.JSON(http.StatusInternalServerError, nil)
}
