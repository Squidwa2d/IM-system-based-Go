package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type SessionData struct {
	UserID   int64  `json:"user_id"`
	DeviceID string `json:"device_id"`
	UserName string `json:"user_name"`
}

type RedisStore struct {
	client *redis.Client
	ctx    context.Context
}

const (
	AccessTokenTTL  = 30 * time.Minute
	RefreshTokenTTL = 24 * time.Hour

	keyAccseePrefix  = "access_token:"
	keyRefreshPrefix = "refresh_token:"
)

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{
		client: client,
		ctx:    context.Background(),
	}
}

func NewRedis(Addr, Password string, DB int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     Addr,
		Password: Password,
		DB:       DB,
	})
}
func (s *RedisStore) CreateSession(accessToken, refreshToken string, userID int64, deviceID string, userName string) (string, string, error) {
	sessionData := SessionData{
		UserID:   userID,
		DeviceID: deviceID,
		UserName: userName,
	}
	dataJson, err := json.Marshal(sessionData)
	if err != nil {
		return "", "", err
	}

	//写入access_token
	accessKey := fmt.Sprintf("%s%s", keyAccseePrefix, accessToken)
	if err := s.client.Set(s.ctx, accessKey, dataJson, AccessTokenTTL).Err(); err != nil {
		return "", "", err
	}

	//写入refresh_token
	refreshKey := fmt.Sprintf("%s%d%s", keyRefreshPrefix, userID, deviceID)
	if err := s.client.Set(s.ctx, refreshKey, refreshToken, RefreshTokenTTL).Err(); err != nil {
		s.client.Del(s.ctx, accessKey)
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (s *RedisStore) UpdateAccessToken(accessToken string, userID int64, deviceID string, userName string) error {
	sessionData := SessionData{
		UserID:   userID,
		DeviceID: deviceID,
		UserName: userName,
	}
	dataJson, err := json.Marshal(sessionData)
	if err != nil {
		return err
	}

	//写入access_token
	accessKey := fmt.Sprintf("%s%s", keyAccseePrefix, accessToken)
	if err := s.client.Set(s.ctx, accessKey, dataJson, AccessTokenTTL).Err(); err != nil {
		return err
	}
	return nil
}

func (s *RedisStore) DeleteSession(accessToken string, userID int64, deviceID string) error {
	key := fmt.Sprintf("%s%s", keyAccseePrefix, accessToken)
	err := s.client.Del(s.ctx, key).Err()
	if err != nil {
		return err
	}
	refreshKey := fmt.Sprintf("%s%d%s", keyRefreshPrefix, userID, deviceID)
	err = s.client.Del(s.ctx, refreshKey).Err()
	if err != nil {
		return err
	}
	return nil
}

func (s *RedisStore) ValidateAccessToken(token string) (*SessionData, bool, error) {
	key := fmt.Sprintf("%s%s", keyAccseePrefix, token)

	val, err := s.client.Get(s.ctx, key).Result()
	if err == redis.Nil {
		return nil, false, nil // Token 不存在或过期
	}
	if err != nil {
		return nil, false, err
	}
	var data SessionData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, false, err
	}
	return &data, true, nil
}

func (s *RedisStore) ValidateRefreshToken(userID int64, deviceID, refreshToken string) (bool, error) {
	refreshKey := fmt.Sprintf("%s%d%s", keyRefreshPrefix, userID, deviceID)

	storeToken, err := s.client.Get(s.ctx, refreshKey).Result()
	if storeToken != refreshToken || err == redis.Nil {
		return false, errors.New("invalid refresh token")
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *RedisStore) CheckRrefreshToken(userID int64, deviceID string) (bool, string) {
	refreshKey := fmt.Sprintf("%s%d%s", keyRefreshPrefix, userID, deviceID)

	_, err := s.client.Get(s.ctx, refreshKey).Result()
	if err == redis.Nil {
		return false, "" // Token 不存在或过期
	}
	if err != nil {
		return false, ""
	}
	return true, refreshKey
}
