package main

import (
	"log"

	"context"
	api "github.com/Squidwa2d/IM-system-based-Go/api"
	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	util "github.com/Squidwa2d/IM-system-based-Go/utils"
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
	server, err := api.NewServer(config, store)
	if err != nil {
		log.Fatal("无法创建服务器:", err)
	}
	// 启动服务器
	if err := server.Start(config.ServerAddress); err != nil {
		log.Fatal("无法启动服务器:", err)
	}
}
