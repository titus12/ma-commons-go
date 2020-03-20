// 本包中通用方法存放的地方，比如取环境变量
// 获取环境变量失败直接Panic,让进程停止掉，一来这些方法一定只会在启动时调用
// 启动时发现的错误不应该隐藏掉。

package server

import (
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

func stringEnv(name string) string {
	envstr := os.Getenv(name)
	if len(envstr) <= 0 {
		logrus.Warnf("环境变量 %s 不存在", name)
	}
	return envstr
}

func stringSliceEnv(name, sep string) []string {
	envstr := stringEnv(name)
	return strings.Split(envstr, sep)
}

func intEnv(name string) int {
	envstr := stringEnv(name)
	if len(envstr) <= 0 {
		return 0
	}

	v, err := strconv.Atoi(envstr)
	if err != nil {
		logrus.WithError(err).Panicf("环境变量 %s 的值 %s 无法转成 int", name, envstr)
	}
	return v
}

func intSliceEnv(name, sep string) []int {
	// todo:
	return nil
}
