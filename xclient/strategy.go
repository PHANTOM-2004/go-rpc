package xclient

import (
	"math/rand"
	"sync"
	"time"
)

type LoadBalanceStrategy interface {
	GetServer(req string) string
	UpdateServer(servers []string) error
}

var _ LoadBalanceStrategy = (*LBRandom)(nil)

type LBRandom struct {
	r       *rand.Rand
	servers []string
	mu      sync.RWMutex
}

func NewLBRandom(servers []string) *LBRandom {
	randSrc := rand.NewSource(time.Now().UnixMicro())
	res := &LBRandom{
		r:       rand.New(randSrc),
		servers: servers,
	}
	return res
}

func (lb *LBRandom) UpdateServer(servers []string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.servers = servers
	return nil
}

func (lb *LBRandom) GetServer(req string) string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	n := len(lb.servers)
	return lb.servers[lb.r.Intn(n)]
}

var _ LoadBalanceStrategy = (*LBRoundRobin)(nil)

type LBRoundRobin struct {
	r       *rand.Rand
	index   int
	servers []string
	mu      sync.RWMutex
}

func NewLBRoundRobin(servers []string) *LBRoundRobin {
	randSrc := rand.NewSource(time.Now().UnixMicro())
	res := &LBRoundRobin{
		r:       rand.New(randSrc),
		servers: servers,
	}
	res.index = res.r.Intn(len(servers))
	return res
}

func (lb *LBRoundRobin) UpdateServer(servers []string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.servers = servers

	return nil
}

func (lb *LBRoundRobin) GetServer(req string) string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	n := len(lb.servers)
	s := lb.servers[lb.index%n]
	// servers could be updated, so mode n to ensure safety
	lb.index = (lb.index + 1) % n
	return s
}
