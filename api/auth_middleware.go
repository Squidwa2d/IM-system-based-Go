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
		authorizationHeader := c.GetHeader(authorizationHeaderKey)
		if authorizationHeader == "" {
			// 封装错误响应：Code=401，Message=具体错误，Data=null
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
				Code:    http.StatusUnauthorized,
				Message: "authorization header is not provided",
				Data:    nil,
			})
			return
		}

		// Bearer token 拆分
		fields := strings.Fields(authorizationHeader)
		if len(fields) < 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
				Code:    http.StatusUnauthorized,
				Message: "invalid authorization header format (expected: Bearer <token>)",
				Data:    nil,
			})
			return
		}

		// 校验授权类型
		authType := strings.ToLower(fields[0])
		if authType != authorizationTypeBearer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
				Code:    http.StatusUnauthorized,
				Message: fmt.Sprintf("unsupported authorization type %s (only Bearer is supported)", authType),
				Data:    nil,
			})
			return
		}

		// 校验 token 有效性
		accessToken := fields[1]
		payload, err := tokenMaker.VerifyToken(accessToken)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{
				Code:    http.StatusUnauthorized,
				Message: fmt.Sprintf("invalid or expired token: %s", err.Error()),
				Data:    nil,
			})
			return
		}

		// 正常流程：将 payload 存入上下文，继续执行后续处理
		c.Set(authorizationPayloadKey, payload)
		c.Next()
	}
}
