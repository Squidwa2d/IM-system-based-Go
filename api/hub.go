package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"context"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	Hub             *Hub
	Conn            *websocket.Conn
	Send            chan []byte
	ConversationsID []int64
	UserID          int64
	DeviceID        string
	Srv             *Server
}

type MessageInput struct {
	ConversationId int64  `json:"conversation_id"`
	MsgType        int    `json:"msg_type"` // 1:text, 2:image, 3:file, 4:voice
	Content        string `json:"content"`
	SenderID       int64  `json:"sender_id"`
}

type BroadcastMessage struct {
	ConversationId int64              `json:"conversation_id"`
	MsgType        int                `json:"msg_type"` //1:text, 2:image, 3:file, 4:voice
	Content        interface{}        `json:"content"`
	SenderID       int64              `json:"sender_id"`
	SendTime       pgtype.Timestamptz `json:"send_time"`
}

type Hub struct {
	mu         sync.Mutex
	clients    map[int64]map[string]*Client
	broadcast  chan *DispatchTask
	register   chan *Client
	unregister chan *Client
	stop       chan struct{}
	rooms      map[int64]map[*Client]bool //0:在线但是没订阅,1:在线并且订阅
}

type DispatchTask struct {
	TargetClients []*Client          // 接收者列表 (单聊是1个，群聊是N个)
	Message       []BroadcastMessage // 消息内容
}

type ControlMessage struct {
	Action string `json:"action"` // "join_room", "leave_room"
	RoomID int64  `json:"room_id"`
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {

		MessageType, MessageBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket unexpected close error: %v\n", err)
			} else {
				fmt.Printf("WebSocket closed: %v\n", err)
			}
			break
		}

		if MessageType != websocket.TextMessage {
			fmt.Printf("WebSocket received non-text message type: %v\n", MessageType)
			continue
		}

		go c.handleMessage(MessageBytes)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// ✅ 使用标签退出外层循环
		batchLoop:
			for {
				select {
				case nextMsg := <-c.Send:
					w.Write([]byte{'\n'})
					w.Write(nextMsg)
				default:
					break batchLoop // 明确退出外层 for 循环
				}
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msgBytes []byte) {

	// 1. 先尝试解析为通用 map ，判断类型
	var rawMsg map[string]interface{}
	if err := json.Unmarshal(msgBytes, &rawMsg); err != nil {
		c.SendError("invalid json format")
		return
	}

	// 检查是否有 action 字段，且不为空
	if action, ok := rawMsg["action"]; ok && action != "" {
		// 确认为控制消息，再解析到结构体
		var controlMsg ControlMessage
		if err := json.Unmarshal(msgBytes, &controlMsg); err != nil {
			c.SendError("invalid control message format")
			return
		}

		// 持锁处理控制逻辑
		c.Hub.mu.Lock()

		switch controlMsg.Action {
		case "join_room":
			if _, ok := c.Hub.rooms[controlMsg.RoomID]; !ok {
				c.Hub.rooms[controlMsg.RoomID] = make(map[*Client]bool)
			}
			if c.Hub.rooms[controlMsg.RoomID][c] == false {
				//TODO: 加入房间后，需要查询房间内已有消息，发送给客户端
				msgs, err := c.Srv.loadHistory(context.Background(), loadHistoryParams{
					ConversationID: controlMsg.RoomID,
					CursorID:       pgtype.Int8{Valid: false},
					Limit:          10,
				})
				if err != nil {
					c.SendError("failed to load history")
					c.Hub.mu.Unlock()
					return
				}
				c.Hub.mu.Unlock()
				var broMsgs []BroadcastMessage
				for _, msg := range msgs.msgs {
					broMsgs = append(broMsgs, BroadcastMessage{
						ConversationId: msg.ConversationID,
						MsgType:        int(msg.MsgType),
						Content:        msg.Content,
						SenderID:       msg.SenderID,
						SendTime:       msg.CreatedAt,
					})
				}
				c.Hub.SendMessageToUser(c, broMsgs)
				c.Hub.rooms[controlMsg.RoomID][c] = true
				fmt.Printf("✅ [JOIN] 用户 %d 加入房间 %d\n", c.UserID, controlMsg.RoomID)
			}
		case "leave_room":
			if _, ok := c.Hub.rooms[controlMsg.RoomID]; ok {
				c.Hub.rooms[controlMsg.RoomID][c] = false
			}
			fmt.Printf("✅ [LEAVE] 用户 %d 离开房间 %d\n", c.UserID, controlMsg.RoomID)
			c.Hub.mu.Unlock()

		default:
			fmt.Printf("⚠️ [WARN] 未知的 Action: %s\n", controlMsg.Action)
			c.Hub.mu.Unlock()
		}
		return
	}

	// 2. 如果没有 action 字段，为普通消息

	var message MessageInput
	// 解析为普通消息结构体
	if err := json.Unmarshal(msgBytes, &message); err != nil {
		c.SendError("invalid message format")
		return
	}

	// 权限校验
	if message.SenderID != c.UserID {
		c.SendError("sender id mismatch")
		return
	}

	// 持久化存储
	msgarg := storeMessageParams{
		ConversationID: message.ConversationId,
		MsgType:        int16(message.MsgType),
		Content:        message.Content,
		SenderID:       message.SenderID,
	}
	var msg storeMessageResult
	var err error
	if msg, err = c.Srv.storeMessage(context.Background(), msgarg); err != nil {
		c.SendError("failed to store message")
		return
	}

	// 准备广播数据
	broadcast := BroadcastMessage{
		ConversationId: msg.ConversationID,
		MsgType:        int(msg.MsgType),
		Content:        msg.Content,
		SenderID:       msg.SenderID,
		SendTime:       msg.CreatedAt,
	}

	// 3. 查找目标用户
	c.Hub.mu.Lock()
	targetClients := make([]*Client, 0)
	if room, ok := c.Hub.rooms[message.ConversationId]; ok {
		for client, status := range room {
			if status == true && client != c {
				targetClients = append(targetClients, client)
			}
		}
	}
	c.Hub.mu.Unlock()

	if len(targetClients) > 0 {
		c.Hub.SendMessageToUsers(targetClients, []BroadcastMessage{broadcast})
	} else {
		fmt.Printf("⚠️ [WARN] 房间 %d 中没有其他在线用户，消息未发送\n", message.ConversationId)
	}
}

