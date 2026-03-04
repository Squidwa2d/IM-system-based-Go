package main

import (
	"log"

	"context"
	db "github.com/Squidwa2d/chat-room/db"
	"github.com/Squidwa2d/chat-room/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// 加载配置
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal("无法加载配置:", err)
	}

	//连接数据库
	pool, err := pgxpool.New(context.Background(), config.DBSource)
	if err != nil {
		log.Fatal("无法创建数据库连接池:", err)
	}
	store := db.NewStore(pool)
	defer pool.Close()

	// 启动服务器
}
