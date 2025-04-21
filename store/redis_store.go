package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client     *redis.Client
	queueKey   string
	pendingKey string
}

func NewRedisStore(addr string, password string, db int, baseKey string) *RedisStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisStore{
		client:     rdb,
		queueKey:   fmt.Sprintf("%s:queue", baseKey),
		pendingKey: fmt.Sprintf("%s:pending", baseKey),
	}
}

func (s *RedisStore) Push(task interfaces.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}
	return s.client.LPush(context.Background(), s.queueKey, data).Err()
}

func (s *RedisStore) Pop() (interfaces.Task, error) {
	ctx := context.Background()
	data, err := s.client.RPop(ctx, s.queueKey).Result()
	if err == redis.Nil {
		return interfaces.Task{}, errors.New("no tasks available")
	}
	if err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to pop task: %w", err)
	}

	if err := s.client.LPush(ctx, s.pendingKey, data).Err(); err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to move to pending: %w", err)
	}

	var task interfaces.Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return task, nil
}

func (s *RedisStore) Ack(task interfaces.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task for ack: %w", err)
	}

	return s.client.LRem(context.Background(), s.pendingKey, 1, data).Err()
}