func (c *Client) SendError(msg string) {
	errResp := map[string]interface{}{
		"type":    "error",
		"message": msg,
	}
	data, err := json.Marshal(errResp)
	if err != nil {
		fmt.Printf("错误消息序列化失败: %v", err)
		return
	}
	select {
	case c.Send <- data:
	default:
		fmt.Printf("用户 %d 设备 %s 错误消息发送通道阻塞", c.UserID, c.DeviceID)
	}
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan *DispatchTask),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[int64]map[string]*Client),
		stop:       make(chan struct{}),
		rooms:      make(map[int64]map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case task := <-h.broadcast:
			h.handleBroadcast(task)

		case <-h.stop:
			fmt.Printf("🛑 Hub 接收到停止信号，正在清理资源...\n")
			h.cleanup()
			return
		}
	}
}

func (h *Hub) cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	count := 0
	for _, clients := range h.rooms {
		for client := range clients {
			// 关闭 WebSocket 连接
			client.Conn.Close()
			count++
		}
	}

	fmt.Printf("✅ Hub 已停止，共断开 %d 个客户端连接\n", count)

}

func (h *Hub) Stop() {
	select {
	case h.stop <- struct{}{}:
	default:
	}
}

func (h *Hub) handleRegister(c *Client) {
	// 🔴 1. 进锁前打印，确认函数被调用

	h.mu.Lock()
	defer h.mu.Unlock()

	// 初始化用户设备地图
	if _, ok := h.clients[c.UserID]; !ok {
		h.clients[c.UserID] = make(map[string]*Client)
	}

	// 处理旧连接 (踢除旧设备)
	if old, ok := h.clients[c.UserID][c.DeviceID]; ok {
		fmt.Printf("⚠️ [HUB] 发现旧连接: User=%d, Device=%s, 正在关闭旧 Send 通道...\n", c.UserID, c.DeviceID)

		close(old.Send)

		for _, rid := range old.ConversationsID {
			if room, exists := h.rooms[rid]; exists {
				delete(room, old)
				if len(room) == 0 {
					delete(h.rooms, rid)
				}
			}
		}
		delete(h.clients[c.UserID], c.DeviceID)
	}

	// 加入房间
	if len(c.ConversationsID) == 0 {
		fmt.Printf("⚠️ [WARN] 用户 %d 没有加入任何房间 (ConversationsID 为空)\n", c.UserID)
	} else {
		for _, conversationID := range c.ConversationsID {
			if _, ok := h.rooms[conversationID]; !ok {
				h.rooms[conversationID] = make(map[*Client]bool)
			}
			h.rooms[conversationID][c] = false
		}
		fmt.Printf("📝 [HUB] 用户 %d 已加入 %d 个房间\n", c.UserID, len(c.ConversationsID))
	}

	h.clients[c.UserID][c.DeviceID] = c

	fmt.Printf("✅ [HUB] 用户 %d 的设备 %s 注册成功! 当前在线设备数: %d\n",
		c.UserID, c.DeviceID, len(h.clients[c.UserID]))
}

