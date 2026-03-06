package api

import (
	"fmt"
	"net/http"
	"strings"

	token "github.com/Squidwa2d/IM-system-based-Go/token" // 替换为你的 token 包路径

	"github.com/gin-gonic/gin"
)

// 定义统一的响应结构体（前后端约定格式）

// 常量定义（可抽离到单独的常量文件）
const (
	authorizationHeaderKey  = "authorization"
	authorizationTypeBearer = "bearer"
	authorizationPayloadKey = "authorization_payload"
)

// 优化后的鉴权中间件
func AuthMiddleware(tokenMaker token.Maker) gin.HandlerFunc {
	return func(c *gin.Context) {
		var accessToken string

		// 1. 优先尝试从 Header 获取 (标准 HTTP 请求)
		authHeader := c.GetHeader(authorizationHeaderKey)
		if authHeader != "" {
			// 如果是 Header，预期格式为 "Bearer <token>"
			fields := strings.Fields(authHeader)
			if len(fields) != 2 || strings.ToLower(fields[0]) != authorizationTypeBearer {
				c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
					Code:    http.StatusUnauthorized,
					Message: "invalid authorization header format (expected: Bearer <token>)",
					Data:    nil,
				})
				return
			}
			accessToken = fields[1]
		} else {
			// 2. 如果 Header 没有，尝试从 Query 参数获取 (WebSocket 握手)
			tokenQuery := c.Query("token")
			if tokenQuery == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
					Code:    http.StatusUnauthorized,
					Message: "missing token (provide via 'Authorization' header or 'token' query param)",
					Data:    nil,
				})
				return
			}
			// Query 参数通常不需要 "Bearer " 前缀，直接赋值
			accessToken = tokenQuery
		}

		// 3. 校验 token 有效性
		if accessToken == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
				Code:    http.StatusUnauthorized,
				Message: "token is empty",
				Data:    nil,
			})
			return
		}

		payload, err := tokenMaker.VerifyToken(accessToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
				Code:    http.StatusUnauthorized,
				Message: fmt.Sprintf("invalid or expired token: %s", err.Error()),
				Data:    nil,
			})
			return
		}

		c.Set(authorizationPayloadKey, payload)
		c.Next()
	}
}
