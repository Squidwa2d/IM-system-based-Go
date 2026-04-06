package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	token "github.com/Squidwa2d/IM-system-based-Go/token"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

/*
Get User Detail API
*/
type GetUserDetailRequest struct {
	UserID int64 `json:"user_id" binding:"required" form:"user_id"`
}

type GetUserDetailResponse struct {
	db.GetUserDetailRow
}

func (s *Server) getUserDetail(c *gin.Context) {
	var req GetUserDetailRequest
	// 先尝试从 JSON body 获取参数，如果失败则从 query 参数获取
	if err := c.ShouldBindQuery(&req); err != nil {
		// 如果 query 参数绑定失败，尝试 JSON body
		if err2 := c.ShouldBindJSON(&req); err2 != nil {
			c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err2))
			return
		}
	}

	// 获取用户详情
	userDetail, err := s.store.GetUserDetail(c, req.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully get user detail",
		Data: GetUserDetailResponse{
			GetUserDetailRow: userDetail,
		},
	})
}

/*
Update User Profile API
*/
type UpdateUserProfileRequest struct {
	Nickname  string `json:"nickname"`
	Signature string `json:"signature"`
	Gender    string `json:"gender"`
	Birthday  string `json:"birthday"` // 格式：YYYY-MM-DD
}

func (s *Server) updateUserProfile(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req UpdateUserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	// 获取当前用户
	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 准备更新参数
	var nickname pgtype.Text
	if req.Nickname != "" {
		nickname = pgtype.Text{
			String: req.Nickname,
			Valid:  true,
		}
	}

	var signature pgtype.Text
	if req.Signature != "" {
		signature = pgtype.Text{
			String: req.Signature,
			Valid:  true,
		}
	}

	var gender pgtype.Text
	if req.Gender != "" {
		gender = pgtype.Text{
			String: req.Gender,
			Valid:  true,
		}
	}

	var birthday pgtype.Date
	if req.Birthday != "" {
		t, err := time.Parse("2006-01-02", req.Birthday)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("invalid birthday format, use YYYY-MM-DD")))
			return
		}
		birthday = pgtype.Date{
			Time:  t,
			Valid: true,
		}
	}

	// 更新用户资料
	profile, err := s.store.UpdateUserProfile(c, db.UpdateUserProfileParams{
		UserID:    user.ID,
		Nickname:  nickname,
		Signature: signature,
		Gender:    gender,
		Birthday:  birthday,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Profile updated successfully",
		Data:    profile,
	})
}

/*
Update User Avatar API
*/
type UpdateUserAvatarRequest struct {
	AvatarUrl string `json:"avatar_url" binding:"required"`
}

