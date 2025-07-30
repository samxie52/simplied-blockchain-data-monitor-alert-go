package main

import (
	"flag"
	"fmt"
	"os"

	"simplied-blockchain-data-monitor-alert-go/internal/config"
	"simplied-blockchain-data-monitor-alert-go/pkg/database"
	"simplied-blockchain-data-monitor-alert-go/pkg/logger"
)

func main() {
	var (
		command        *string = flag.String("command", "up", "Migration command: up, down, version, force")
		migrationsPath *string = flag.String("path", "./migrations", "Path to migrations directory")
		forceVersion   *int    = flag.Int("force-version", 0, "Version to force (used with force command)")
		// flag.String() 返回一个指向字符串的指针
		configPath *string = flag.String("config", ".env", "Path to configuration file")
	)
	// flag.Parse() 解析命令行参数
	flag.Parse()

	// 加载配置
	cfg := config.Config{}
	if err := config.NewEnvLoader(*configPath).Load(&cfg); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 创建日志记录器
	loggerConfig := logger.Config{
		Level:     "info",
		Format:    "text",
		Output:    "stdout",
		Component: "migrator",
	}
	log, err := logger.New(loggerConfig)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	// 创建数据库连接
	pgManager, err := database.NewPostgresManager(cfg.Database, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create PostgreSQL manager")
	}
	defer pgManager.Close()

	// 创建迁移器
	migrator, err := database.NewMigrator(pgManager.GetDB().DB, *migrationsPath, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to create migrator")
	}

	// 执行迁移命令
	switch *command {
	case "up":
		if err := migrator.Up(); err != nil {
			log.WithError(err).Fatal("Migration up failed")
		}
		log.Info("Migration up completed")

	case "version":
		v, dirty, err := migrator.Version()
		if err != nil {
			log.WithError(err).Fatal("Failed to get migration version")
		}
		fmt.Printf("Current version: %d, Dirty: %t\n", v, dirty)

	case "force":
		if *forceVersion < 0 {
			log.Fatal("Force version must be >= 0")
		}
		if err := migrator.Force(*forceVersion); err != nil {
			log.WithError(err).Fatal("Failed to force migration version")
		}
		log.Info("Migration version forced successfully")

	default:
		log.Fatal("Unknown command: " + *command)
	}
}
