package coordinate

import (
	"fmt"
	"time"

	"github.com/pourer/pikamgr/coordinate/etcd"
	"github.com/pourer/pikamgr/coordinate/zk"
)

type Client interface {
	Create(path string, data []byte) error
	Update(path string, data []byte) error
	Delete(path string) error

	Read(path string, must bool) ([]byte, error)
	List(path string, must bool) ([]string, error)

	Close() error

	WatchInOrder(path string) (<-chan struct{}, []string, error)

	CreateEphemeral(path string, data []byte) (<-chan struct{}, error)
	CreateEphemeralInOrder(path string, data []byte) (<-chan struct{}, string, error)
}

func NewCoordinator(coordinator string, addrlist string, auth string, timeout time.Duration) (Client, error) {
	switch coordinator {
	case "zk", "zookeeper":
		return zk.New(addrlist, auth, timeout)
	case "etcd":
		return etcd.New(addrlist, auth, timeout)
	}
	return nil, fmt.Errorf("invalid coordinator name:%s", coordinator)
}