func (h *Hub) handleUnregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 1. 清理 clients 中的客户端
	if userClients, ok := h.clients[c.UserID]; ok {
		delete(userClients, c.DeviceID)
		fmt.Printf("🔖 用户 %d 的设备 %s 断开连接,该用户当前在线设备数: %d", c.UserID, c.DeviceID, len(userClients))
		if len(userClients) == 0 {
			delete(h.clients, c.UserID)
			fmt.Printf("👻 用户 %d 完全离线", c.UserID)
		}
	}

	// 2. 清理 rooms 中的客户端
	for roomID, clientsInRoom := range h.rooms {
		if _, ok := clientsInRoom[c]; ok {
			delete(clientsInRoom, c)
			// 如果房间无客户端，删除房间
			if len(clientsInRoom) == 0 {
				delete(h.rooms, roomID)
			}
		}
	}
}

func (h *Hub) handleBroadcast(task *DispatchTask) {
	msgBytes, err := json.Marshal(task.Message)
	if err != nil {
		fmt.Printf("消息序列化失败: %v", err)
		return
	}

	h.mu.Lock()
	// 注意：这里不要defer unlock，因为如果需要触发 unregister，可能需要更复杂的逻辑

	clientsToRemove := make([]*Client, 0)

	for _, client := range task.TargetClients {
		select {
		case client.Send <- msgBytes:
		default:
			fmt.Printf("⚠️ 用户 %d 设备 %s 发送通道阻塞，标记断开", client.UserID, client.DeviceID)
			clientsToRemove = append(clientsToRemove, client)
		}
	}
	h.mu.Unlock()

	// 在锁外处理断开，避免死锁，并调用统一的注销逻辑
	for _, client := range clientsToRemove {
		client.Conn.Close() // 关闭底层连接，触发 readPump 的 error
	}
}

func (h *Hub) SendMessageToUser(targetClient *Client, msg []BroadcastMessage) {
	task := &DispatchTask{
		TargetClients: []*Client{targetClient},
		Message:       msg,
	}
	h.broadcast <- task
}

func (h *Hub) SendMessageToUsers(targetClients []*Client, msg []BroadcastMessage) {
	task := &DispatchTask{
		TargetClients: targetClients,
		Message:       msg,
	}
	h.broadcast <- task
}
