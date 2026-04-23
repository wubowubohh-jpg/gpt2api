package db

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	embedsql "github.com/432539/gpt2api/sql"
	"github.com/432539/gpt2api/pkg/logger"
)

// AutoMigrate 使用内嵌的 SQL 迁移文件自动建表/升级表结构。
// 幂等调用,已执行过的迁移不会重复执行。
func AutoMigrate(rawDB *sql.DB) error {
	log := logger.L()

	goose.SetBaseFS(embedsql.EmbedMigrations)
	goose.SetLogger(gooseLogger{log})

	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	log.Info("auto-migrate: running embedded goose migrations...")
	if err := goose.Up(rawDB, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	ver, err := goose.GetDBVersion(rawDB)
	if err != nil {
		log.Warn("auto-migrate: cannot read db version", zap.Error(err))
	} else {
		log.Info("auto-migrate: done", zap.Int64("db_version", ver))
	}
	return nil
}

// gooseLogger 适配 goose.Logger 接口,转到 zap。
type gooseLogger struct {
	z *zap.Logger
}

func (l gooseLogger) Fatalf(format string, v ...interface{}) {
	l.z.Fatal(fmt.Sprintf(format, v...))
}

func (l gooseLogger) Printf(format string, v ...interface{}) {
	l.z.Info(fmt.Sprintf(format, v...))
}
