package topom

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/pourer/pikamgr/topom/client/gslb"
	"github.com/pourer/pikamgr/topom/dao"
	"github.com/pourer/pikamgr/utils/log"
)

func (s *service) AddGSLB(gslbName, addr string) error {
	if len(addr) == 0 {
		return errors.New("invalid gslb address")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	gslbs, err := s.gslbMapper.Info()
	if err != nil {
		return err
	}

	g, ok := gslbs[gslbName]
	if !ok {
		g = &dao.GSLB{Name: gslbName}
		gslbs[gslbName] = g
	}
	for _, v := range g.Servers {
		if v == addr {
			return fmt.Errorf("gslbName-[%s] server-[%s] already exists", gslbName, addr)
		}
	}

	//_, err = gslb.NewClient(addr, time.Second).Info()
	//if err != nil {
	//	return err
	//}

	g.Servers = append(g.Servers, addr)
	if err := s.gslbMapper.Update(g); err != nil {
		return err
	}
	return s.refreshGSLBBackendInfo()
}

func (s *service) DelGSLB(gslbName, addr string) error {
	if len(addr) == 0 {
		return errors.New("invalid gslb address")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	gslbs, err := s.gslbMapper.Info()
	if err != nil {
		return err
	}

	g, ok := gslbs[gslbName]
	if !ok {
		return fmt.Errorf("gslbName-[%s] not found", gslbName)
	}

	index := -1
	for i, v := range g.Servers {
		if v == addr {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("gslbName-[%s] server-[%s] not found", gslbName, addr)
	}
	g.Servers = append(g.Servers[:index], g.Servers[index+1:]...)

	if err := s.gslbMapper.Update(g); err != nil {
		return err
	}
	if len(g.Servers) == 0 {
		if err := s.gslbMapper.Delete(g); err != nil {
			return err
		}
	}
	return s.refreshGSLBBackendInfo()
}

func (s *service) GSLBMonitorInfo(addr string) ([]byte, error) {
	c := gslb.NewClient(addr, time.Second)
	if text, err := c.Info(); err != nil {
		return nil, err
	} else {
		return text, nil
	}
}

func (s *service) refreshGSLBBackendInfo() error {
	gslbs, err := s.gslbMapper.Info()
	if err != nil {
		return err
	}

	for name, v := range gslbs {
		backends, monitors, err := s.getGSLBBackends(name)
		if err != nil {
			log.Errorf("service::refreshGSLBBackendInfo getGSLBBackends fail. gslbname-[%s] err-[%s]", name, err)
			return err
		}
		g := &dao.GSLB{
			Name:     name,
			Servers:  v.Servers,
			Monitors: monitors,
			Backends: backends,
		}

		if err := s.gslbMapper.Update(g); err != nil {
			log.Errorln("service::refreshGSLBBackendInfo update fail. err:", err)
		}
	}

	return nil
}

func (s *service) getGSLBBackends(gslbName string) (dao.GSLBBackendGroups, dao.GSLBMonitors, error) {
	switch gslbName {
	case "haproxy":
		return s.haproxyBackends("")
	case "lvs":
		return s.lvsBackends("haproxy")
	default:
		return nil, nil, fmt.Errorf("unsupported gslb type. gslbName:%s", gslbName)
	}
}

func (s *service) haproxyBackends(backendName string) (dao.GSLBBackendGroups, dao.GSLBMonitors, error) {
	if len(s.stats.servers) == 0 {
		return nil, nil, errors.New("redis stats empty")
	}

	groups, err := s.groupMapper.Info()
	if err != nil {
		return nil, nil, err
	}

	var backends dao.GSLBBackendGroups
	for _, v := range sortGroups(groups) {
		if len(v.Servers) == 0 {
			continue
		}

		bg := &dao.GSLBBackendGroup{
			Name:        v.Name,
			ServerGroup: make(dao.GSLBBackends),
		}

		bValid := true
		for i, server := range v.Servers {
			rs, ok := s.stats.servers[server.Addr]
			if !ok || rs == nil || rs.Error != nil || rs.Timeout {
				continue
			}
			if i != 0 {
				if rs.MasterAddr() != v.Servers[0].Addr {
					// 如果当前Pika-Group内Master不为v.Servers[0].Addr，则认为主从关系出错
					bValid = false
					break
				}
				if rs.MasterLinkStatus() != MasterLinkStatusUp {
					// 如果当前Pika-Group内Slave与Master之间的状态不为：up，则此slave不对外提供服务
					continue
				}
			}

			doFunc := func(state dao.ServeState, port int) {
				sg, ok := bg.ServerGroup[state.String()]
				if !ok {
					sg = &dao.GSLBBackend{
						Port: port,
					}
					bg.ServerGroup[state.String()] = sg
				}
				sg.Servers = append(sg.Servers, server.Addr)
			}

			doFunc(dao.ServeStateRead, v.ProxyReadPort)
			if i == 0 {
				doFunc(dao.ServerStateWrite, v.ProxyWritePort)
			}
		}

		if bValid {
			backends = append(backends, bg)
		}
	}

	return backends, nil, nil
}

func (s *service) lvsBackends(backendName string) (dao.GSLBBackendGroups, dao.GSLBMonitors, error) {
	gslbs, err := s.gslbMapper.Info()
	if err != nil {
		return nil, nil, err
	}

	g, ok := gslbs[backendName]
	if !ok {
		log.Warnln("service::lvsBackends not found backend type. backendName:", backendName)
		return nil, nil, nil
	}

	var (
		monitors = g.Servers
		backends dao.GSLBBackendGroups
	)

	for _, bs := range g.Backends {
		bg := &dao.GSLBBackendGroup{
			Name:        bs.Name,
			ServerGroup: make(dao.GSLBBackends),
		}

		for state, v := range bs.ServerGroup {
			sg, ok := bg.ServerGroup[state]
			if !ok {
				sg = &dao.GSLBBackend{
					Port: v.Port,
				}
				bg.ServerGroup[state] = sg
			}

			sPort := strconv.Itoa(v.Port)
			servers := make([]string, 0, len(g.Servers))
			for _, vv := range g.Servers {
				ip, _, err := net.SplitHostPort(vv)
				if err != nil {
					log.Errorln("service::lvsBackends SplitHostPort failed. err:", err)
					continue
				}

				servers = append(servers, net.JoinHostPort(ip, sPort))
			}

			sg.Servers = servers
		}

		backends = append(backends, bg)
	}

	return backends, monitors, nil
}
