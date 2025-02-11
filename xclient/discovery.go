package xclient

import (
	"sync"
)

type SelectStrategy int

const (
	Random SelectStrategy = iota
	RoundRobin
	ConsHash
)

type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(req string) (string, error)
	GetAll() ([]string, error)
}

// 手动维护的discovery
type MultiServerDiscovery struct {
	mu       sync.RWMutex
	strategy LoadBalanceStrategy
	servers  []string
}

func NewMultiServerDiscovery(servers []string, mode SelectStrategy) *MultiServerDiscovery {
	res := &MultiServerDiscovery{}
	switch mode {
	case Random:
		res.strategy = NewLBRandom(servers)
	case RoundRobin:
		res.strategy = NewLBRoundRobin(servers)
	case ConsHash:
		// NOTE: 暂且设置100个虚拟节点
		res.strategy = NewLBConsHash(servers, 100)
	default:
		panic("unsupported strategy")
	}
	res.servers = servers
	return res
}

var _ Discovery = (*MultiServerDiscovery)(nil)

func (d *MultiServerDiscovery) Refresh() error {
	return nil
}

func (d *MultiServerDiscovery) Update(servers []string) error {
	err := d.strategy.UpdateServer(servers)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers

	return nil
}

// 返回选中的server
func (d *MultiServerDiscovery) Get(request string) (string, error) {
	res := d.strategy.GetServer(request)
	return res, nil
}

// 返回所有server
func (d *MultiServerDiscovery) GetAll() ([]string, error) {
	servers := make([]string, len(d.servers))
	copy(servers, d.servers)
	return servers, nil
}
