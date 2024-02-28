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

func (app *atesAuthSvc) registerUser(c echo.Context) error {

	// Everybody can add User: kind of self-registration
	// todo: only users can self-register, need AuthZ for adding another roles

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		app.logger.Error(err)
		return c.JSON(http.StatusInternalServerError, nil)
	}

	var u User
	err = json.Unmarshal(body, &u)
	if err != nil {
		app.logger.Error(err)
		return c.JSON(http.StatusInternalServerError, nil)
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
		app.logger.Error(err)
		return c.JSON(http.StatusInternalServerError, nil)
	}

	result := app.userDb.Create(&u)
	if result.RowsAffected == 1 {
		// User creating could be failed if login is not unique (database constraint)
		var userFromDb User
		result = app.userDb.First(&userFromDb, "login = ?", u.Login)
		if result.RowsAffected == 1 {
			go app.notifyAsync("UserCreated", userFromDb)
			return c.JSON(http.StatusOK, userFromDb)
		}
	}

	return c.JSON(http.StatusInternalServerError,
		common.FromKeysAndValues("error", "failed to create user"))
}

func (app *atesAuthSvc) deleteUser(c echo.Context) error {
	panic("Not implemented")
}

func (app *atesAuthSvc) updateUser(c echo.Context) error {
	panic("Not implemented")
}

func (app *atesAuthSvc) verify(c echo.Context) error {
	tokenInfo, err := app.oauthServer.ValidationBearerToken(c.Request())
	if err != nil {
		app.logger.Error(err)
		return c.String(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, Verification{PublicId: tokenInfo.GetUserID()})
}

func (app *atesAuthSvc) token(c echo.Context) error {
	err := app.oauthServer.HandleTokenRequest(c.Response().Writer, c.Request())
	if err != nil {
		app.logger.Error(err)
	}
	return err
}

func (app *atesAuthSvc) authorize(c echo.Context) error {
	err := app.oauthServer.HandleAuthorizeRequest(c.Response().Writer, c.Request())
	if err != nil {
		app.logger.Error(err)
	}
	return err
}

func (app *atesAuthSvc) checkPassword(ctx context.Context, clientID, username, password string) (userID string, err error) {
	var userFromDb User
	result := app.userDb.First(&userFromDb, "login = ?", username)
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
