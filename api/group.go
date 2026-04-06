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
Invite Group Member API
*/
type InviteGroupMemberRequest struct {
	ConversationID int64  `json:"conversation_id" binding:"required"`
	TargetUsername string `json:"target_username" binding:"required"`
}

func (s *Server) inviteGroupMember(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req InviteGroupMemberRequest
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

	// 检查群组是否存在
	group, err := s.store.GetConversation(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("group not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 验证是否为群组
	if group.Type != 2 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("not a group conversation")))
		return
	}

	// 检查当前用户是否为群主或管理员
	member, err := s.store.GetConversationMember(c, db.GetConversationMemberParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if member.Role != 1 && member.Role != 2 {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("only owner or admin can invite members")))
		return
	}

	// 获取目标用户
	target, err := s.store.GetUserByUsername(c, req.TargetUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("target user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 检查是否已是群成员
	exists, err := s.store.CheckMemberExists(c, db.CheckMemberExistsParams{
		ConversationID: req.ConversationID,
		UserID:         target.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("user already in group")))
		return
	}

	// 邀请进群
	memberRecord, err := s.store.InviteGroupMember(c, db.InviteGroupMemberParams{
		ConversationID: req.ConversationID,
		UserID:         target.ID,
		Role:           3, // 普通成员
		LastReadMessageID: pgtype.Int8{
			Valid: false,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Member invited successfully",
		Data:    memberRecord,
	})
}

/*
Kick Group Member API
*/
type KickGroupMemberRequest struct {
	ConversationID int64  `json:"conversation_id" binding:"required"`
	TargetUsername string `json:"target_username" binding:"required"`
}

func (s *Server) kickGroupMember(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req KickGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	group, err := s.store.GetConversation(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("group not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if group.Type != 2 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("not a group conversation")))
		return
	}

	// 检查当前用户权限
	member, err := s.store.GetConversationMember(c, db.GetConversationMemberParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if member.Role != 1 {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("only owner can kick members")))
		return
	}

	target, err := s.store.GetUserByUsername(c, req.TargetUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("target user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 不能踢群主
	if target.ID == group.OwnerID.Int64 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("cannot kick group owner")))
		return
	}

	err = s.store.KickGroupMember(c, db.KickGroupMemberParams{
		ConversationID: req.ConversationID,
		UserID:         target.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Member kicked successfully",
		Data:    nil,
	})
}

/*
Leave Group API
*/
type LeaveGroupRequest struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

func (s *Server) leaveGroup(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req LeaveGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	group, err := s.store.GetConversation(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("group not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if group.Type != 2 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("not a group conversation")))
		return
	}

	// 群主不能退出群，只能转让或解散
	member, err := s.store.GetConversationMember(c, db.GetConversationMemberParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if member.Role == 1 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("group owner cannot leave group, please transfer ownership first")))
		return
	}

	err = s.store.LeaveGroup(c, db.LeaveGroupParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully left group",
		Data:    nil,
	})
}

/*
Update Group Info API
*/
type UpdateGroupInfoRequest struct {
	ConversationID int64  `json:"conversation_id" binding:"required"`
	GroupName      string `json:"group_name"`
	AvatarUrl      string `json:"avatar_url"`
}

func (s *Server) updateGroupInfo(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req UpdateGroupInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	group, err := s.store.GetConversation(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("group not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if group.Type != 2 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("not a group conversation")))
		return
	}

	// 检查权限
	member, err := s.store.GetConversationMember(c, db.GetConversationMemberParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if member.Role != 1 && member.Role != 2 {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("only owner or admin can update group info")))
		return
	}

	// 更新群组信息
	var name pgtype.Text
	if req.GroupName != "" {
		name = pgtype.Text{
			String: req.GroupName,
			Valid:  true,
		}
	} else {
		name = group.Name
	}

	var avatar pgtype.Text
	if req.AvatarUrl != "" {
		avatar = pgtype.Text{
			String: req.AvatarUrl,
			Valid:  true,
		}
	} else {
		avatar = group.AvatarUrl
	}

	updatedGroup, err := s.store.UpdateGroupInfo(c, db.UpdateGroupInfoParams{
		ID:        req.ConversationID,
		Name:      name,
		AvatarUrl: avatar,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Group info updated successfully",
		Data:    updatedGroup,
	})
}

/*
Transfer Group Owner API
*/
type TransferGroupOwnerRequest struct {
	ConversationID   int64  `json:"conversation_id" binding:"required"`
	NewOwnerUsername string `json:"new_owner_username" binding:"required"`
}

