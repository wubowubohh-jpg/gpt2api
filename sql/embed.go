package sql

import "embed"

// EmbedMigrations 内嵌 migrations 目录下的全部 .sql 文件。
// 被 internal/db.AutoMigrate 引用,实现启动时自动建表。
//
//go:embed migrations/*.sql
var EmbedMigrations embed.FS
