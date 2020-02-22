package data

type GCacheKey interface {
	GetPrimary() string
	GetElementPrimary() interface{}
}
