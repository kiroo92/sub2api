package repository

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// ponytail: dissolution scans short-lived image tasks; add a per-key index if task volume makes deletion slow.
func deleteTeamImageTaskCache(ctx context.Context, rdb *redis.Client, ids map[int64]bool) error {
	if len(ids) == 0 {
		return nil
	}
	var cursor uint64
	for {
		keys, next, err := rdb.Scan(ctx, cursor, imageTaskKeyPrefix+"*", 100).Result()
		if err != nil {
			return err
		}
		for _, key := range keys {
			data, err := rdb.Get(ctx, key).Bytes()
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				return err
			}
			var record service.ImageTaskRecord
			if err = json.Unmarshal(data, &record); err != nil {
				return err
			}
			if ids[record.APIKeyID] {
				if err = rdb.Del(ctx, key).Err(); err != nil {
					return err
				}
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
