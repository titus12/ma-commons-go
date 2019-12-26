package gcache

type GCacheKey interface {
	GetPrimary() string
	GetElementPrimary() interface{}
}
