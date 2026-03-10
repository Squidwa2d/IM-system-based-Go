package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	token "github.com/Squidwa2d/IM-system-based-Go/token"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleWebSocket(c *gin.Context) {
	// 1. 身份验证
	authPayload, exists := c.Get(authorizationPayloadKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, errors.New("unauthorized")))
		return
	}
	payload := authPayload.(*token.Payload)

	// 2. 获取用户信息
	user, err := s.store.GetUserByUsername(c, payload.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	userID := user.ID

	// 3.获取 DeviceID
	// 优先从 Query 参数获取，其次从 Header 获取
	deviceID := c.Query("device")

	deviceID = strings.ToUpper(deviceID)
	if deviceID == "" {
		fmt.Printf("❌ 未提供 device_id 参数\n")
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("device_id 参数缺失")))
	}

	if deviceID != "PC" && deviceID != "MOBILE" {
		fmt.Printf("❌ device 参数无效\n")
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("device_id 参数无效")))
	}
	// 4.获取该用户的所有会话ID
	conversationIDs, err := s.store.GetUserAllConversations(c, userID)
	if err != nil {
		log.Printf("❌ 获取用户 %s 会话列表失败: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, errors.New("failed to load conversations")))
		return
	}

	// 5. 升级协议
	conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade failed for %s: %v", user.Username, err)
		return
	}

	// 6. 创建 Client
	client := &Client{
		Hub:             s.hub,
		Conn:            conn,
		Send:            make(chan []byte, 256), // 缓冲大小适中
		UserID:          userID,
		DeviceID:        deviceID,
		ConversationsID: conversationIDs,
		Srv:             s,
	}

	log.Printf("🔗 WebSocket 连接建立: User=%s, Device=%s, IP=%s", user.Username, deviceID, c.ClientIP())

	// 7. 注册到 Hub
	s.hub.register <- client

	// 8. 启动读写协程
	go client.readPump()
	go client.writePump()

}
