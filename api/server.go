package api

import (
	"fmt"

	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"

	token "github.com/Squidwa2d/IM-system-based-Go/token"
	"github.com/Squidwa2d/IM-system-based-Go/utils"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/minio/minio-go/v7"
	"log"
	"net/http"
	"time"
)

type Response struct {
	Code    int         `json:"code"`    // 业务状态码（非 HTTP 状态码）
	Message string      `json:"message"` // 提示信息
	Data    interface{} `json:"data"`    // 业务数据（成功时返回，失败时为 null）
}

type Server struct {
	hub         *Hub
	config      util.Config
	store       db.Store
	tokenMaker  token.Maker
	router      *gin.Engine
	redis       *RedisStore
	MinIOClient *minio.Client
}

func NewServer(config util.Config, store db.Store, rdb *RedisStore, minioClient *minio.Client) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}
	r := gin.Default()
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("device", validDevice)
	}
	hub := NewHub()
	server := &Server{
		config:      config,
		store:       store,
		tokenMaker:  tokenMaker,
		router:      r,
		hub:         hub,
		redis:       rdb,
		MinIOClient: minioClient,
	}
	server.setupRouter()
	return server, nil
}

func (s *Server) StartHTTP(address string) *http.Server {
	// 先启动 Hub，确保在接收请求前 Ready
	go s.hub.Run()
	log.Println("✅ Hub 已启动")

	srv := &http.Server{
		Addr:         address,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 在后台启动 HTTP 服务
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ HTTP 服务启动失败: %v", err)
		}
	}()

	return srv
}

func (server *Server) setupRouter() {
	v1 := server.router.Group("/api/v1")
	v1.POST("/auth/login", server.login)
	v1.POST("/auth/register", server.register)
	v1.GET("/auth/refresh", server.refreshToken)

	authRoutes := v1.Use(AuthMiddleware(server.tokenMaker))

	// 用户模块
	authRoutes.POST("/users/passwd", server.updatePassword)

	// 用户资料模块
	authRoutes.GET("/users/detail", server.getUserDetail)
	authRoutes.POST("/users/profile", server.updateUserProfile)
	authRoutes.POST("/users/avatar", server.updateUserAvatar)

	// 好友模块
	authRoutes.POST("/users/search", server.searchUsers)
	authRoutes.GET("/friends/requests", server.getFriendRequestList)
	authRoutes.GET("/friends/requests/count", server.getFriendRequestCount)
	authRoutes.POST("/friends/add", server.addFriend)
	authRoutes.POST("/friends/accept", server.acceptFriendRequest)
	authRoutes.POST("/friends/reject", server.rejectFriendRequest)
	authRoutes.POST("/friends/delete", server.deleteFriend)
	authRoutes.POST("/friends/list", server.getFriendList)
	authRoutes.POST("/friends/block", server.blockFriend)
	authRoutes.POST("/friends/unblock", server.unblockFriend)

	// 会话模块
	authRoutes.POST("/conversations/createGroupe", server.createGroupeConversation)
	authRoutes.POST("/conversations/listConversations", server.listConversations)
	authRoutes.POST("/conversations/createPrivate", server.createPrivateConversation)
	authRoutes.POST("/conversations/members", server.getConversationMembers)

	// 群组管理模块
	authRoutes.POST("/groups/invite", server.inviteGroupMember)
	authRoutes.POST("/groups/kick", server.kickGroupMember)
	authRoutes.POST("/groups/leave", server.leaveGroup)
	authRoutes.POST("/groups/update-info", server.updateGroupInfo)
	authRoutes.POST("/groups/transfer", server.transferGroupOwner)
	authRoutes.POST("/groups/announcement", server.updateGroupAnnouncement)
	authRoutes.POST("/groups/get-announcement", server.getGroupAnnouncement)
	authRoutes.POST("/groups/mute", server.muteGroupMember)
	authRoutes.POST("/groups/unmute", server.unmuteGroupMember)

	// 消息模块
	authRoutes.POST("/messages/uploadFile", server.uploadFile)
	authRoutes.POST("/messages/forward", server.forwardMessage)
	authRoutes.POST("/messages/forward-batch", server.forwardMultipleMessages)
	authRoutes.POST("/messages/recall", server.recallMessage)

	// WebSocket 连接
	authRoutes.GET("/ws/connect", server.handleWebSocket)
}

func (s *Server) StopHub() {
	log.Println("🛑 正在关闭 Hub...")
	s.hub.Stop() // 你需要在 hub.go 中实现这个方法来广播关闭信号并退出 Run 循环
}

func errorResponse(code int, err error) Response {
	return Response{
		Code:    code,
		Message: err.Error(),
		Data:    nil,
	}
}
