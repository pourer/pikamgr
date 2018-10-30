package mapper

import (
	"fmt"
	"sync"

	"github.com/pourer/pikamgr/coordinate"
	"github.com/pourer/pikamgr/topom/dao"
	"github.com/pourer/pikamgr/utils/log"
)

type groupMapper struct {
	product string
	client  Client
	mutex   *sync.Mutex
	groups  dao.Groups
}

func NewGroupMapper(product string, client Client) (*groupMapper, error) {
	g := &groupMapper{
		product: product,
		client:  client,
		mutex:   new(sync.Mutex),
		groups:  make(dao.Groups),
	}
	if err := g.init(); err != nil {
		return nil, err
	}

	return g, nil
}

func (m *groupMapper) init() error {
	paths, err := m.client.List(coordinate.GroupDir(m.product), false)
	if err != nil {
		return err
	}

	groups := make(dao.Groups)
	for _, path := range paths {
		data, err := m.client.Read(path, true)
		if err != nil {
			return err
		}

		g := &dao.Group{}
		if err := g.Decode(data); err != nil {
			return err
		}

		groups[g.Name] = g
	}

	m.mutex.Lock()
	m.groups = groups
	m.mutex.Unlock()

	return nil
}

func (m *groupMapper) Create(g *dao.Group) error {
	data := g.Encode()
	log.Infof("groupMapper::CreateGroup group-[%s]:\n%s\n", g.Name, string(data))

	if err := m.client.Update(coordinate.GroupPath(m.product, g.Name), data); err != nil {
		log.Errorln("groupMapper::CreateGroup update fail. err:", err)
		return fmt.Errorf("groupMapper::CreateGroup update fail. group-[%s] err-[%s]", g.Name, err.Error())
	}

	m.mutex.Lock()
	m.groups[g.Name] = g
	m.mutex.Unlock()

	return nil
}

func (m *groupMapper) Update(g *dao.Group) error {
	data := g.Encode()
	log.Infof("groupMapper::UpdateGroup group-[%s]:\n%s\n", g.Name, string(data))

	if err := m.client.Update(coordinate.GroupPath(m.product, g.Name), data); err != nil {
		log.Errorln("groupMapper::UpdateGroup update fail. err:", err)
		return fmt.Errorf("groupMapper::UpdateGroup update fail. group-[%s] err-[%s]", g.Name, err.Error())
	}

	m.mutex.Lock()
	m.groups[g.Name] = g
	m.mutex.Unlock()

	return nil
}

func (m *groupMapper) Remove(g *dao.Group) error {
	data := g.Encode()
	log.Infof("groupMapper::RemoveGroup group-[%s]:\n%s\n", g.Name, string(data))

	if err := m.client.Delete(coordinate.GroupPath(m.product, g.Name)); err != nil {
		log.Errorln("groupMapper::RemoveGroup delete fail. err:", err)
		return fmt.Errorf("groupMapper::RemoveGroup delete fail. group-[%s] err-[%s]", g.Name, err.Error())
	}

	m.mutex.Lock()
	delete(m.groups, g.Name)
	m.mutex.Unlock()

	return nil
}

func (m *groupMapper) Info() (dao.Groups, error) {
	m.mutex.Lock()
	groups := m.groups
	m.mutex.Unlock()
	return groups, nil
}
