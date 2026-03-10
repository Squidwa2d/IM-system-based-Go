package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/Squidwa2d/IM-system-based-Go/api"
	db "github.com/Squidwa2d/IM-system-based-Go/db/sqlc"
	util "github.com/Squidwa2d/IM-system-based-Go/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. 加载配置
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal("❌ 无法加载配置:", err)
	}

	// 2. 配置并初始化数据库连接池
	poolConfig, err := pgxpool.ParseConfig(config.DBSource)
	if err != nil {
		log.Fatal("❌ 无法解析数据库配置:", err)
	}

	// ✅ 关键优化：显式设置连接池参数
	// 根据服务器性能和数据库 max_connections 调整
	// 假设数据库允许 100-200 连接，单实例设为 50 比较安全
	poolConfig.MaxConns = 50
	poolConfig.MinConns = 10
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		log.Fatal("❌ 无法创建数据库连接池:", err)
	}

	// 验证连接
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("❌ 数据库连接失败:", err)
	}
	log.Println("✅ 数据库连接池初始化成功")

	store := db.NewStore(pool)

	// 配置并初始化 Redis 客户端
	rdb := redis.NewClient(&redis.Options{
		Addr: config.RdbSource, // Redis 地址
		DB:   0,                // 默认数据库
	})
	rdbStore := api.NewRedisStore(rdb)

	//配置并初始化MinioClient

	minioClient, err := api.NewMinioClient(config.MinioEndpoint, config.MinioAccessKey, config.MinioSecretKey, config.MinioUseSSL)
	// 3. 初始化 Server
	server, err := api.NewServer(config, store, rdbStore, minioClient)
	if err != nil {
		log.Fatal("❌ 无法创建服务器:", err)
	}

	// 4. 启动 HTTP 服务 (返回 *http.Server 以便控制)
	httpServer := server.StartHTTP(config.ServerAddress)

	// 5. 监听中断信号 (Ctrl+C, kill)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("🚀 服务器已启动，监听地址: %s", config.ServerAddress)
	log.Println("⌨️  按 Ctrl+C 停止服务...")

	<-quit // 等待中断信号
	log.Println("🛑 接收到退出信号，开始停机...")

	// 6. 创建超时上下文 (给正在处理的请求 5 秒 时间完成)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 7. 关闭 HTTP 服务器 (停止接收新连接，等待旧连接处理完)
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("⚠️ 服务器强制关闭: %v", err)
	}

	// 8. 关闭 Hub (通知所有 WebSocket 客户端断开，清理内存)
	server.StopHub()

	// 9. 关闭数据库连接池 (确保所有写操作已完成)
	pool.Close()
	log.Println("✅ 服务器已完全停止，资源已释放")
}
