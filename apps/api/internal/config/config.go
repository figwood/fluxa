package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv             string
	HTTPAddr           string
	DBDriver           string
	SQLitePath         string
	PostgresDSN        string
	AutoMigrate        bool
	WorkerPollInterval time.Duration
	WorkerID           string
	WorkerReplicas     int
	JWTSecret          string
	AccessTokenTTL     time.Duration
	GitlabURL          string
	GitlabToken        string
	GitlabTimeout      time.Duration
	TrustedProxies     []string
}

func Load() Config {
	loadDotEnv()
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "local"
	}
	return Config{
		AppEnv:             env("APP_ENV", "development"),
		HTTPAddr:           env("HTTP_ADDR", ":8090"),
		DBDriver:           env("DB_DRIVER", "postgres"),
		SQLitePath:         env("SQLITE_PATH", "./data/fluxa.db"),
		PostgresDSN:        env("POSTGRES_DSN", ""),
		AutoMigrate:        envBool("AUTO_MIGRATE", true),
		WorkerPollInterval: time.Duration(envInt("WORKER_POLL_INTERVAL_MS", 2000)) * time.Millisecond,
		WorkerID:           env("WORKER_ID", fmt.Sprintf("worker-%s-%d", hostname, os.Getpid())),
		WorkerReplicas:     envInt("WORKER_REPLICAS", 1),
		JWTSecret:          env("JWT_SECRET", "fluxa-dev-secret-change-me"),
		AccessTokenTTL:     time.Duration(envInt("ACCESS_TOKEN_TTL_HOURS", 8)) * time.Hour,
		GitlabURL:          env("GITLAB_URL", ""),
		GitlabToken:        env("GITLAB_TOKEN", ""),
		GitlabTimeout:      time.Duration(envInt("GITLAB_TIMEOUT_SECONDS", 10)) * time.Second,
		TrustedProxies:     envCSV("TRUSTED_PROXIES"),
	}
}

// loadDotEnv loads repository-local configuration without overriding values
// already supplied by the process environment. .env.local has precedence over
// .env, matching the convention used by the web application.
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	root := dir
	for {
		if exists(filepath.Join(root, "go.work")) || exists(filepath.Join(root, "pnpm-workspace.yaml")) {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			root = dir
			break
		}
		root = parent
	}
	for _, name := range []string{".env.local", ".env"} {
		loadEnvFile(filepath.Join(root, name))
	}
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		_ = os.Setenv(key, value)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func envCSV(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	items := []string{}
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || v == "true" || v == "TRUE"
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
	}
	return fallback
}
