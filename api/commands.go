package api

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
)

type Client struct {
	conn net.Conn
	name string
	ch   chan string
}

var (
	clients    = make(map[*Client]bool)
	broadcast  = make(chan string)
	register   = make(chan *Client)
	unregister = make(chan *Client)
	mu         = sync.Mutex{}
)

type CommandHandler func(client *Client, args []string)

// 全局命令注册表
var commands = make(map[string]CommandHandler)

func init() {
	// 注册命令
	commands["nick"] = handleNick
	commands["list"] = handleList
	commands["quit"] = handleQuit
	commands["help"] = handleHelp
}

func handleCommand(client *Client, input string) {
	// 去掉开头的 "/" 并按空格分割
	// 例如: "/nick Alice" -> ["nick", "Alice"]
	parts := strings.Fields(strings.TrimPrefix(input, "/"))
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0]) // 命令转为小写，方便匹配
	args := parts[1:]                // 剩余部分作为参数

	if handler, ok := commands[cmd]; ok {
		handler(client, args)
	} else {
		client.ch <- fmt.Sprintf("Unknown command: %s", cmd)
	}
}

func handleNick(client *Client, args []string) {
	client.name = args[0]
	client.ch <- "Nickname changed to " + client.name
}

func handleList(client *Client, args []string) {
	client.ch <- "Connected clients:"
	mu.Lock()
	cs := clients
	mu.Unlock()
	client.ch <- fmt.Sprintf("当前在线用户：%d", len(cs))
	for c := range cs {
		client.ch <- c.name
	}
}
func handleQuit(client *Client, args []string) {
	client.ch <- "Goodbye!"
	unregister <- client
	client.conn.Close()
}
func handleHelp(client *Client, args []string) {
	client.ch <- "Available commands:"
	for cmd := range commands {
		client.ch <- cmd
	}
}

func handleBroadcast() {
	for {
		msg := <-broadcast
		mu.Lock()
		for client := range clients {
			select {
			case client.ch <- msg:
			default:
				close(client.ch)
				delete(clients, client)
			}
		}
		mu.Unlock()
	}
}

func handleConnection(conn net.Conn) {
	client := &Client{conn: conn, ch: make(chan string)}
	max := big.NewInt(100)
	randnum, _ := rand.Int(rand.Reader, max)
	client.name = fmt.Sprintf("user" + randnum.String())
	client.ch <- "hello " + client.name + "! Welcome to the chat room!\n" + " Enter /help to see available commands."
	broadcast <- fmt.Sprintf("%s has joined the chat", client.name)
	register <- client
	defer func() {
		unregister <- client
		conn.Close()
		close(client.ch)
	}()
	go func() {
		for msg := range client.ch {
			conn.Write([]byte(msg + "\n"))
		}
	}()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		input := scanner.Text()
		if strings.HasPrefix(input, "/") {
			handleCommand(client, input)
		} else {
			msg := fmt.Sprintf("[%s]: %s", client.name, scanner.Text())
			broadcast <- msg
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
}

func handleClient() {
	for {
		select {
		case client := <-register:
			mu.Lock()
			clients[client] = true
			mu.Unlock()
			fmt.Println("New client connected:", client.name)

		case client := <-unregister:
			mu.Lock()
			if _, ok := clients[client]; ok {
				delete(clients, client)
				close(client.ch)
			}
			mu.Unlock()
			fmt.Println("Client disconnected:", client.name)
		}
	}
}
