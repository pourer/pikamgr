package topom

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/pourer/pikamgr/topom/client/redis"
	"github.com/pourer/pikamgr/topom/dao"
	swerror "github.com/pourer/pikamgr/utils/error"
	"github.com/pourer/pikamgr/utils/log"

	"github.com/CodisLabs/codis/pkg/utils/sync2"
)

func (s *service) CreateGroup(groupName string, rPort, wPort int) error {
	if groupName == "" || utf8.RuneCountInString(groupName) > dao.MAXGroupNameBytesLength {
		return fmt.Errorf("invalid group name = %s, out of range", groupName)
	}
	if rPort == wPort {
		return errors.New("Proxy-Read-Port and Proxy-Write-Port must be not equal")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	if groups[groupName] != nil {
		return fmt.Errorf("group-[%s] already exists", groupName)
	}

	for _, g := range groups {
		if (rPort == g.ProxyReadPort || rPort == g.ProxyWritePort) ||
			(wPort == g.ProxyReadPort || wPort == g.ProxyWritePort) {
			return fmt.Errorf("group-[%s] and group-[%s] port conflict", groupName, g.Name)
		}
	}

	g := &dao.Group{
		Name:           groupName,
		Servers:        []*dao.GroupServer{},
		ProxyReadPort:  rPort,
		ProxyWritePort: wPort,
		CreateTime:     time.Now().Format("2006-01-02 15:04:05"),
	}
	return s.groupMapper.Create(g)
}

func (s *service) RemoveGroup(groupName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	g, ok := groups[groupName]
	if !ok {
		return fmt.Errorf("group-[%s] not found", groupName)
	}
	if len(g.Servers) != 0 {
		return fmt.Errorf("group-[%s] isn't empty", groupName)
	}

	return s.groupMapper.Remove(g)
}

func (s *service) ResyncGroup(groupName string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	g, ok := groups[groupName]
	if !ok {
		return fmt.Errorf("group-[%s] not found", groupName)
	}

	g.OutOfSync = false
	if err := s.groupMapper.Update(g); err != nil {
		return err
	}
	if err := s.resyncGroup(g); err != nil {
		return err
	}

	return nil
}

func (s *service) ResyncGroupAll() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	var fut sync2.Future
	for _, g := range groups {
		fut.Add()
		go func(g *dao.Group) {
			g.OutOfSync = false
			if err := s.groupMapper.Update(g); err != nil {
				fut.Done(g.Name, fmt.Errorf("resync group-[%s] failed.err:%s", g.Name, err.Error()))
			} else {
				if err := s.resyncGroup(g); err != nil {
					fut.Done(g.Name, err)
				}
			}
		}(g)
	}

	for _, v := range fut.Wait() {
		switch err := v.(type) {
		case error:
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *service) AddGroupServer(groupName, addr string) error {
	if addr == "" {
		return errors.New("invalid server address")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	for _, g := range groups {
		for _, v := range g.Servers {
			if v.Addr == addr {
				return fmt.Errorf("server-[%s] already exists", addr)
			}
		}
	}

	g, ok := groups[groupName]
	if !ok {
		return fmt.Errorf("group-[%s] not found", groupName)
	}

	if err := s.outOfSyncBySentinel(); err != nil {
		return err
	}

	g.Servers = append(g.Servers, &dao.GroupServer{Addr: addr})
	if err := s.groupMapper.Update(g); err != nil {
		return err
	}
	return s.refreshGSLBBackendInfo()
}

func (s *service) DelGroupServer(groupName, addr string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	g, ok := groups[groupName]
	if !ok {
		return fmt.Errorf("group-[%s] not found", groupName)
	}

	var index = g.GetServerIndex(addr)
	if index == -1 {
		return fmt.Errorf("group-[%s] doesn't have server-[%s]", groupName, addr)
	}
	if index == 0 && len(g.Servers) > 1 {
		return fmt.Errorf("group-[%s] can't remove master, still in use", groupName)
	}

	if err := s.outOfSyncBySentinel(); err != nil {
		return err
	}

	if index != 0 {
		g.OutOfSync = true
	}
	g.Servers = append(g.Servers[:index], g.Servers[index+1:]...)
	if len(g.Servers) == 0 {
		g.OutOfSync = false
	}

	if err := s.groupMapper.Update(g); err != nil {
		return err
	}
	return s.refreshGSLBBackendInfo()
}

func (s *service) GroupPromoteServer(groupName, addr string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	sentinel, err := s.sentinelMapper.Info()
	if err != nil {
		return err
	}

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	g, ok := groups[groupName]
	if !ok {
		return fmt.Errorf("group-[%s] not found", groupName)
	}

	var index = g.GetServerIndex(addr)
	if index == -1 {
		return fmt.Errorf("group-[%s] doesn't have server-[%s]", groupName, addr)
	}

	if g.Promoting.State != dao.ActionNothing {
		if index != g.Promoting.Index {
			return fmt.Errorf("group-[%s] is promoting index = %d", g.Name, g.Promoting.Index)
		}
	} else {
		if index == 0 {
			return fmt.Errorf("group-[%s] can't promote master", g.Name)
		}
	}

	switch g.Promoting.State {
	case dao.ActionNothing:
		{
			g.Promoting.Index = index
			g.Promoting.State = dao.ActionPreparing
			if err := s.groupMapper.Update(g); err != nil {
				return err
			}
		}
		fallthrough
	case dao.ActionPreparing:
		{
			g.Promoting.State = dao.ActionPrepared
			if err := s.groupMapper.Update(g); err != nil {
				return err
			}
		}
		fallthrough
	case dao.ActionPrepared:
		{
			if len(sentinel.Servers) > 0 {
				sentinel.OutOfSync = true
				if err := s.sentinelMapper.Update(sentinel); err != nil {
					return err
				}

				sentinelClient := redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
				if err := sentinelClient.RemoveGroups(sentinel.Servers, s.config.SentinelClientTimeout.Duration(), map[string]bool{g.Name: true}); err != nil {
					log.Warnln("service::GroupPromoteServer sentinel RemoveGroups failed. sentinel-addrs:", sentinel.Servers, "groupName:", g.Name, "err:", err)
				}
				if s.ha.masters != nil {
					delete(s.ha.masters, g.Name)
				}
			}

			g.Servers[0], g.Servers[g.Promoting.Index] = g.Servers[g.Promoting.Index], g.Servers[0]
			g.Promoting.Index = 0
			g.Promoting.State = dao.ActionFinished
			if err := s.groupMapper.Update(g); err != nil {
				return err
			}

			if err := s.resyncGroup(g); err != nil {
				log.Errorln("service::GroupPromoteServer doSyncAction failed. err:", err)
			}
		}
		fallthrough
	case dao.ActionFinished:
		{
			g = &dao.Group{
				Name:           g.Name,
				Servers:        g.Servers,
				ProxyReadPort:  g.ProxyReadPort,
				ProxyWritePort: g.ProxyWritePort,
				CreateTime:     g.CreateTime,
			}
			return s.groupMapper.Update(g)
		}
	default:
		return fmt.Errorf("group-[%s] action state is invalid", g.Name)
	}
}

func (s *service) GroupForceFullSyncServer(groupName, addr string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return err
	}

	g, ok := groups[groupName]
	if !ok {
		return fmt.Errorf("group-[%s] not found", groupName)
	}

	var index = g.GetServerIndex(addr)
	if index == -1 {
		return fmt.Errorf("group-[%s] doesn't have server-[%s]", groupName, addr)
	} else if index == 0 {
		return fmt.Errorf("group-[%s] master server-[%s] not allowed this operation", groupName, addr)
	}

	if g.Promoting.State != dao.ActionNothing {
		return fmt.Errorf("group-[%s] is promoting", g.Name)
	}

	return s.doForceFullSyncAction(g.Servers[index].Addr, g.Servers[0].Addr)
}

func (s *service) resyncGroup(g *dao.Group) error {
	if len(g.Servers) == 0 {
		return nil
	}

	var multiErr swerror.MultiError
	for index, server := range g.Servers {
		master := g.Servers[0].Addr
		if index == 0 {
			master = "NO:ONE"
		}

		if err := s.doSyncAction(server.Addr, master); err != nil {
			multiErr.Append(fmt.Errorf("service::resyncGroup doSyncAction failed. groupName:%s addr:%s master:%s error:%s", g.Name, server.Addr, master, err.Error()))
		}
	}

	if multiErr.ErrorOrNil() != nil {
		g.OutOfSync = true
		if err := s.groupMapper.Update(g); err != nil {
			multiErr.Append(err)
		}
	}
	return multiErr.ErrorOrNil()
}

func (s *service) doSyncAction(addr, master string) error {
	c, err := redis.NewClient(addr, s.config.ProductAuth, 10*time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.SetMaster(master); err != nil {
		return err
	}

	return nil
}

func (s *service) doForceFullSyncAction(addr, master string) error {
	c, err := redis.NewClient(addr, s.config.ProductAuth, 10*time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.ForceFullSyncFromMaster(master); err != nil {
		return err
	}

	return nil
}

func (s *service) ServerInfo(addr string) ([]byte, error) {
	c, err := redis.NewClient(addr, s.config.ProductAuth, 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	if err := c.EnableKeySpace(); err != nil {
		return nil, err
	}
	if text, _, err := c.InfoFull(); err != nil {
		return nil, err
	} else {
		return []byte(text), nil
	}
}

func (s *service) Info() (interface{}, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	groups, err := s.groupMapper.Info()
	if err != nil {
		return nil, err
	}

	return groups, nil
}
