package services

import (
	"errors"
	etcdclient "github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"sync"
	"time"
)

var (
	HttpRetryTimes                      = 3
	DefaultTTL            time.Duration = 30
	NextRequestWaitSecond time.Duration = 1 * time.Second
)

type Mutex struct {
	LockPath string             // the key stored in etcd
	UserId   string             // the identifier of user
	Client   etcdclient.Client  // etcd client
	KeysAPI  etcdclient.KeysAPI // the interface of etcd client's method (set | get | watch)
	Context  context.Context    // context interface
	TTL      time.Duration      // the life cycle of this key
	Mu       sync.Mutex         // local mutex
}

func new(lockPath string, nodeId string, client etcdclient.Client) (*Mutex, error) {
	if len(lockPath) == 0 || lockPath[0] != '/' {
		return nil, errors.New("lockPath can not be null and should be started with '/'")
	}

	return &Mutex{
		LockPath: lockPath,
		UserId:   nodeId,
		Client:   client,
		KeysAPI:  etcdclient.NewKeysAPI(client),
		Context:  context.TODO(),
		TTL:      DefaultTTL,
	}, nil
}

func (m *Mutex) TryLock() (err error) {
	m.Mu.Lock()
	return m.tryLock()
}

func (m *Mutex) Lock() (err error) {
	m.Mu.Lock()
	for retry := 0; retry <= HttpRetryTimes; retry++ {
		if err = m.lock(); err == nil {
			return nil
		}
		time.Sleep(NextRequestWaitSecond)
	}
	return err
}

func (m *Mutex) tryLock() (err error) {
	options := &etcdclient.SetOptions{
		PrevExist: etcdclient.PrevNoExist,
		TTL:       m.TTL,
	}

	if _, err := m.KeysAPI.Set(m.Context, m.LockPath, m.UserId, options); err != nil {
		return err
	} else {
		return nil
	}
}

func (m *Mutex) lock() (err error) {
	options := &etcdclient.SetOptions{
		PrevExist: etcdclient.PrevNoExist,
		TTL:       m.TTL,
	}

	resp, err := m.KeysAPI.Set(m.Context, m.LockPath, m.UserId, options)
	// transfer err to client.Error
	e, ok := err.(etcdclient.Error)
	if !ok {
		return err
	}

	if e.Code != etcdclient.ErrorCodeNodeExist {
		// this key not existed, but create failed, return error
		return err
	}

	// this key has existed, watch this key
	if resp, err = m.KeysAPI.Get(m.Context, m.LockPath, nil); err != nil {
		return err
	}

	watcherOptions := &etcdclient.WatcherOptions{
		AfterIndex: resp.Index,
		Recursive:  false,
	}

	watcher := m.KeysAPI.Watcher(m.LockPath, watcherOptions)
	for {
		if _, err := watcher.Next(m.Context); err != nil {
			return err
		}
		if resp.Action == "delete" || resp.Action == "expire" {
			// try to create this key again
			if _, err := m.KeysAPI.Set(m.Context, m.LockPath, m.UserId, options); err != nil {
				return err
			} else {
				return nil
			}
		}
	}
}

func (m *Mutex) UnLock() (err error) {
	defer m.Mu.Unlock()
	for retry := 0; retry < HttpRetryTimes; retry++ {
		_, err := m.KeysAPI.Delete(m.Context, m.LockPath, nil)
		if err != nil {
			time.Sleep(NextRequestWaitSecond)
		} else {
			return nil
		}
	}
	return err
}
