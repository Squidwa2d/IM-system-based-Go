package api

import (
	"database/sql"
	"errors"
	"net/http"

	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	token "github.com/Squidwa2d/IM-system-based-Go/token"
	"github.com/gin-gonic/gin"
)

/*

listConversations Api

*/

type listConversationsRequest struct {
	UserName string `json:"username" binding:"required"`
}

type listConversationsResponse struct {
	Conversations []db.Conversation `json:"conversations"`
}

func (server *Server) listConversations(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req listConversationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}
	if authPayload.Username != req.UserName {
		err := errors.New("account doesn't belong to the authenticated user")
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, err))
		return
	}
	user, err := server.store.GetUserByUsername(c, req.UserName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	conversations, err := server.store.ListMyConversations(c, user.ID)
	resq := listConversationsResponse{
		Conversations: conversations,
	}
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully get conversations",
		Data:    resq,
	})
}

/*
createConversation Api
*/
type createGroupeConversationRequest struct {
	UserName  string   `json:"username" binding:"required"`
	Target    []string `json:"target" binding:"required"`
	GroupName string   `json:"group_name" binding:"required"`
}

func (server *Server) createGroupeConversation(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req createGroupeConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	// check if the user is the owner of the account
	if authPayload.Username != req.UserName {
		err := errors.New("account doesn't belong to the authenticated user")
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, err))
		return
	}
	// check if the user exist
	owner, err := server.store.GetUserByUsername(c, req.UserName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	var users []int64
	for _, target := range req.Target {
		user, err := server.store.GetUserByUsername(c, target)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("target user not found")))
				return
			}
			c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
			return
		}
		users = append(users, user.ID)
	}

	arg := db.CreateGroupTxParams{
		OwnerID:   owner.ID,
		UserIDs:   users,
		GroupName: req.GroupName,
	}

	cm, err := server.store.CreateGroupTx(c, &arg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully create conversation",
		Data:    cm,
	})
}
