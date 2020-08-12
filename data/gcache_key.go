package data

import "fmt"

type GCacheKey interface {
	GetPrimary() string
	GetElementPrimary() interface{}
}

func GenerateServerKey(prefix string, uid uint32) string {
	key := fmt.Sprint("%v@%v", prefix, uid)
	return key
}
