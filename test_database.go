package main

import (
	"context"
	"fmt"
	"time"

	"simplied-blockchain-data-monitor-alert-go/internal/config"
	"simplied-blockchain-data-monitor-alert-go/pkg/database"
	"simplied-blockchain-data-monitor-alert-go/pkg/logger"
)

func main() {
	// 加载配置

	cfg := config.Config{}
	if err := config.NewEnvLoader(".env").Load(&cfg); err != nil {
		panic(err)
	}

	// 创建日志记录器
	loggerConfig := logger.Config{
		Level:     "info",
		Format:    "text",
		Output:    "stdout",
		Component: "test",
	}
	log, err := logger.New(loggerConfig)
	if err != nil {
		panic(err)
	}

	// 测试 PostgreSQL 连接
	fmt.Println("Testing PostgreSQL connection...")
	pgManager, err := database.NewPostgresManager(cfg.Database, log)
	if err != nil {
		panic(err)
	}
	defer pgManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pgManager.Ping(ctx); err != nil {
		panic(err)
	}
	fmt.Println("✅ PostgreSQL connection successful")

	// 测试 Redis 连接
	fmt.Println("Testing Redis connection...")
	redisManager, err := database.NewRedisManager(cfg.Redis, log)
	if err != nil {
		panic(err)
	}
	defer redisManager.Close()

	if err := redisManager.Ping(ctx); err != nil {
		panic(err)
	}
	fmt.Println("✅ Redis connection successful")

	// 测试健康检查
	fmt.Println("Testing health checker...")
	healthChecker := database.NewHealthChecker(pgManager, redisManager, log)
	healthChecker.Start()

	time.Sleep(2 * time.Second)
	health := healthChecker.GetHealth()

	fmt.Printf("PostgreSQL: %s (Response: %dms)\n",
		health.PostgreSQL.Status,
		health.PostgreSQL.ResponseTime.Milliseconds())
	fmt.Printf("Redis: %s (Response: %dms)\n",
		health.Redis.Status,
		health.Redis.ResponseTime.Milliseconds())
	fmt.Printf("Overall: %s\n", health.Overall.Status)

	if health.Overall.Healthy {
		fmt.Println("✅ All database connections are healthy")
	} else {
		fmt.Println("❌ Some database connections are unhealthy")
	}
}