func (s *Server) transferGroupOwner(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req TransferGroupOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	group, err := s.store.GetConversation(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("group not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if group.Type != 2 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("not a group conversation")))
		return
	}

	// 只有群主可以转让
	if group.OwnerID.Int64 != user.ID {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("only group owner can transfer ownership")))
		return
	}

	// 获取新群主
	newOwner, err := s.store.GetUserByUsername(c, req.NewOwnerUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("new owner not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 检查新群主是否是群成员
	isMember, err := s.store.CheckMemberExists(c, db.CheckMemberExistsParams{
		ConversationID: req.ConversationID,
		UserID:         newOwner.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	if !isMember {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("new owner must be a group member")))
		return
	}

	// 转让群主 - 更新旧群主角色为普通成员
	err = s.store.TransferGroupOwner(c, db.TransferGroupOwnerParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,     // 旧群主
		UserID_2:       newOwner.ID, // 新群主
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 更新旧群主的成员角色为普通成员
	err = s.store.UpdateMemberRole(c, db.UpdateMemberRoleParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
		Role:           3, // 普通成员
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Group ownership transferred successfully",
		Data:    nil,
	})
}

/*
Update Group Announcement API
*/
type UpdateGroupAnnouncementRequest struct {
	ConversationID int64  `json:"conversation_id" binding:"required"`
	Content        string `json:"content" binding:"required"`
}

func (s *Server) updateGroupAnnouncement(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req UpdateGroupAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	group, err := s.store.GetConversation(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("group not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if group.Type != 2 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("not a group conversation")))
		return
	}

	// 检查权限
	member, err := s.store.GetConversationMember(c, db.GetConversationMemberParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if member.Role != 1 && member.Role != 2 {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("only owner or admin can update announcement")))
		return
	}

	// 检查是否已有公告
	announcement, err := s.store.GetGroupAnnouncement(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			// 创建新公告
			announcement, err = s.store.CreateGroupAnnouncement(c, db.CreateGroupAnnouncementParams{
				ConversationID: req.ConversationID,
				PublisherID:    user.ID,
				Content:        req.Content,
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
				return
			}
		} else {
			c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
			return
		}
	} else {
		// 更新公告
		announcement, err = s.store.UpdateGroupAnnouncement(c, db.UpdateGroupAnnouncementParams{
			ConversationID: req.ConversationID,
			PublisherID:    user.ID,
			Content:        req.Content,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
			return
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Group announcement updated successfully",
		Data:    announcement,
	})
}

/*
Get Group Announcement API
*/
type GetGroupAnnouncementRequest struct {
	ConversationID int64 `json:"conversation_id" binding:"required"`
}

func (s *Server) getGroupAnnouncement(c *gin.Context) {
	var req GetGroupAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	announcement, err := s.store.GetGroupAnnouncement(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, Response{
				Code:    http.StatusOK,
				Message: "No announcement found",
				Data:    nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully get group announcement",
		Data:    announcement,
	})
}

/*
Mute Group Member API
*/
type MuteGroupMemberRequest struct {
	ConversationID int64  `json:"conversation_id" binding:"required"`
	TargetUsername string `json:"target_username" binding:"required"`
	MuteDuration   int    `json:"mute_duration" binding:"required,min=1"` // minutes
}

func (s *Server) muteGroupMember(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req MuteGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	group, err := s.store.GetConversation(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("group not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if group.Type != 2 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("not a group conversation")))
		return
	}

	member, err := s.store.GetConversationMember(c, db.GetConversationMemberParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if member.Role != 1 && member.Role != 2 {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("only owner or admin can mute members")))
		return
	}

	target, err := s.store.GetUserByUsername(c, req.TargetUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("target user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	mutedUntil := time.Now().Add(time.Duration(req.MuteDuration) * time.Minute)
	muteRecord, err := s.store.MuteGroupMember(c, db.MuteGroupMemberParams{
		ConversationID: req.ConversationID,
		UserID:         target.ID,
		MutedUntil: pgtype.Timestamptz{
			Time:  mutedUntil,
			Valid: true,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Member muted successfully",
		Data:    muteRecord,
	})
}

/*
Unmute Group Member API
*/
type UnmuteGroupMemberRequest struct {
	ConversationID int64  `json:"conversation_id" binding:"required"`
	TargetUsername string `json:"target_username" binding:"required"`
}

func (s *Server) unmuteGroupMember(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req UnmuteGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	group, err := s.store.GetConversation(c, req.ConversationID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("group not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if group.Type != 2 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("not a group conversation")))
		return
	}

	member, err := s.store.GetConversationMember(c, db.GetConversationMemberParams{
		ConversationID: req.ConversationID,
		UserID:         user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	if member.Role != 1 && member.Role != 2 {
		c.JSON(http.StatusForbidden, errorResponse(http.StatusForbidden, errors.New("only owner or admin can unmute members")))
		return
	}

	target, err := s.store.GetUserByUsername(c, req.TargetUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("target user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	err = s.store.UnmuteGroupMember(c, db.UnmuteGroupMemberParams{
		ConversationID: req.ConversationID,
		UserID:         target.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Member unmuted successfully",
		Data:    nil,
	})
}
