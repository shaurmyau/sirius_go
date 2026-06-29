package config

import "os"

type DBConfig struct{ Host, Port, User, Pass, Name string }
type Config struct{ DB DBConfig }

func Load() *Config {
	return &Config{DB: DBConfig{
		Host: env("DB_HOST", "localhost"),
		Port: env("DB_PORT", "5432"),
		User: env("DB_USER", "dbuser"),
		Pass: env("DB_PASS", "dbpass"),
		Name: env("DB_NAME", "exam"),
	}}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
