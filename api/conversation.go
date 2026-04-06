package api

import (
	"database/sql"
	"errors"
	"net/http"

	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	token "github.com/Squidwa2d/IM-system-based-Go/token"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

/*

listConversations Api

*/

type listConversationsRequest struct {
	UserName string `json:"username" binding:"required"`
}

type ConversationResponse struct {
	ID        int64              `json:"id"`
	Type      int16              `json:"type"`
	Name      interface{}        `json:"name"`
	AvatarUrl interface{}        `json:"avatar_url"`
	OwnerID   interface{}        `json:"owner_id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
}

type listConversationsResponse struct {
	Conversations []ConversationResponse `json:"conversations"`
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

	// 转换 conversation 数据格式
	var convResponses []ConversationResponse
	for _, conv := range conversations {
		resp := ConversationResponse{
			ID:        conv.ID,
			Type:      conv.Type,
			Name:      nil,
			AvatarUrl: nil,
			OwnerID:   nil,
			CreatedAt: conv.CreatedAt,
			UpdatedAt: conv.UpdatedAt,
		}

		// 处理可选字段
		if conv.Name.Valid {
			resp.Name = conv.Name.String
		}
		if conv.AvatarUrl.Valid {
			resp.AvatarUrl = conv.AvatarUrl.String
		}
		if conv.OwnerID.Valid {
			resp.OwnerID = conv.OwnerID.Int64
		}

		convResponses = append(convResponses, resp)
	}

	resq := listConversationsResponse{
		Conversations: convResponses,
	}
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully get conversations",
		Data:    resq,
	})
}

/*
Get Conversation Members Api
*/

type GetConversationMembersRequest struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

func (server *Server) getConversationMembers(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req GetConversationMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := server.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 检查用户是否为群成员
	isMember, err := server.store.CheckMemberExists(c, db.CheckMemberExistsParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	if !isMember {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("you are not a member of this conversation")))
		return
	}

	members, err := server.store.ListConversationMembers(c, req.ConversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully get conversation members",
		Data:    members,
	})
}

/*
create private conversation Api
重构：增加好友关系验证，只有好友间才能创建私聊

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
		Message: "Successfully create group conversation",
		Data:    cm,
	})
}

/*

create private conversation Api
重构：增加好友关系验证，只有好友间才能创建私聊

*/

type createPrivateConversationRequest struct {
	UserName string `json:"username" binding:"required"`
	Target   string `json:"target" binding:"required"`
}

func (server *Server) createPrivateConversation(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req createPrivateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	// check if the user is the owner of the account
	if authPayload.Username != req.UserName {
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, errors.New("account doesn't belong to the authenticated user")))
		return
	}

	// check if the user exist
	user, err := server.store.GetUserByUsername(c, req.UserName)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	//check if the target user exist
	target, err := server.store.GetUserByUsername(c, req.Target)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("target user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 验证好友关系：只有好友才能创建私聊会话
	isFriend, err := server.store.CheckFriendshipExists(c, db.CheckFriendshipExistsParams{
		UserID:   user.ID,
		FriendID: target.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	if !isFriend {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("can only create private conversation with friends")))
		return
	}

	// 检查是否已存在私聊会话
	existingConv, err := server.store.GetPrivateConversation(c, db.GetPrivateConversationParams{
		UserID:   user.ID,
		UserID_2: target.ID,
	})
	if err == nil {
		// 如果已存在，直接返回
		c.JSON(http.StatusOK, Response{
			Code:    http.StatusOK,
			Message: "Private conversation already exists",
			Data:    existingConv,
		})
		return
	}

	// 创建私聊会话
	cm, err := server.store.CreatePrivateTx(c, &db.CreatePrivateTxParams{UserId: user.ID, FriendId: target.ID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully create private conversation",
		Data:    cm,
	})

}
