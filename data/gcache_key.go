package data

import "fmt"

type GCacheKey interface {
	GetPrimary() string
	GetElementPrimary() interface{}
}

type GCacheTKey struct {
	primary        string
	elementPrimary interface{}
}

func NewGCacheTKey(primary string, elementPrimary interface{}) *GCacheTKey {
	return &GCacheTKey{primary: primary, elementPrimary: elementPrimary}
}

func (k *GCacheTKey) GetPrimary() string {
	return k.primary
}

func (k *GCacheTKey) GetElementPrimary() interface{} {
	return k.elementPrimary
}

func (k *GCacheTKey) String() string {
	return fmt.Sprintf("primary %v, elementPrimary %v", k.primary, k.elementPrimary)
}

func GenerateServerKey(prefix string, uid uint32) string {
	key := fmt.Sprint("%v@%v", prefix, uid)
	return key
}
