package redis

import (
	"errors"
	db "github.com/go-redis/redis/v7"
	"github.com/prometheus/common/log"
	"os"
	"time"
)

var (
	_default_redis *db.ClusterClient
)

func InitRedis(hosts []string, pass string, poolsize int) {
	log.Infof("redis hosts %v, pass %v", hosts, pass)
	client := db.NewClusterClient(&db.ClusterOptions{
		Addrs:    hosts,
		Password: pass,
		PoolSize: poolsize,
	})

	pong, err := client.Ping().Result()
	if err != nil {
		log.Errorf("Connect to redis servers failed: %v", err)
		os.Exit(-1)
	} else {
		log.Info(pong)
	}
	_default_redis = client
}

func getRedis() *db.ClusterClient {
	return _default_redis
}

func IsNilReply(err error) bool {
	return err == db.Nil
}

func RedisSet(key string, data interface{}, expiration time.Duration) (err error) {
	client := getRedis()
	if client == nil {
		err = errors.New("redis service unavailable")
		return
	}

	reply := client.Set(key, data, expiration)
	err = reply.Err()
	return
}

func RedisGet(key string) (reply *db.StringCmd, err error) {
	client := getRedis()
	if client == nil {
		err = errors.New("redis service unavailable")
		return
	}

	reply = client.Get(key)
	err = reply.Err()
	return
}

func RedisHSet(hKey, key, data string) (succ int64, err error) {
	client := getRedis()
	if client == nil {
		err = errors.New("redis service unavailable")
		return
	}

	succ, err = client.HSet(hKey, key, data).Result()
	return
}

func RedisHGet(hKey, key string) (value string, err error) {
	client := getRedis()
	if client == nil {
		err = errors.New("redis service unavailable")
		return
	}

	value, err = client.HGet(hKey, key).Result()
	return
}

func RedisHIncr(hKey, key string, num int64) (err error) {
	client := getRedis()
	if client == nil {
		err = errors.New("redis service unavailable")
		return
	}

	reply := client.HIncrBy(hKey, key, num)
	err = reply.Err()
	return
}

func RedisHash(hKey string) (hash map[string]string, err error) {
	client := getRedis()
	if client == nil {
		err = errors.New("redis service unavailable")
		return
	}

	hash, err = client.HGetAll(hKey).Result()
	return
}
