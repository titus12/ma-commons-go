package utils

import "strings"

// 剔除掉前后空格，以及 制表符 回车符 换行符
func Trim(s string) string {
	return strings.Trim(s, " \t\r\n")
}
