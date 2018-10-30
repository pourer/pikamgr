package topom

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pourer/pikamgr/config"
	"github.com/pourer/pikamgr/protocol"
	"github.com/pourer/pikamgr/topom/client/redis"
	"github.com/pourer/pikamgr/topom/dao"
	"github.com/pourer/pikamgr/utils/log"
)

var ErrClosedTopom = errors.New("use of closed topom")

type TopomMapper interface {
	Create() error
	Delete() error
	Info() (*dao.Topom, error)
}

type GroupMapper interface {
	Create(group *dao.Group) error
	Update(group *dao.Group) error
	Remove(group *dao.Group) error
	Info() (dao.Groups, error)
}

type SentinelMapper interface {
	Update(sentinel *dao.Sentinel) error
	Info() (*dao.Sentinel, error)
}

type GSLBMapper interface {
	Update(g *dao.GSLB) error
	Delete(g *dao.GSLB) error
	Info() (dao.GSLBs, error)
}

type TemplateFileMapper interface {
	Info() (dao.TemplateFiles, error)
}

type service struct {
	config         *config.DashboardConfig
	topomMapper    TopomMapper
	groupMapper    GroupMapper
	sentinelMapper SentinelMapper
	gslbMapper     GSLBMapper
	tfMapper       TemplateFileMapper

	stats struct {
		redisp  *redis.Pool
		servers map[string]*RedisStats
	}

	ha struct {
		redisp  *redis.Pool
		monitor *redis.Sentinel
		masters map[string]string
	}

	gslbs struct {
		Stats map[string]*GSLBStats
	}

	mutex                   *sync.Mutex
	started, closed, online int32
	done                    chan struct{}
	wg                      sync.WaitGroup
}

func NewService(config *config.DashboardConfig, topomMapper TopomMapper, groupMapper GroupMapper, sentinelMapper SentinelMapper,
	gslbMapper GSLBMapper, tfMapper TemplateFileMapper) (*service, error) {
	s := &service{
		config:         config,
		topomMapper:    topomMapper,
		groupMapper:    groupMapper,
		sentinelMapper: sentinelMapper,
		gslbMapper:     gslbMapper,
		tfMapper:       tfMapper,
		mutex:          new(sync.Mutex),
		done:           make(chan struct{}),
	}

	s.ha.redisp = redis.NewPool("", time.Second*5)
	s.stats.redisp = redis.NewPool(config.ProductAuth, time.Second*5)
	s.stats.servers = make(map[string]*RedisStats)

	return s, nil
}

func (s *service) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil
	}
	close(s.done)
	s.wg.Wait()

	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, p := range []*redis.Pool{
		s.stats.redisp, s.ha.redisp,
	} {
		if p != nil {
			p.Close()
		}
	}

	if err := s.topomMapper.Delete(); err != nil {
		log.Errorln(err, "service::Close delete topom faild. productName:", s.config.ProductName, "err:", err)
		return fmt.Errorf("delete topom faild. productName-[%s]", s.config.ProductName)
	}
	atomic.StoreInt32(&s.online, 0)

	return nil
}

func (s *service) IsOnline() bool {
	return atomic.LoadInt32(&s.online) == 1
}

func (s *service) Start() error {
	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrClosedTopom
	}

	if !atomic.CompareAndSwapInt32(&s.started, 0, 1) {
		return nil
	}

