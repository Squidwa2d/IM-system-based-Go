package api

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
	"log"
	"net/http"
	"sync"
	"time"
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
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	UserID   int64
	DeviceID string
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	//设置读取超时，如果长时间收不到消息，就关闭连接
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	//设置pong处理器，如果收到pong消息，就重置读取超时时间
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		//TODO: 处理消息
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
			if !ok { // The hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
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

type BroadcastMessage struct {
	ConversationId int                `json:"conversation_id"`
	MsgType        int                `json:"msg_type"` //1:text, 2:image, 3:file, 4:voice
	Content        interface{}        `json:"content"`
	SenderID       int                `json:"sender_id"`
	SendTime       pgtype.Timestamptz `json:"send_time"`
}

type Hub struct {
	mu         sync.Mutex
	clients    map[int64]map[string]*Client
	broadcast  chan *DispatchTask
	register   chan *Client
	unregister chan *Client
}

type DispatchTask struct {
	TargetUserIDs []int64          // 接收者列表 (单聊是1个，群聊是N个)
	Message       BroadcastMessage // 消息内容
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan *DispatchTask),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[int64]map[string]*Client),
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
		}
	}
}

func (h *Hub) handleRegister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c.UserID]; !ok {
		h.clients[c.UserID] = make(map[string]*Client)
		//TODO : 用户上线处理 更改状态
	}

	if old, ok := h.clients[c.UserID][c.DeviceID]; ok {
		close(old.Send)
		delete(h.clients[c.UserID], c.DeviceID)
		log.Printf("🔄 用户 %d 的设备 %s 重新连接，旧连接已关闭", c.UserID, c.DeviceID)
	}

	h.clients[c.UserID][c.DeviceID] = c
	log.Printf("🔗 用户 %d 的设备 %s 连接成功,该用户当前在线设备数: %d", c.UserID, c.DeviceID, len(h.clients[c.UserID]))
}

func (h *Hub) handleUnregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c.UserID]; ok {
		delete(h.clients[c.UserID], c.DeviceID)
		log.Printf("🔖 用户 %d 的设备 %s 断开连接,该用户当前在线设备数: %d", c.UserID, c.DeviceID, len(h.clients[c.UserID]))
	}

	if len(h.clients[c.UserID]) == 0 {
		delete(h.clients, c.UserID)
		log.Printf("👻 用户 %d 完全离线", c.UserID)
		//TODO: 用户离线处理 更改状态
	}
}

func (h *Hub) handleBroadcast(task *DispatchTask) {
	msgBytes, err := json.Marshal(task.Message)
	if err != nil {
		log.Printf("消息序列化失败: %v", err)
		return
	}

	//TODO: 消息持久化

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, userID := range task.TargetUserIDs {
		if _, ok := h.clients[userID]; ok {
			for _, client := range h.clients[userID] {
				select {
				case client.Send <- msgBytes:
				default:
					log.Printf("⚠️ 用户 %d 设备 %s 发送通道阻塞，强制断开", client.UserID, client.DeviceID)
					close(client.Send)
				}
			}
		}
	}
}

func (h *Hub) SendMessageToUser(targetID int64, msg BroadcastMessage) {
	task := &DispatchTask{
		TargetUserIDs: []int64{targetID},
		Message:       msg,
	}
	h.broadcast <- task
}

func (h *Hub) SendMessageToUsers(targetIDs []int64, msg BroadcastMessage) {
	task := &DispatchTask{
		TargetUserIDs: targetIDs,
		Message:       msg,
	}
	h.broadcast <- task
}
