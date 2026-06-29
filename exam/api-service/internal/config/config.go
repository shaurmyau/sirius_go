package config

import "os"

type DBConfig struct {
	Host, Port, User, Pass, Name string
}

type S3Config struct {
	Host, Port, User, Pass, ExternalURL string
}

type Config struct {
	DB      DBConfig
	S3      S3Config
	JWKSUrl string
}

func Load() *Config {
	return &Config{
		DB: DBConfig{
			Host: env("DB_HOST", "localhost"),
			Port: env("DB_PORT", "5432"),
			User: env("DB_USER", "dbuser"),
			Pass: env("DB_PASS", "dbpass"),
			Name: env("DB_NAME", "exam"),
		},
		S3: S3Config{
			Host:        env("S3_HOST", "localhost"),
			Port:        env("S3_PORT", "9000"),
			User:        env("S3_USER", "s3user"),
			Pass:        env("S3_PASS", "s3pass123"),
			ExternalURL: env("S3_EXTERNAL_URL", ""),
		},
		JWKSUrl: env("JWKS_URL", "https://id.klsh.ru/realms/sirius/protocol/openid-connect/certs"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