func (s *Server) updateUserAvatar(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req UpdateUserAvatarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 更新头像
	updatedUser, err := s.store.UpdateAvatar(c, db.UpdateAvatarParams{
		ID: user.ID,
		AvatarUrl: pgtype.Text{
			String: req.AvatarUrl,
			Valid:  true,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Avatar updated successfully",
		Data:    updatedUser,
	})
}

/*
Recall Message API (消息撤回)
*/
type RecallMessageRequest struct {
	MessageID int64 `json:"message_id" binding:"required"`
}

func (s *Server) recallMessage(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req RecallMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	recallMsg, err := s.store.RecallMessage(c, db.RecallMessageParams{
		ID:       req.MessageID,
		SenderID: user.ID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("message not found or cannot be recalled")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Message recalled successfully",
		Data:    recallMsg,
	})
}

/*
Forward Message API
*/
type ForwardMessageRequest struct {
	MessageID            int64 `json:"message_id" binding:"required"`
	TargetConversationID int64 `json:"target_conversation_id" binding:"required"`
}

func (s *Server) forwardMessage(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req ForwardMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	// 获取当前用户
	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 获取原消息
	originalMsg, err := s.store.GetMessage(c, req.MessageID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("message not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 验证目标会话是否存在
	_, err = s.store.GetConversation(c, req.TargetConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("target conversation not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 检查用户是否是目标会话的成员
	isMember, err := s.store.CheckMemberExists(c, db.CheckMemberExistsParams{
		ConversationID: req.TargetConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	if !isMember {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("not a member of target conversation")))
		return
	}

	// 创建转发消息
	forwardedMsg, err := s.store.CreateMessage(c, db.CreateMessageParams{
		ConversationID: req.TargetConversationID,
		SenderID:       user.ID,
		MsgType:        originalMsg.MsgType,
		Content:        originalMsg.Content,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 记录转发
	forwardRecord, err := s.store.CreateMessageForward(c, db.CreateMessageForwardParams{
		OriginalMessageID:    req.MessageID,
		ForwardedBy:          user.ID,
		TargetConversationID: req.TargetConversationID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 准备广播数据
	broadcastMsg := BroadcastMessage{
		ConversationId: req.TargetConversationID,
		MsgType:        int(forwardedMsg.MsgType),
		Content:        forwardedMsg.Content,
		SenderID:       user.ID,
		SendTime:       forwardedMsg.CreatedAt,
	}

	// 查找目标会话中的在线用户并发送
	s.hub.mu.Lock()
	targetClients := make([]*Client, 0)
	if room, ok := s.hub.rooms[req.TargetConversationID]; ok {
		for client, status := range room {
			if status == true && client.UserID != user.ID {
				targetClients = append(targetClients, client)
			}
		}
	}
	s.hub.mu.Unlock()

	if len(targetClients) > 0 {
		s.hub.SendMessageToUsers(targetClients, []BroadcastMessage{broadcastMsg})
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Message forwarded successfully",
		Data: map[string]interface{}{
			"message":        forwardedMsg,
			"forward_record": forwardRecord,
		},
	})
}

/*
Forward Multiple Messages API
*/
type ForwardMultipleMessagesRequest struct {
	MessageIDs           []int64 `json:"message_ids" binding:"required,min=1"`
	TargetConversationID int64   `json:"target_conversation_id" binding:"required"`
}

func (s *Server) forwardMultipleMessages(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req ForwardMultipleMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 验证目标会话
	_, err = s.store.GetConversation(c, req.TargetConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("target conversation not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	isMember, err := s.store.CheckMemberExists(c, db.CheckMemberExistsParams{
		ConversationID: req.TargetConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	if !isMember {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("not a member of target conversation")))
		return
	}

	// 批量转发消息
	forwardedMessages := make([]db.Message, 0)
	forwardRecords := make([]db.MessageForward, 0)

	for _, msgID := range req.MessageIDs {
		// 获取原消息
		originalMsg, err := s.store.GetMessage(c, msgID)
		if err != nil {
			continue // 跳过不存在的消息
		}

		// 创建转发消息
		forwardedMsg, err := s.store.CreateMessage(c, db.CreateMessageParams{
			ConversationID: req.TargetConversationID,
			SenderID:       user.ID,
			MsgType:        originalMsg.MsgType,
			Content:        originalMsg.Content,
		})
		if err != nil {
			continue
		}

		// 记录转发
		forwardRecord, err := s.store.CreateMessageForward(c, db.CreateMessageForwardParams{
			OriginalMessageID:    msgID,
			ForwardedBy:          user.ID,
			TargetConversationID: req.TargetConversationID,
		})
		if err != nil {
			continue
		}

		forwardedMessages = append(forwardedMessages, forwardedMsg)
		forwardRecords = append(forwardRecords, forwardRecord)

		// 广播消息
		broadcastMsg := BroadcastMessage{
			ConversationId: req.TargetConversationID,
			MsgType:        int(forwardedMsg.MsgType),
			Content:        forwardedMsg.Content,
			SenderID:       user.ID,
			SendTime:       forwardedMsg.CreatedAt,
		}

		s.hub.mu.Lock()
		targetClients := make([]*Client, 0)
		if room, ok := s.hub.rooms[req.TargetConversationID]; ok {
			for client, status := range room {
				if status == true && client.UserID != user.ID {
					targetClients = append(targetClients, client)
				}
			}
		}
		s.hub.mu.Unlock()

		if len(targetClients) > 0 {
			s.hub.SendMessageToUsers(targetClients, []BroadcastMessage{broadcastMsg})
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Messages forwarded successfully",
		Data: map[string]interface{}{
			"forwarded_count": len(forwardedMessages),
			"messages":        forwardedMessages,
			"forward_records": forwardRecords,
		},
	})
}
