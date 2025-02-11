package xclient

import (
	"hash/crc32"
	"math/rand"
	"sort"
	"strconv"
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

var _ LoadBalanceStrategy = (*LBConsHash)(nil)

type LBConsHash struct {
	r            *rand.Rand
	mu           sync.RWMutex
	circle       map[uint32]string
	sortedHashes []uint32
	vnodesCnt    int
}

// 一种naive的实现
func NewLBConsHash(servers []string, vnodesCnt int) *LBConsHash {
	lb := &LBConsHash{
		circle:    make(map[uint32]string),
		vnodesCnt: vnodesCnt,
	}
	for _, node := range servers {
		lb.addNode(node)
	}
	sort.Slice(lb.sortedHashes, func(i, j int) bool {
		return lb.sortedHashes[i] < lb.sortedHashes[j]
	})

	return lb
}

func (lb *LBConsHash) updateNodes(addr []string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.circle = make(map[uint32]string)
	lb.sortedHashes = make([]uint32, 0)

	for _, node := range addr {
		lb.addNode(node)
	}
	sort.Slice(lb.sortedHashes, func(i, j int) bool {
		return lb.sortedHashes[i] < lb.sortedHashes[j]
	})
}

func (lb *LBConsHash) addNode(addr string) {
	for i := 0; i < lb.vnodesCnt; i++ {
		virtualNode := addr + "#" + strconv.Itoa(i)
		hash := crc32.ChecksumIEEE([]byte(virtualNode))
		lb.circle[hash] = addr
		lb.sortedHashes = append(lb.sortedHashes, hash)
	}
}

func (lb *LBConsHash) GetServer(req string) string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	hash := crc32.ChecksumIEEE([]byte(req))
	idx := sort.Search(len(lb.sortedHashes), func(i int) bool {
		return lb.sortedHashes[i] >= hash
	})
	if idx >= len(lb.sortedHashes) {
		idx = 0
	}
	return lb.circle[lb.sortedHashes[idx]]
}

// 暂且保持这样的接口, 实际上更好的方式是增删, 但是暂且这样,
// 毕竟是一个demo
func (lb *LBConsHash) UpdateServer(servers []string) error {
	lb.updateNodes(servers)
	return nil
}
