package topom

import (
	"time"

	"github.com/pourer/pikamgr/topom/client/gslb"
	"github.com/pourer/pikamgr/topom/client/redis"

	"github.com/CodisLabs/codis/pkg/utils/sync2"
)

type RedisStats struct {
	Error    error
	Stats    map[string]string
	Sentinel map[string]*redis.SentinelGroup
	UnixTime int64
	Timeout  bool
}

func (s *RedisStats) MasterAddr() string {
	addr, ok := s.Stats["master_addr"]
	if ok {
		return addr
	}
	return ""
}

const (
	MasterLinkStatusUp   = "up"
	MasterLinkStatusDown = "down"
)

func (s *RedisStats) MasterLinkStatus() string {
	addr, ok := s.Stats["master_link_status"]
	if ok {
		return addr
	}
	return ""
}

type GSLBStats struct {
	Error    error
	UnixTime int64
	Timeout  bool
}

func (s *service) newRedisStats(addr string, timeout time.Duration, do func(addr string) (*RedisStats, error)) *RedisStats {
	var ch = make(chan struct{})
	stats := &RedisStats{}

	go func() {
		defer close(ch)
		p, err := do(addr)
		if err != nil {
			stats.Error = err
		} else {
			stats.Stats, stats.Sentinel = p.Stats, p.Sentinel
		}
	}()

	select {
	case <-ch:
		return stats
	case <-time.After(timeout):
		return &RedisStats{Timeout: true}
	}
}

func (s *service) refreshRedisStats(timeout time.Duration) (*sync2.Future, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return nil, err
	}

	sentinel, err := s.sentinelMapper.Info()
	if err != nil {
		return nil, err
	}

	var fut sync2.Future
	goStats := func(addr string, do func(addr string) (*RedisStats, error)) {
		fut.Add()
		go func() {
			stats := s.newRedisStats(addr, timeout, do)
			stats.UnixTime = time.Now().Unix()
			fut.Done(addr, stats)
		}()
	}
	for _, g := range groups {
		for _, x := range g.Servers {
			goStats(x.Addr, func(addr string) (*RedisStats, error) {
				_, m, err := s.stats.redisp.InfoFull(addr)
				if err != nil {
					return nil, err
				}
				return &RedisStats{Stats: m}, nil
			})
		}
	}
	for _, server := range sentinel.Servers {
		goStats(server, func(addr string) (*RedisStats, error) {
			c, err := s.ha.redisp.GetClient(addr)
			if err != nil {
				return nil, err
			}
			defer s.ha.redisp.PutClient(c)
			_, m, err := c.Info()
			if err != nil {
				return nil, err
			}
			sentinel := redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
			p, err := sentinel.MastersAndSlavesClient(c)
			if err != nil {
				return nil, err
			}
			return &RedisStats{Stats: m, Sentinel: p}, nil
		})
	}

	go func() {
		stats := make(map[string]*RedisStats)
		for k, v := range fut.Wait() {
			stats[k] = v.(*RedisStats)
		}

		s.mutex.Lock()
		s.stats.servers = stats
		s.mutex.Unlock()
	}()

	return &fut, nil
}

func (s *service) newGSLBStats(addr string, timeout time.Duration, do func(addr string) (*GSLBStats, error)) *GSLBStats {
	var ch = make(chan struct{})
	stats := &GSLBStats{}

	go func() {
		defer close(ch)
		_, err := do(addr)
		if err != nil {
			stats.Error = err
		}
	}()

	select {
	case <-ch:
		return stats
	case <-time.After(timeout):
		return &GSLBStats{Timeout: true}
	}
}

func (s *service) refreshGSLBStats(timeout time.Duration) (*sync2.Future, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	gslbs, err := s.gslbMapper.Info()
	if err != nil {
		return nil, err
	}

	var fut sync2.Future
	goStats := func(addr string, do func(addr string) (*GSLBStats, error)) {
		fut.Add()
		go func() {
			stats := s.newGSLBStats(addr, timeout, do)
			stats.UnixTime = time.Now().Unix()
			fut.Done(addr, stats)
		}()
	}

	for _, g := range gslbs {
		for _, addr := range g.Servers {
			goStats(addr, func(addr string) (*GSLBStats, error) {
				_, err := gslb.NewClient(addr, timeout).Info()
				if err != nil {
					return nil, err
				}
				return &GSLBStats{}, nil
			})
		}
	}

	go func() {
		stats := make(map[string]*GSLBStats)
		for k, v := range fut.Wait() {
			stats[k] = v.(*GSLBStats)
		}

		s.mutex.Lock()
		s.gslbs.Stats = stats
		s.mutex.Unlock()
	}()

	return &fut, nil
}
