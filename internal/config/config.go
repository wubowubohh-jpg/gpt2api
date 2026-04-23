package config

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Log       LogConfig       `mapstructure:"log"`
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Crypto    CryptoConfig    `mapstructure:"crypto"`
	Security  SecurityConfig  `mapstructure:"security"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Upstream  UpstreamConfig  `mapstructure:"upstream"`
	EPay      EPayConfig      `mapstructure:"epay"`
	Backup    BackupConfig    `mapstructure:"backup"`
	SMTP      SMTPConfig      `mapstructure:"smtp"`
}

type AppConfig struct {
	Name    string `mapstructure:"name"`
	Env     string `mapstructure:"env"`
	Listen  string `mapstructure:"listen"`
	BaseURL string `mapstructure:"base_url"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type MySQLConfig struct {
	DSN                string `mapstructure:"dsn"`
	MaxOpenConns       int    `mapstructure:"max_open_conns"`
	MaxIdleConns       int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeSec int    `mapstructure:"conn_max_lifetime_sec"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type JWTConfig struct {
	Secret        string `mapstructure:"secret"`
	AccessTTLSec  int    `mapstructure:"access_ttl_sec"`
	RefreshTTLSec int    `mapstructure:"refresh_ttl_sec"`
	Issuer        string `mapstructure:"issuer"`
}

type CryptoConfig struct {
	AESKey string `mapstructure:"aes_key"`
}

type SecurityConfig struct {
	BcryptCost  int      `mapstructure:"bcrypt_cost"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

type SchedulerConfig struct {
	MinIntervalSec   int     `mapstructure:"min_interval_sec"`
	DailyUsageRatio  float64 `mapstructure:"daily_usage_ratio"`
	LockTTLSec       int     `mapstructure:"lock_ttl_sec"`
	Cooldown429Sec   int     `mapstructure:"cooldown_429_sec"`
	WarnedPauseHours int     `mapstructure:"warned_pause_hours"`
}

type UpstreamConfig struct {
	BaseURL            string `mapstructure:"base_url"`
	RequestTimeoutSec  int    `mapstructure:"request_timeout_sec"`
	SSEReadTimeoutSec  int    `mapstructure:"sse_read_timeout_sec"`
}

// BackupConfig 数据库备份配置。
type BackupConfig struct {
	Dir           string `mapstructure:"dir"`            // 备份落盘目录,默认 /app/data/backups
	Retention     int    `mapstructure:"retention"`      // 保留最近 N 个(>0),0 表示不自动清理
	MysqldumpBin  string `mapstructure:"mysqldump_bin"`  // 默认 mysqldump
	MysqlBin      string `mapstructure:"mysql_bin"`      // 恢复用,默认 mysql
	MaxUploadMB   int    `mapstructure:"max_upload_mb"`  // 上传 .sql.gz 上限,默认 512
	AllowRestore  bool   `mapstructure:"allow_restore"`  // 是否允许 /restore 端点(生产强烈建议 false 手动切)
}

type EPayConfig struct {
	// GatewayURL 形如 https://pay.example.com/submit.php
	// 空字符串时整个充值通道被视为未启用,前端 list 会提示运维未配置。
	GatewayURL string `mapstructure:"gateway_url"`
	PID        string `mapstructure:"pid"`
	Key        string `mapstructure:"key"`
	// NotifyURL 后端异步回调(必填完整 https,不要带 query)
	NotifyURL string `mapstructure:"notify_url"`
	// ReturnURL 支付成功浏览器跳回(前端路由页,如 /billing)
	ReturnURL string `mapstructure:"return_url"`
	// SignType 目前只支持 MD5,保留扩展位。
	SignType string `mapstructure:"sign_type"`
	// Expires 订单默认有效期(分钟),0 取默认 30
	ExpiresMin int `mapstructure:"expires_min"`
}

// SMTPConfig 用于注册欢迎 / 充值到账 邮件通知。
// Host 为空时邮件通道整体关闭,不影响主流程。
type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`      // 显示的 From 地址
	FromName string `mapstructure:"from_name"` // 显示名
	UseTLS   bool   `mapstructure:"use_tls"`   // true 隐式 TLS(465),false STARTTLS(587)
}

var (
	global *Config
	once   sync.Once
)

func Load(path string) (*Config, error) {
	var loadErr error
	once.Do(func() {
		v := viper.New()
		v.SetEnvPrefix("GPT2API")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		// 设置合理的默认值,使纯环境变量部署(Zeabur 等)无需 config.yaml
		setDefaults(v)

		// config.yaml 是可选的——文件存在就加载,不存在则全走环境变量 + 默认值
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			// 如果文件不存在,这不是致命错误;其他错误(权限/语法)仍然上报
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				if !isFileNotExist(path) {
					loadErr = fmt.Errorf("read config: %w", err)
					return
				}
			}
			// 文件不存在,继续靠环境变量
		}

		var c Config
		if err := v.Unmarshal(&c); err != nil {
			loadErr = fmt.Errorf("unmarshal config: %w", err)
			return
		}

		// Zeabur / PaaS 自动注入的独立环境变量 → 自动组装 DSN / Addr
		autoWirePaaS(&c)

		global = &c
	})
	return global, loadErr
}

// setDefaults 为 Zeabur / 纯环境变量部署场景设置合理默认值。
func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "gpt2api")
	v.SetDefault("app.env", "prod")
	v.SetDefault("app.listen", ":8080")
	v.SetDefault("app.base_url", "")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.output", "stdout")

	v.SetDefault("mysql.max_open_conns", 50)
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.conn_max_lifetime_sec", 3600)

	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 50)

	v.SetDefault("jwt.access_ttl_sec", 86400)
	v.SetDefault("jwt.refresh_ttl_sec", 2592000)
	v.SetDefault("jwt.issuer", "gpt2api")

	v.SetDefault("security.bcrypt_cost", 10)

	v.SetDefault("scheduler.min_interval_sec", 60)
	v.SetDefault("scheduler.daily_usage_ratio", 0.6)
	v.SetDefault("scheduler.lock_ttl_sec", 1200)
	v.SetDefault("scheduler.cooldown_429_sec", 600)
	v.SetDefault("scheduler.warned_pause_hours", 24)

	v.SetDefault("upstream.base_url", "https://chatgpt.com")
	v.SetDefault("upstream.request_timeout_sec", 60)
	v.SetDefault("upstream.sse_read_timeout_sec", 300)

	v.SetDefault("smtp.port", 465)
	v.SetDefault("smtp.from_name", "GPT2API")
	v.SetDefault("smtp.use_tls", true)

	v.SetDefault("epay.sign_type", "MD5")
	v.SetDefault("epay.expires_min", 30)
}

// isFileNotExist 检测文件是否不存在。
func isFileNotExist(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

// autoWirePaaS 从 Zeabur / Railway / Render 等 PaaS 平台注入的独立环境变量
// 自动组装 MySQL DSN 和 Redis Addr。仅当 DSN/Addr 尚未设置时生效。
func autoWirePaaS(c *Config) {
	// ---- MySQL DSN ----
	// Zeabur 注入: MYSQL_HOST, MYSQL_PORT, MYSQL_USERNAME, MYSQL_PASSWORD, MYSQL_DATABASE
	// 兼容 Docker Compose 的 MYSQL_USER
	if c.MySQL.DSN == "" {
		host := envOr("MYSQL_HOST", "")
		port := envOr("MYSQL_PORT", "3306")
		user := envOr("MYSQL_USERNAME", envOr("MYSQL_USER", ""))
		pass := envOr("MYSQL_PASSWORD", "")
		dbname := envOr("MYSQL_DATABASE", "gpt2api")
		if host != "" && user != "" {
			c.MySQL.DSN = fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s?parseTime=true&loc=Local&charset=utf8mb4&collation=utf8mb4_unicode_ci",
				user, pass, host, port, dbname,
			)
		}
	}

	// ---- Redis Addr ----
	// Zeabur 注入: REDIS_HOST, REDIS_PORT, REDIS_PASSWORD
	if c.Redis.Addr == "" {
		host := envOr("REDIS_HOST", "")
		port := envOr("REDIS_PORT", "6379")
		if host != "" {
			c.Redis.Addr = host + ":" + port
		}
	}
	if c.Redis.Password == "" {
		if p := os.Getenv("REDIS_PASSWORD"); p != "" {
			c.Redis.Password = p
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Get 返回全局配置,仅在 Load 之后调用。
func Get() *Config {
	if global == nil {
		panic("config not loaded; call config.Load first")
	}
	return global
}
