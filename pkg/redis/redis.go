package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/pkg/config"
)

// NewClient creates a new Redis client.
func NewClient(ctx context.Context, cfg config.RedisConfig, log zerolog.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Verify connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("pinging Redis: %w", err)
	}

	log.Info().
		Str("addr", cfg.Addr()).
		Int("db", cfg.DB).
		Msg("connected to Redis")

	return client, nil
}

// Close gracefully closes the Redis client.
func Close(client *redis.Client) error {
	if client != nil {
		return client.Close()
	}
	return nil
}
