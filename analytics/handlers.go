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

type NResult struct {
	N int64 //or int ,or some else
}

// getToday renders today's metrics
func (svc *anSvc) getToday(c echo.Context) error {
	userIsAllowed, _ := svc.checkAuth(c, []model.UserRole{model.RoleAdmin})
	if !userIsAllowed {
		return forbidden(c)
	}

	var metrics TodayMetrics
	var n NResult

	svc.anDb.Table("account_logs").
		Where("billing_cycle_id = ?", 0).
		Select("sum(credit) - sum(debit) as n").Scan(&n)
	metrics.ManagementProfit = int(n.N)

	svc.anDb.Table("account_logs").
		Where("balance < 0").
		Select("count(distinct user_id) as n").Scan(&n)
	metrics.UsersWithNegativeBalance = int(n.N)

	return c.JSON(http.StatusOK, metrics)
}

// getExpensive renders cost of most expensive task of day or from days interval
func (svc *anSvc) getExpensive(c echo.Context) error {

	// Analytics doesn't have enough data to render title of task!
	// todo: ask TaskManager to stream information about tasks to analytics

	dayFrom := c.Param("dayFrom")
	dayTo := c.Param("dayTo")
	if dayTo == "" {
		dayTo = dayFrom
	}

	dFrom, err := time.Parse("2006-01-02", dayFrom)
	if err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}
	dTo, err := time.Parse("2006-01-02", dayTo)
	if err != nil {
		return c.JSON(http.StatusBadRequest, nil)
	}

	var n NResult
	svc.anDb.Table("account_logs").
		Where("operation_type_id = ? and updated_at > ? and updated_at < ?",
			model.CompletionReward,
			dFrom.Format("2006-01-02"), dTo.AddDate(0, 0, 1).Format("2006-01-02")).
		Select("max(debit) as n").Scan(&n)

	metrics := ExpensiveMetrics{
		DayFrom: dayFrom,
		DayTo:   dayTo,
		Cost:    int(n.N),
	}

	return c.JSON(http.StatusOK, metrics)
}
