package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"path/filepath"
	"time"

	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	token "github.com/Squidwa2d/IM-system-based-Go/token"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/minio/minio-go/v7"
)

type storeMessageParams struct {
	ConversationID int64
	SenderID       int64
	MsgType        int16
	Content        string
}

type storeMessageResult struct {
	ConversationID int64
	SenderID       int64
	MsgType        int16
	Content        string
	CreatedAt      pgtype.Timestamptz
}

func (s *Server) storeMessage(c context.Context, arg storeMessageParams) (msg storeMessageResult, err error) {
	message, err := s.store.CreateMessage(c, db.CreateMessageParams{
		ConversationID: arg.ConversationID,
		SenderID:       arg.SenderID,
		MsgType:        arg.MsgType,
		Content:        arg.Content,
	})
	if err != nil {
		return storeMessageResult{}, err
	}
	msg = storeMessageResult{
		ConversationID: message.ConversationID,
		SenderID:       message.SenderID,
		MsgType:        message.MsgType,
		Content:        message.Content,
		CreatedAt:      message.CreatedAt,
	}
	return msg, nil
}

type loadHistoryParams struct {
	ConversationID int64
	CursorID       pgtype.Int8
	Limit          int32
}

type loadHistoryResult struct {
	msgs []storeMessageResult
}

func geneMessageResult(msgs []db.Message) []storeMessageResult {
	var result []storeMessageResult
	for _, msg := range msgs {
		result = append(result, storeMessageResult{
			ConversationID: msg.ConversationID,
			SenderID:       msg.SenderID,
			MsgType:        msg.MsgType,
			Content:        msg.Content,
			CreatedAt:      msg.CreatedAt,
		})
	}
	return result
}

func (s *Server) loadHistory(c context.Context, arg loadHistoryParams) (msg loadHistoryResult, err error) {
	messages, err := s.store.ListHistoryMessages(c, db.ListHistoryMessagesParams{
		ConversationID: arg.ConversationID,
		CursorID:       arg.CursorID,
		Limit:          arg.Limit,
	})
	if err != nil {
		return loadHistoryResult{}, err
	}
	result := loadHistoryResult{
		msgs: geneMessageResult(messages),
	}
	return result, nil
}

type UploadFileRequest struct {
	ConversationID int64 `json:"conversation_id"`
	SenderID       int64 `json:"sender_id"`
	MsgType        int16 `json:"msg_type"`
}

var minIOBucket = "im-files"
var minIOEndpoint = "minio.wdnndw.cn"

func (s *Server) uploadFile(c *gin.Context) {
	// 1. 权限验证
	authPayload := c.MustGet(authorizationPayloadKey).(*token.Payload)

	jsonStr := c.PostForm("data")
	if jsonStr == "" {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("缺少 data 参数")))
		return
	}

	var req UploadFileRequest
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, fmt.Errorf("JSON 格式错误: %v", err)))
		return
	}

	// 获取用户并验证
	user, err := s.store.GetUserById(c, req.SenderID)
	if err != nil {
		c.JSON(http.StatusNotFound, errorResponse(http.StatusNotFound, errors.New("用户不存在")))
		return
	}

	// 验证发送者身份
	if user.Username != authPayload.Username {
		c.JSON(http.StatusUnauthorized, errorResponse(http.StatusUnauthorized, errors.New("用户名不匹配")))
		return
	}

	if req.MsgType == 1 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("消息类型不正确")))
		return
	}

	// 2. 获取文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("未找到上传文件")))
		return
	}

	// 3. 校验大小
	if file.Size > 100*1024*1024 {
		c.JSON(http.StatusBadRequest, errorResponse(http.StatusBadRequest, errors.New("文件过大")))
		return
	}

	// 4. 打开文件流
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, errors.New("无法读取文件")))
		return
	}
	defer src.Close() // 确保请求结束时关闭

	// 5. 生成 Object Key
	ext := filepath.Ext(file.Filename)
	datePath := time.Now().Format("2006/01/02")

	// 使用加密安全的随机数
	randInt, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, errors.New("生成随机数失败")))
		return
	}
	objectKey := fmt.Sprintf("%s/%d_%x%s", datePath, time.Now().UnixNano(), randInt, ext)

	// 6. 准备上传参数
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	bucketName := minIOBucket

	_, err = s.MinIOClient.PutObject(c.Request.Context(), minIOBucket, objectKey, src, file.Size, minio.PutObjectOptions{
		ContentType: contentType,
		// 如果需要公共读，可以通过存储桶策略设置，或者在这里设置 UserMetadata
	})

	if err != nil {
		fmt.Printf("MinIO Upload Error: %v\n", err)
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, fmt.Errorf("上传失败: %v", err)))
		return
	}

	// 7. 构造 URL (使用结构体中的 Endpoint)
	fileURL := fmt.Sprintf("https://%s/%s/%s", minIOEndpoint, bucketName, objectKey)

	// 8. 持久化存储到数据库
	message, err := s.store.CreateMessage(c, db.CreateMessageParams{
		ConversationID: req.ConversationID,
		SenderID:       req.SenderID,
		MsgType:        req.MsgType,
		Content:        fileURL,
	})
	if err != nil {
		fmt.Printf("DB Save Error: %v\n", err)
		c.JSON(http.StatusInternalServerError, errorResponse(http.StatusInternalServerError, fmt.Errorf("保存记录失败: %v", err)))
		return
	}

	// 9. 返回结果
	result := geneMessageResult([]db.Message{message})
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Message: "上传成功",
		Data:    result,
	})
}
