package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: .env not loaded:", err)
	}

	tenantID := flag.Uint("tenant", 1, "tenant ID that owns the knowledge base")
	kbID := flag.String("kb", "", "knowledge base ID")
	docID := flag.String("doc", "", "knowledge/document ID to re-run wiki ingestion for")
	dbHost := flag.String("db-host", localEnvHost("DB_HOST", "localhost"), "PostgreSQL host")
	redisAddr := flag.String("redis-addr", localEnvHost("REDIS_ADDR", "localhost:6379"), "Redis address")
	wait := flag.Bool("wait", true, "wait for the wiki worker to finish")
	timeout := flag.Duration("timeout", 10*time.Minute, "maximum time to wait for wiki generation")
	flag.Parse()

	if *kbID == "" || *docID == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/wiki-rerun -kb <kb_id> -doc <knowledge_id> [-tenant 1]")
		os.Exit(2)
	}

	dbUser := envOr("DB_USER", "postgres")
	dbPass := envOr("DB_PASSWORD", "")
	dbName := envOr("DB_NAME", "WeKnora")
	dbPort := envOr("DB_PORT", "5432")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		*dbHost, dbPort, dbUser, dbPass, dbName)
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect database:", err)
		os.Exit(1)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		fmt.Fprintln(os.Stderr, "database pool:", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	redisDB, _ := strconv.Atoi(envOr("REDIS_DB", "0"))
	rdb := redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
		Password: envOr("REDIS_PASSWORD", ""),
		DB:       redisDB,
	})
	taskClient := asynq.NewClientFromRedisClient(rdb)
	defer taskClient.Close()

	pendingRepo := repository.NewTaskPendingOpsRepository(gormDB)
	accepted, err := service.EnqueueWikiIngest(
		context.Background(),
		taskClient,
		pendingRepo,
		uint64(*tenantID),
		*kbID,
		*docID,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "enqueue wiki ingest:", err)
		if accepted {
			fmt.Fprintln(os.Stderr, "durable pending op was written; restart the server or retry to schedule the worker")
		}
		os.Exit(1)
	}
	if !accepted {
		fmt.Fprintln(os.Stderr, "knowledge base is not active; pending op was not written")
		os.Exit(1)
	}

	fmt.Printf("queued wiki:ingest for knowledge %s in KB %s (tenant %d)\n", *docID, *kbID, *tenantID)
	if *wait {
		fmt.Println("waiting for wiki generation to finish...")
		if err := waitForWikiIngest(gormDB, uint64(*tenantID), *kbID, *docID, *timeout); err != nil {
			fmt.Fprintln(os.Stderr, "wiki generation failed:", err)
			os.Exit(1)
		}
		fmt.Printf("wiki:ingest finished for knowledge %s in KB %s (tenant %d)\n", *docID, *kbID, *tenantID)
		return
	}
	fmt.Println("queue-only mode: the worker will pick it up without re-parsing chunks")
}

func waitForWikiIngest(db *gorm.DB, tenantID uint64, kbID, docID string, timeout time.Duration) error {
	startedAt := time.Now()
	deadline := startedAt.Add(timeout)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s; operation is still pending in task_pending_ops", timeout)
		}

		var pending int64
		if err := db.Table("task_pending_ops").
			Where("tenant_id = ? AND task_type = ? AND scope = ? AND scope_id = ? AND op = ? AND dedup_key = ?",
				tenantID, "wiki:ingest", "knowledge_base", kbID, "ingest", docID).
			Count(&pending).Error; err != nil {
			return fmt.Errorf("poll pending op: %w", err)
		}
		if pending > 0 {
			<-ticker.C
			continue
		}

		var deadLetters int64
		if err := db.Table("task_dead_letters").
			Where("tenant_id = ? AND task_type = ? AND scope = ? AND scope_id = ? AND related_id = ? AND failed_at >= ?",
				tenantID, "wiki:ingest", "knowledge_base", kbID, docID, startedAt).
			Count(&deadLetters).Error; err != nil {
			return fmt.Errorf("poll dead letter: %w", err)
		}
		if deadLetters > 0 {
			return fmt.Errorf("operation reached a dead letter")
		}
		return nil
	}
}

func localEnvHost(key, fallback string) string {
	value := envOr(key, fallback)
	if value == "redis" || value == "redis:6379" {
		return "localhost:6379"
	}
	if strings.HasPrefix(value, "redis:") {
		return "localhost:" + strings.TrimPrefix(value, "redis:")
	}
	if value == "postgres" {
		return "localhost"
	}
	return value
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
