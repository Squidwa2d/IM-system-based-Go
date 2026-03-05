package api

import (
	"fmt"

	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	token "github.com/Squidwa2d/IM-system-based-Go/token"
	"github.com/Squidwa2d/IM-system-based-Go/utils"
	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int         `json:"code"`    // 业务状态码（非 HTTP 状态码）
	Message string      `json:"message"` // 提示信息
	Data    interface{} `json:"data"`    // 业务数据（成功时返回，失败时为 null）
}

type Server struct {
	config     util.Config
	store      db.Store
	tokenMaker token.Maker
	router     *gin.Engine
}

func NewServer(config util.Config, store db.Store) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}
	r := gin.Default()

	server := &Server{
		config:     config,
		store:      store,
		tokenMaker: tokenMaker,
		router:     r,
	}
	server.setupRouter()
	return server, nil
}

func (server *Server) Start(address string) error {
	return server.router.Run(address)
}

func (server *Server) setupRouter() {
	v1 := server.router.Group("/api/v1")
	v1.POST("/auth/login", server.login)
	v1.POST("/auth/register", server.register)
	v1.GET("auth/refresh", server.refreshToken)

	authRoutes := v1.Use(AuthMiddleware(server.tokenMaker))
	authRoutes.POST("/users/passwd", server.updatePassword)

	authRoutes.POST("/conversations/createGroupe", server.createGroupeConversation)
	authRoutes.POST("/conversations/listConversations", server.listConversations)
}

func errorResponse(code int, err error) Response {
	return Response{
		Code:    code,
		Message: err.Error(),
		Data:    nil,
	}
}
