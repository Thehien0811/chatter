package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// AppConfig holds all shared runtime configuration for the application.
type AppConfig struct {
	App struct {
		Name string `mapstructure:"name"`
		Env  string `mapstructure:"env"`
		Port string `mapstructure:"port"`
	} `mapstructure:"app"`
	JWT struct {
		Secret string        `mapstructure:"secret"`
		Expiry time.Duration `mapstructure:"expiry"`
	} `mapstructure:"jwt"`
	Database struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Name     string `mapstructure:"name"`
		SSLMode  string `mapstructure:"ssl_mode"`
		DSN      string `mapstructure:"dsn"`
	} `mapstructure:"database"`
	Redis struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`
	Casbin struct {
		ModelFile string `mapstructure:"model_file"`
	} `mapstructure:"casbin"`
	Services struct {
		AuthURL   string `mapstructure:"auth_url"`
		UserURL   string `mapstructure:"user_url"`
		FriendURL string `mapstructure:"friend_url"`
		ChatURL   string `mapstructure:"chat_url"`
		CallURL   string `mapstructure:"call_url"`
	} `mapstructure:"services"`
	MinIO struct {
		Endpoint  string `mapstructure:"endpoint"`
		AccessKey string `mapstructure:"access_key"`
		SecretKey string `mapstructure:"secret_key"`
		Bucket    string `mapstructure:"bucket"`
		Secure    bool   `mapstructure:"secure"`
	} `mapstructure:"minio"`
}

func (c *AppConfig) DSN() string {
	if c.Database.DSN != "" {
		return c.Database.DSN
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Name,
		c.Database.SSLMode,
	)
}

func Load() (*AppConfig, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	v.SetDefault("app.name", "chatter")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.port", "8080")
	v.SetDefault("jwt.secret", "change-me-in-production")
	v.SetDefault("jwt.expiry", "2h")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "postgres")
	v.SetDefault("database.password", "postgres")
	v.SetDefault("database.name", "chatter")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("casbin.model_file", "config/casbin/model.conf")
	v.SetDefault("services.auth_url", "localhost:9001")
	v.SetDefault("services.user_url", "localhost:9005")
	v.SetDefault("services.friend_url", "localhost:9002")
	v.SetDefault("services.chat_url", "localhost:9003")
	v.SetDefault("services.call_url", "localhost:9004")
	v.SetDefault("minio.endpoint", "localhost:9000")
	v.SetDefault("minio.bucket", "chatter-images")
	v.SetDefault("minio.secure", false)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("jwt.secret", "JWT_SECRET")
	_ = v.BindEnv("app.port", "APP_PORT")
	_ = v.BindEnv("database.host", "DB_HOST")
	_ = v.BindEnv("database.user", "DB_USER")
	_ = v.BindEnv("database.password", "DB_PASSWORD")
	_ = v.BindEnv("database.name", "DB_NAME")
	_ = v.BindEnv("redis.host", "REDIS_HOST")
	_ = v.BindEnv("redis.password", "REDIS_PASSWORD")
	_ = v.BindEnv("casbin.model_file", "CASBIN_MODEL_FILE")
	_ = v.BindEnv("minio.endpoint", "MINIO_ENDPOINT")
	_ = v.BindEnv("minio.access_key", "MINIO_ACCESS_KEY")
	_ = v.BindEnv("minio.secret_key", "MINIO_SECRET_KEY")
	_ = v.BindEnv("minio.bucket", "MINIO_BUCKET")
	_ = v.BindEnv("minio.secure", "MINIO_SECURE")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := &AppConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}
