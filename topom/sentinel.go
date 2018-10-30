package topom

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pourer/pikamgr/topom/client/redis"
	"github.com/pourer/pikamgr/utils/log"

	"github.com/CodisLabs/codis/pkg/utils/math2"
)

func (s *service) AddSentinel(addr string) error {
	if len(addr) == 0 {
		return errors.New("invalid sentinel address")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	sentinel, err := s.sentinelMapper.Info()
	if err != nil {
		return err
	}

	for _, v := range sentinel.Servers {
		if v == addr {
			return fmt.Errorf("sentinel-[%s] already exists", addr)
		}
	}

	sentinelClient := redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
	if err := sentinelClient.FlushConfig(addr, s.config.SentinelClientTimeout.Duration()); err != nil {
		return err
	}

	sentinel.Servers = append(sentinel.Servers, addr)
	sentinel.OutOfSync = true
	return s.sentinelMapper.Update(sentinel)
}

func (s *service) DelSentinel(addr string, force bool) error {
	if len(addr) == 0 {
		return errors.New("invalid sentinel address")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	sentinel, err := s.sentinelMapper.Info()
	if err != nil {
		return err
	}

	index := -1
	for i, v := range sentinel.Servers {
		if v == addr {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("sentinel-[%s] not found", addr)
	}

	sentinel.OutOfSync = true
	if err := s.sentinelMapper.Update(sentinel); err != nil {
		return err
	}

	sentinelClient := redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
	if err := sentinelClient.RemoveGroupsAll([]string{addr}, s.config.SentinelClientTimeout.Duration()); err != nil {
		log.Warnln("service::DelSentinel remove sentinel", addr, "failed. err:", err)
		if !force {
			return fmt.Errorf("remove sentinel %s failed", addr)
		}
	}

	sentinel.Servers = append(sentinel.Servers[:index], sentinel.Servers[index+1:]...)
	return s.sentinelMapper.Update(sentinel)
}

func (s *service) ResyncSentinels() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	sentinel, err := s.sentinelMapper.Info()
	if err != nil {
		return err
	}
	sentinel.OutOfSync = true
	if err := s.sentinelMapper.Update(sentinel); err != nil {
		return err
	}

	config := &redis.MonitorConfig{
		Quorum:               s.config.SentinelQuorum,
		ParallelSyncs:        s.config.SentinelParallelSyncs,
		DownAfter:            s.config.SentinelDownAfter.Duration(),
		FailoverTimeout:      s.config.SentinelFailoverTimeout.Duration(),
		NotificationScript:   s.config.SentinelNotificationScript,
		ClientReconfigScript: s.config.SentinelClientReconfigScript,
	}

	sentinelClient := redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
	if err := sentinelClient.RemoveGroupsAll(sentinel.Servers, s.config.SentinelClientTimeout.Duration()); err != nil {
		log.Errorln("service::ResyncSentinels remove sentinels failed. err:", err)
	}
	if err := sentinelClient.MonitorGroups(sentinel.Servers, s.config.SentinelClientTimeout.Duration(), config, groups.GetMasters()); err != nil {
		log.Errorln("service::ResyncSentinels resync sentinels failed. err:", err)
		return err
	}
	s.reWatchSentinels(sentinel.Servers)

	sentinel.OutOfSync = false
	return s.sentinelMapper.Update(sentinel)
}

func (s *service) SentinelInfo(addr string) ([]byte, error) {
	c, err := redis.NewClientNoAuth(addr, time.Second)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	if text, _, err := c.Info(); err != nil {
		return nil, err
	} else {
		return []byte(text), nil
	}
}

func (s *service) SentinelMonitoredInfo(addr string) (interface{}, error) {
	sentinel := redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
	if info, err := sentinel.MastersAndSlaves(addr, s.config.SentinelClientTimeout.Duration()); err != nil {
		return nil, err
	} else {
		return info, nil
	}
}

func (s *service) reWatchSentinels(servers []string) {
	if s.ha.monitor != nil {
		s.ha.monitor.Cancel()
		s.ha.monitor = nil
	}

	if len(servers) == 0 {
		s.ha.masters = nil
	} else {
		s.ha.monitor = redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
		s.ha.monitor.LogFunc = log.Warnf
		s.ha.monitor.ErrFunc = log.Errorf

		go func(p *redis.Sentinel) {
			var trigger = make(chan struct{}, 1)
			delayUntil := func(deadline time.Time) {
				for !p.IsCanceled() {
					var d = deadline.Sub(time.Now())
					if d <= 0 {
						return
					}
					time.Sleep(math2.MinDuration(d, time.Second))
				}
			}

			go func() {
				defer close(trigger)
				callback := func() {
					select {
					case trigger <- struct{}{}:
					default:
					}
				}
				for !p.IsCanceled() {
					timeout := time.Minute * 15
					retryAt := time.Now().Add(time.Second * 10)
					if !p.Subscribe(servers, timeout, callback) {
						delayUntil(retryAt)
					} else {
						callback()
					}
				}
			}()

			go func() {
				for range trigger {
					var success int
					for i := 0; i != 10 && !p.IsCanceled() && success != 2; i++ {
						timeout := time.Second * 5
						masters, err := p.Masters(servers, timeout)
						if err != nil {
							log.Errorln("service::reWatchSentinels fetch group masters failed. err:", err)
						} else {
							if !p.IsCanceled() {
								s.switchMasters(masters)
							}
							success += 1
						}
						delayUntil(time.Now().Add(time.Second * 5))
					}
				}
			}()
		}(s.ha.monitor)
	}

	log.Infoln("service::reWatchSentinels reWatch sentinels:", servers)
}

func (s *service) switchMasters(masters map[string]string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if atomic.LoadInt32(&s.closed) == 1 {
		return ErrClosedTopom
	}

	s.ha.masters = masters
	if len(masters) > 0 {
		cache := &redis.InfoCache{
			Auth:    s.config.ProductAuth,
			Timeout: 100 * time.Millisecond,
		}

		for groupName, masterAddr := range masters {
			if err := s.trySwitchGroupMaster(groupName, masterAddr, cache); err != nil {
				log.Errorln("service::SwitchMasters sentinel switch group master failed. err:", err)
			}
		}
	}

	return nil
}

func (s *service) trySwitchGroupMaster(groupName, masterAddr string, cache *redis.InfoCache) error {
	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	g, ok := groups[groupName]
	if !ok {
		return fmt.Errorf("group-[%s] not found", groupName)
	}

	var index = func() int {
		for i, v := range g.Servers {
			if v.Addr == masterAddr {
				return i
			}
		}
		for i, v := range g.Servers {
			rid1 := cache.GetRunId(masterAddr)
			rid2 := cache.GetRunId(v.Addr)
			if rid1 != "" && rid1 == rid2 {
				return i
			}
		}
		return -1
	}()
	if index == -1 {
		return fmt.Errorf("group-[%s] doesn't have server %s with runid = '%s'", groupName, masterAddr, cache.GetRunId(masterAddr))
	}
	if index == 0 {
		return nil
	}

	log.Warnf("group-[%s] will switch master to server[%d] = %s", groupName, index, g.Servers[index].Addr)

	g.Servers[0], g.Servers[index] = g.Servers[index], g.Servers[0]
	g.OutOfSync = true
	if err := s.groupMapper.Update(g); err != nil {
		return err
	}
	return s.refreshGSLBBackendInfo()
}

func (s *service) outOfSyncBySentinel() error {
	sentinel, err := s.sentinelMapper.Info()
	if err == nil && len(sentinel.Servers) != 0 {
		sentinel.OutOfSync = true
		if err := s.sentinelMapper.Update(sentinel); err != nil {
			return fmt.Errorf("update sentinel fail. err:%s", err.Error())
		}
	}

	return nil
}
