package main

import (
	"ates/common"
	"context"
	"encoding/json"
	"errors"
	"github.com/labstack/echo/v4"
	"io"
	"net/http"
)

// registerUser reads user data from request body and registers new user
func (svc *authSvc) registerUser(c echo.Context) error {

	// Everybody can add User: kind of self-registration
	// todo: only users can self-register, need AuthZ for adding another roles

	ctx := context.Background()
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		svc.logger.Error(err)
		return c.JSON(http.StatusBadRequest, nil)
	}

	var u User
	err = json.Unmarshal(body, &u)
	if err != nil {
		svc.logger.Error(err)
		return c.JSON(http.StatusBadRequest, nil)
	}

	if u.Login == "" || u.Password == "" {
		return c.JSON(http.StatusInternalServerError,
			common.FromKeysAndValues("error", "must provide both login and password"))
	}

	if u.RoleID == 0 {
		u.RoleID = 2 // User
	}

	err = u.calculatePasswordHash()
	if err != nil {
		svc.logger.Error(err)
		return c.JSON(http.StatusInternalServerError, nil)
	}

	result := svc.userDb.Create(&u)
	if result.RowsAffected == 1 {
		// User creating could be failed if login is not unique (database constraint)
		var userFromDb User
		result = svc.userDb.First(&userFromDb, "login = ?", u.Login)

		if result.RowsAffected == 1 {
			go svc.notifyAsync(&ctx, "UserCreated", userFromDb)
			return c.JSON(http.StatusOK, userFromDb)
		}
	}

	return c.JSON(http.StatusInternalServerError,
		common.FromKeysAndValues("error", "failed to create user"))
}

func (svc *authSvc) deleteUser(c echo.Context) error {
	panic("Not implemented")
}

func (svc *authSvc) updateUser(c echo.Context) error {
	panic("Not implemented")
}

// verify validates access token from request header
func (svc *authSvc) verify(c echo.Context) error {
	tokenInfo, err := svc.oauthServer.ValidationBearerToken(c.Request())
	if err != nil {
		svc.logger.Error(err)
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, AuthVerification{PublicId: tokenInfo.GetUserID()})
}

// token exchanges user and password to access and refresh tokens
// /oauth/token?grant_type=password&username=USER&password=PASSWORD&client_id=default&client_secret=secret
func (svc *authSvc) token(c echo.Context) error {
	err := svc.oauthServer.HandleTokenRequest(c.Response().Writer, c.Request())
	if err != nil {
		svc.logger.Error(err)
	}
	return err
}

func (svc *authSvc) checkPassword(_ context.Context, _, username, password string) (userID string, err error) {
	var userFromDb User
	result := svc.userDb.First(&userFromDb, "login = ?", username)
	if result.RowsAffected == 1 {
		if userFromDb.checkPassword(password) {
			userID = userFromDb.PublicId
		} else {
			err = errors.New("bad password")
		}
	} else {
		err = errors.New("user not found")
	}
	return
}
