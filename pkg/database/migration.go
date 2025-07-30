package database

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // 导入file源驱动
	"github.com/sirupsen/logrus"

	"simplied-blockchain-data-monitor-alert-go/pkg/logger"
)

// Migrator 数据库迁移器
type Migrator struct {
	// 迁移器
	migrate *migrate.Migrate
	// 日志记录器
	logger *logger.Logger
}

// NewMigrator 创建迁移器
func NewMigrator(db *sql.DB, migrationsPath string, logger *logger.Logger) (*Migrator, error) {
	/**
	1. 创建 Postgres 驱动
	2. 创建迁移器
	3. 返回迁移器
	*/
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", filepath.Clean(migrationsPath))
	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return &Migrator{
		migrate: m,
		logger:  logger,
	}, nil
}

// Up 执行向上迁移
func (m *Migrator) Up() error {
	m.logger.Info("Starting database migration up")

	err := m.migrate.Up()
	if err != nil && err != migrate.ErrNoChange {
		m.logger.WithError(err).Error("Migration up failed")
		return fmt.Errorf("migration up failed: %w", err)
	}

	if err == migrate.ErrNoChange {
		m.logger.Info("No migrations to apply")
	} else {
		m.logger.Info("Migration up completed successfully")
	}

	return nil
}

// Version 获取当前迁移版本
func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err != nil {
		m.logger.WithError(err).Error("Failed to get migration version")
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	m.logger.WithFields(logrus.Fields{
		"version": version,
		"dirty":   dirty,
	}).Info("Current migration version")

	return version, dirty, nil
}

// Force 强制设置迁移版本（用于修复脏状态）
func (m *Migrator) Force(version int) error {
	m.logger.WithField("version", version).Info("Forcing migration version")

	err := m.migrate.Force(version)
	if err != nil {
		m.logger.WithError(err).Error("Failed to force migration version")
		return fmt.Errorf("failed to force migration version: %w", err)
	}

	m.logger.Info("Migration version forced successfully")
	return nil
}
