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
Get Friend Request Count API
*/
type GetFriendRequestCountResponse struct {
	Count int64 `json:"count"`
}

func (s *Server) getFriendRequestCount(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	requests, err := s.store.GetFriendRequestList(c, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully get friend request count",
		Data: GetFriendRequestCountResponse{
			Count: int64(len(requests)),
		},
	})
}

/*
Search Users API
*/
type SearchUsersRequest struct {
	Keyword  string `json:"keyword" binding:"required"`
	Page     int32  `json:"page" binding:"min=1"`
	PageSize int32  `json:"page_size" binding:"min=1,max=50"`
}

type SearchUsersResponse struct {
	Users []db.User `json:"users"`
	Total int64     `json:"total"`
}

func (s *Server) searchUsers(c *gin.Context) {

	var req SearchUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	// 设置默认分页参数
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	offset := (req.Page - 1) * req.PageSize

	searchPattern := "%" + req.Keyword + "%"
	// 搜索用户
	users, err := s.store.SearchUsers(c, db.SearchUsersParams{
		Username: searchPattern,
		Limit:    req.PageSize,
		Offset:   offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 获取总数
	total, err := s.store.SearchUsersCount(c, searchPattern)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 转换为 User 数组
	userList := make([]db.User, len(users))
	for i, u := range users {
		userList[i] = db.User{
			ID:         u.ID,
			Username:   u.Username,
			PasswdHash: "",
			AvatarUrl:  u.AvatarUrl,
			Status:     u.Status,
			CreatedAt:  u.CreatedAt,
			UpdatedAt:  u.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully searched users",
		Data: SearchUsersResponse{
			Users: userList,
			Total: total,
		},
	})
}

/*
Add Friend API
*/
type AddFriendRequest struct {
	TargetUsername string `json:"target_username" binding:"required"`
}

func (s *Server) addFriend(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req AddFriendRequest
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

	// 不能添加自己为好友
	if user.ID == target.ID {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("cannot add yourself as friend")))
		return
	}

	// 检查好友关系是否已存在
	isFriend, err := s.store.CheckFriendshipExists(c, db.CheckFriendshipExistsParams{
		UserID:   user.ID,
		FriendID: target.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	if isFriend {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("already friends")))
		return
	}

	// 创建好友请求
	friendship, err := s.store.CreateFriendship(c, db.CreateFriendshipParams{
		UserID:   user.ID,
		FriendID: target.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Friend request sent successfully",
		Data:    friendship,
	})
}

/*
Get Friend Request List API
*/
type GetFriendRequestListResponse struct {
	Requests []db.GetFriendRequestListRow `json:"requests"`
}

func (s *Server) getFriendRequestList(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	requests, err := s.store.GetFriendRequestList(c, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully get friend request list",
		Data: GetFriendRequestListResponse{
			Requests: requests,
		},
	})
}

/*
Accept Friend Request API
*/
type AcceptFriendRequest struct {
	RequesterUsername string `json:"requester_username" binding:"required"`
}

func (s *Server) acceptFriendRequest(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req AcceptFriendRequest
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

	// 获取请求者
	requester, err := s.store.GetUserByUsername(c, req.RequesterUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("requester not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 接受好友请求
	err = s.store.AcceptFriendship(c, db.AcceptFriendshipParams{
		UserID:   requester.ID,
		FriendID: user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	// 检查是否已存在私聊会话
	_, err = s.store.GetPrivateConversation(c, db.GetPrivateConversationParams{
		UserID:   user.ID,
		UserID_2: requester.ID,
	})
	if err != nil {
		// 如果不存在，创建私聊会话
		_, err = s.store.CreatePrivateTx(c, &db.CreatePrivateTxParams{UserId: user.ID, FriendId: requester.ID})
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
			return
		}
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Friend request accepted",
		Data:    nil,
	})
}

/*
Reject Friend Request API
*/
type RejectFriendRequest struct {
	RequesterUsername string `json:"requester_username" binding:"required"`
}

func (s *Server) rejectFriendRequest(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req RejectFriendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	requester, err := s.store.GetUserByUsername(c, req.RequesterUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("requester not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	err = s.store.RejectFriendship(c, db.RejectFriendshipParams{
		UserID:   requester.ID,
		FriendID: user.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Friend request rejected",
		Data:    nil,
	})
}

/*
Delete Friend API
*/
type DeleteFriendRequest struct {
	FriendUsername string `json:"friend_username" binding:"required"`
}

func (s *Server) deleteFriend(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req DeleteFriendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	friend, err := s.store.GetUserByUsername(c, req.FriendUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("friend not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	err = s.store.DeleteFriendship(c, db.DeleteFriendshipParams{
		UserID:   user.ID,
		FriendID: friend.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Friend deleted successfully",
		Data:    nil,
	})
}

/*
Get Friend List API
*/
type GetFriendListRequest struct {
	Page     int32 `json:"page" binding:"min=1"`
	PageSize int32 `json:"page_size" binding:"min=1,max=50"`
}

type GetFriendListResponse struct {
	Friends []db.GetFriendListRow `json:"friends"`
	Total   int64                 `json:"total"`
}

func (s *Server) getFriendList(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req GetFriendListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 {
		req.PageSize = 20
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	offset := (req.Page - 1) * req.PageSize

	friends, err := s.store.GetFriendList(c, db.GetFriendListParams{
		UserID: user.ID,
		Limit:  req.PageSize,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Successfully get friend list",
		Data: GetFriendListResponse{
			Friends: friends,
			Total:   int64(len(friends)),
		},
	})
}

/*
Block Friend API
*/
type BlockFriendRequest struct {
	FriendUsername string `json:"friend_username" binding:"required"`
}

func (s *Server) blockFriend(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req BlockFriendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	friend, err := s.store.GetUserByUsername(c, req.FriendUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("friend not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	err = s.store.BlockFriend(c, db.BlockFriendParams{
		UserID:   user.ID,
		FriendID: friend.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Friend blocked successfully",
		Data:    nil,
	})
}

/*
Unblock Friend API
*/
type UnblockFriendRequest struct {
	FriendUsername string `json:"friend_username" binding:"required"`
}

func (s *Server) unblockFriend(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	var req UnblockFriendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, err))
		return
	}

	user, err := s.store.GetUserByUsername(c, authPayload.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	friend, err := s.store.GetUserByUsername(c, req.FriendUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("friend not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	err = s.store.UnblockFriend(c, db.UnblockFriendParams{
		UserID:   user.ID,
		FriendID: friend.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "Friend unblocked successfully",
		Data:    nil,
	})
}
