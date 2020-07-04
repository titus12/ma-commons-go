package utils

import (
	"regexp"
)

var (
	chineseName *regexp.Regexp
	email       *regexp.Regexp
	mobilePhone *regexp.Regexp
	onlyChinese *regexp.Regexp
	ip          *regexp.Regexp
	ipAndPort   *regexp.Regexp
	lanIp       *regexp.Regexp
)

func init() {
	/*chineseName = regexp.MustCompile(`^([\u4e00-\u9fa5][·\u4e00-\u9fa5]{0,30}[\u4e00-\u9fa5])$`)
	email = regexp.MustCompile(`[\w+\.]+[\w+]+@+[0-9A-Za-z]+\.+[A-Za-z]+$`)
	mobilePhone = regexp.MustCompile(`^((\+86)|(86))?(-|\s)?1\d{10}$`)
	onlyChinese = regexp.MustCompile(`^[\u4e00-\u9fa5]+$`)
	lanIp = regexp.MustCompile(`^(127\.0\.0\.1)|(localhost)|(10\.\d{1,3}\.\d{1,3}\.\d{1,3})|(172\.((1[6-9])|(2\d)|(3[01]))\.\d{1,3}\.\d{1,3})|(192\.168\.\d{1,3}\.\d{1,3})`)
	ipAndPort = regexp.MustCompile(`\d{1,3}(\.\d{1,3}){3}:\d+$`)
	ip = regexp.MustCompile(`\d{1,3}(\.\d{1,3}){3}$`)*/
}

func IsChineseName(str string) bool {
	return chineseName.MatchString(str)
}

func IsEmail(str string) bool {
	return email.MatchString(str)
}

func IsMobilePhone(str string) bool {
	return mobilePhone.MatchString(str)
}

func IsOnlyChinese(str string) bool {
	return onlyChinese.MatchString(str)
}

// 是否为一个内网ip
func IsLanIp(str string) bool {
	if ip.MatchString(str) {
		return lanIp.MatchString(str)
	}
	return false
}

// 是否为一个IP
func IsIp(str string) bool {
	return ip.MatchString(str)
}

func IsIpAndPort(str string) bool {
	return ipAndPort.MatchString(str)
}
