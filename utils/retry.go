package utils

import (
	"time"

	"github.com/sirupsen/logrus"
)

// 重试方法，在同一个线程中重试，重试过程中不会另外启动goroutine
// attempts: 重试次数
// sleep: 重试间隔时间
// sleepRage: 间隔倍率，每一次重试时间都会以这个倍率翻倍
// fn: 重试需要执行的方法
func Retry(attempts int, sleep time.Duration, sleepRate int, fn func() error) error {
	if err := fn(); err != nil {
		if s, ok := err.(stop); ok {
			return s.error
		}

		if attempts--; attempts > 0 {
			logrus.Warnf("retry func error: %s. attemps #%d after %s.", err.Error(), attempts, sleep)

			if sleep != 0 {
				time.Sleep(sleep)
			}

			return Retry(attempts, time.Duration(int64(sleep)*int64(sleepRate)), sleepRate, fn)
		}
		return err
	}
	return nil
}

type stop struct {
	error
}

func NoRetryError(err error) stop {
	return stop{err}
}
