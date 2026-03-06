package api

import (
	"database/sql"
	"errors"
	token "github.com/Squidwa2d/IM-system-based-Go/token"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

func (s *Server) handleWebSocket(c *gin.Context) {
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)
	userName := authPayload.Username
	user, err := s.store.GetUserByUsername(c, userName)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("target user not found")))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, err))
		return
	}
	log.Printf("success get user: %s\n", user.Username)
	userID := user.ID
	// 获取 DeviceID (前端传参 ?device_id=iphone_123)
	deviceID := c.Request.UserAgent()
	if deviceID == "" {
		deviceID = "unknown_device_" + time.Now().Format("20060102150405") // 临时生成
	}
	log.Printf("success get device: %s\n", deviceID)
	conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		Hub:      s.hub,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		UserID:   userID,
		DeviceID: deviceID,
	}

	s.hub.register <- client // 注册

	go client.writePump()
	go client.readPump()
}
