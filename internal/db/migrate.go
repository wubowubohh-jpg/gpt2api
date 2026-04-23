package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	embedsql "github.com/432539/gpt2api/sql"
	"github.com/432539/gpt2api/pkg/logger"
)

// AutoMigrate 使用内嵌的 SQL 迁移文件自动建表/升级表结构。
// 幂等调用,已执行过的迁移不会重复执行。
//
// dsn: 应用的 MySQL DSN。函数内部会自动追加 multiStatements=true
// 打开一个独立连接专门跑迁移,不影响主连接。
func AutoMigrate(dsn string) error {
	log := logger.L()

	migrateDSN := ensureMultiStatements(dsn)

	migrateDB, err := sql.Open("mysql", migrateDSN)
	if err != nil {
		return fmt.Errorf("open migrate db: %w", err)
	}
	defer migrateDB.Close()

	migrateDB.SetMaxOpenConns(1)

	goose.SetBaseFS(embedsql.EmbedMigrations)
	goose.SetLogger(gooseLogger{log})

	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	log.Info("auto-migrate: running embedded goose migrations...")
	if err := goose.Up(migrateDB, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	ver, err := goose.GetDBVersion(migrateDB)
	if err != nil {
		log.Warn("auto-migrate: cannot read db version", zap.Error(err))
	} else {
		log.Info("auto-migrate: done", zap.Int64("db_version", ver))
	}
	return nil
}

// ensureMultiStatements 确保 DSN 中包含 multiStatements=true。
// MySQL 驱动需要此参数才能在单次 Exec 中执行多条 SQL(goose 迁移需要)。
func ensureMultiStatements(dsn string) string {
	if strings.Contains(dsn, "multiStatements=true") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&multiStatements=true"
	}
	return dsn + "?multiStatements=true"
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
