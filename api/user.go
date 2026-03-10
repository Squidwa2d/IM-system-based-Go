package api

import (
	"errors"
	"net/http"

	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	token "github.com/Squidwa2d/IM-system-based-Go/token"
	util "github.com/Squidwa2d/IM-system-based-Go/utils"
	"github.com/gin-gonic/gin"
)

/*

updatePasswd api

*/

type UpdateUserRequest struct {
	Username  string `json:"username" binding:"required"`
	NewPasswd string `json:"new_passwd"`
	Passwd    string `json:"passwd" binding:"required"`
}

func (s *Server) updatePassword(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	// check if the username is the same as the one in the token
	if req.Username != authPayload.Username {
		err := errors.New("account doesn't belong to the authenticated user")
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, err))
		return
	}

	// check if the password is the same
	if req.Passwd == req.NewPasswd {
		err := errors.New("new password cannot be the same as the old one")
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	// check if the user exists
	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// check if the password is correct
	err = util.CheckPasswordHash(req.Passwd, user.PasswdHash)
	if err != nil {
		err := errors.New("incorrect password")
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, err))
		return
	}

	// update the password
	hashedPasswd, err := util.HashPassword(req.NewPasswd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	arg := db.UpdatePasswdParams{
		ID:         user.ID,
		PasswdHash: hashedPasswd,
	}

	_, err = s.store.UpdatePasswd(c, arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "password updated successfully",
		Data:    nil,
	})
}
