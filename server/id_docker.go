// +build docker

package server

import (
	"bufio"
	"bytes"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// 获取docker容器id
func idInitImpl() {
	file, err := os.Open("/proc/self/cgroup")
	if err != nil {
		logrus.WithError(err).Panicf("读取Docker容器id失败")
	}

	defer file.Close()
	line := bufio.NewReader(file)
	var content []byte
	for {
		content, _, err = line.ReadLine()
		if err == io.EOF {
			break
		}

		// 容器id是64位的
		if len(content) > 64 {
			break
		}
	}

	if len(content) < 64 {
		logrus.Panicf("没有获取到Docker容器信息...content: %s", content)
	}

	idx := bytes.LastIndexByte(content, '/')
	if idx < 0 {
		logrus.Panicf("没有找到Docker容器id...content: %s", content)
	}

	ID = string(content[idx+1 : idx+13])
	logrus.Infof("容器ID: %s\n", ID)
}
