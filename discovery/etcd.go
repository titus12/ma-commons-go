// etcd的服务发现
// +build etcd

package discovery

type etcdimpl struct {
}

func newImpl() *etcdimpl {
	return nil
}

func (k8s *etcdimpl) Listen() <-chan *Node {

	return nil
}

func (k8s *etcdimpl) Stop() <-chan struct{} {

	return nil
}

func (k8s *etcdimpl) IsStop() bool {
	return false
}