LOOP:
	for {
		select {
		case <-s.done:
			return nil
		default:
		}

		if err := s.topomMapper.Create(); err != nil {
			log.Errorln("service::Start create topom fail. err:", err)
		} else {
			break LOOP
		}

		select {
		case <-s.done:
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	atomic.StoreInt32(&s.online, 1)

	sentinel, err := s.sentinelMapper.Info()
	if err != nil {
		return err
	}
	s.reWatchSentinels(sentinel.Servers)

	s.wg.Add(1)
	go s.doStats(time.Second)
	return nil
}

func (s *service) Overview() (*protocol.Overview, error) {
	topom, err := s.Topom()
	if err != nil {
		return nil, err
	}

	stats, err := s.Stats()
	if err != nil {
		return nil, err
	}

	return &protocol.Overview{
		Version: config.Version,
		Compile: config.Compile,
		Config:  s.config,
		Model:   topom,
		Stats:   stats,
	}, nil
}

func (s *service) Topom() (*protocol.Topom, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	t, err := s.topomMapper.Info()
	if err != nil {
		return nil, err
	}

	return &protocol.Topom{
		StartTime:   t.StartTime,
		AdminAddr:   t.AdminAddr,
		ProductName: t.ProductName,
		Pid:         t.Pid,
		Pwd:         t.Pwd,
		Sys:         t.Sys,
	}, nil
}

func (s *service) Stats() (*protocol.Stats, error) {
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
	gslbs, err := s.gslbMapper.Info()
	if err != nil {
		return nil, err
	}
	tfs, err := s.tfMapper.Info()
	if err != nil {
		return nil, err
	}

	stats := &protocol.Stats{
		Closed: atomic.LoadInt32(&s.closed) == 1,
	}

	sGroups := sortGroups(groups)
	stats.Group.Models = make([]*protocol.Group, 0, len(sGroups))
	for _, g := range sGroups {
		pg := &protocol.Group{
			Name:           g.Name,
			OutOfSync:      g.OutOfSync,
			Servers:        []*protocol.GroupServer{},
			ProxyReadPort:  g.ProxyReadPort,
			ProxyWritePort: g.ProxyWritePort,
		}
		for _, v := range g.Servers {
			pg.Servers = append(pg.Servers, &protocol.GroupServer{
				Addr: v.Addr,
			})
		}
		pg.Promoting.Index = g.Promoting.Index
		pg.Promoting.State = g.Promoting.State
		stats.Group.Models = append(stats.Group.Models, pg)
	}

	stats.Group.Stats = make(map[string]*protocol.RedisStats)
	for _, g := range groups {
		for _, v := range g.Servers {
			if vv := s.stats.servers[v.Addr]; vv != nil {
				pr := &protocol.RedisStats{
					Error:    vv.Error,
					Stats:    vv.Stats,
					Sentinel: make(map[string]*protocol.SentinelGroup),
					UnixTime: vv.UnixTime,
					Timeout:  vv.Timeout,
				}
				for k, v := range vv.Sentinel {
					pr.Sentinel[k] = &protocol.SentinelGroup{
						Master: v.Master,
						Slaves: v.Slaves,
					}
				}
				stats.Group.Stats[v.Addr] = pr
			}
		}
	}

	stats.HA.Model = &protocol.Sentinel{
		Servers:   sentinel.Servers,
		OutOfSync: sentinel.OutOfSync,
	}
	stats.HA.Stats = make(map[string]*protocol.RedisStats)
	for _, server := range sentinel.Servers {
		if vv, ok := s.stats.servers[server]; ok && vv != nil {
			pr := &protocol.RedisStats{
				Error:    vv.Error,
				Stats:    vv.Stats,
				Sentinel: make(map[string]*protocol.SentinelGroup),
				UnixTime: vv.UnixTime,
				Timeout:  vv.Timeout,
			}
			for k, v := range vv.Sentinel {
				pr.Sentinel[k] = &protocol.SentinelGroup{
					Master: v.Master,
					Slaves: v.Slaves,
				}
			}
			stats.HA.Stats[server] = pr
		}
	}
	stats.HA.Masters = make(map[string]string)
	if s.ha.masters != nil {
		for gName, addr := range s.ha.masters {
			stats.HA.Masters[gName] = addr
		}
	}

	stats.GSLB.Models = make(map[string]*protocol.GSLB)
	stats.GSLB.Stats = make(map[string]*protocol.GSLBStats)
	for _, v := range gslbs {
		stats.GSLB.Models[v.Name] = &protocol.GSLB{Servers: v.Servers}
		for _, server := range v.Servers {
			if vv, ok := s.gslbs.Stats[server]; ok && vv != nil {
				stats.GSLB.Stats[server] = &protocol.GSLBStats{
					Error:    vv.Error,
					UnixTime: vv.UnixTime,
					Timeout:  vv.Timeout,
				}
			}
		}
	}

	stats.Template.FileNames = sortTemplateFiles(tfs)

	return stats, nil
}

func (s *service) ViewTemplateFile(fileName string) ([]byte, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tfs, err := s.tfMapper.Info()
	if err != nil {
		return nil, err
	}

	tf, ok := tfs[fileName]
	if !ok {
		return nil, fmt.Errorf("templateFile-[%s] not found", fileName)
	}

	return tf.Data, nil
}

func (s *service) doStats(timeout time.Duration) {
	defer s.wg.Done()

	for {
		select {
		case <-s.done:
			return
		default:
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()

			w, err := s.refreshRedisStats(timeout)
			if err != nil {
				log.Errorln("service::doStats refreshRedisStats fail. err:", err)
			}
			if w != nil {
				w.Wait()
			}
		}()
		go func() {
			defer wg.Done()

			w, err := s.refreshGSLBStats(timeout)
			if err != nil {
				log.Errorln("service::doStats refreshGSLBStats fail. err:", err)
			}
			if w != nil {
				w.Wait()
			}
		}()

		wg.Wait()

		s.mutex.Lock()
		if err := s.refreshGSLBBackendInfo(); err != nil {
			log.Errorln("service::doStats refreshGSLBBackendInfo fail. err:", err)
		}
		s.mutex.Unlock()

		select {
		case <-s.done:
			return
		case <-time.After(timeout):
		}
	}
}
