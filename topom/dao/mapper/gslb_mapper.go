package mapper

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/pourer/pikamgr/coordinate"
	"github.com/pourer/pikamgr/topom/dao"
	"github.com/pourer/pikamgr/utils/log"
)

type gslbMapper struct {
	product string
	client  Client
	gm      *groupMapper
	mutex   *sync.Mutex
	gslbs   dao.GSLBs
}

func NewGSLBMapper(product string, client Client, gm *groupMapper) (*gslbMapper, error) {
	g := &gslbMapper{
		product: product,
		client:  client,
		gm:      gm,
		mutex:   new(sync.Mutex),
		gslbs:   make(dao.GSLBs),
	}
	if err := g.init(); err != nil {
		return nil, err
	}

	return g, nil
}

func (m *gslbMapper) init() error {
	paths, err := m.client.List(coordinate.GSLBDir(), false)
	if err != nil {
		return err
	}

	gslbs := make(dao.GSLBs)
	for _, path := range paths {
		productPaths, err := m.client.List(path, false)
		if err != nil {
			return err
		}

		for _, pPath := range productPaths {
			if filepath.Base(pPath) != m.product {
				continue
			}

			data, err := m.client.Read(pPath, true)
			if err != nil {
				return err
			}

			g := &dao.GSLB{}
			if err := g.Decode(data); err != nil {
				return err
			}
			if len(g.Servers) == 0 {
				continue
			}

			gslbs[g.Name] = g
		}
	}

	m.mutex.Lock()
	m.gslbs = gslbs
	m.mutex.Unlock()

	return nil
}

func (m *gslbMapper) Update(g *dao.GSLB) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if old, ok := m.gslbs[g.Name]; ok && reflect.DeepEqual(old, g) {
		log.Tracef("gslbMapper::Update gslbName-[%s]. old and new is equal.")
		return nil
	}

	data := g.Encode()
	log.Infof("gslbMapper::Update gslbName-[%s]\n%s\n", g.Name, string(data))
	if err := m.client.Update(coordinate.GSLBPath(g.Name, m.product), data); err != nil {
		log.Errorln("gslbMapper::Update update fail. err:", err)
		return fmt.Errorf("gslbMapper::Update update fail. gslbName-[%s] err-[%s]", g.Name, err.Error())
	}

	m.gslbs[g.Name] = g
	return nil
}

func (m *gslbMapper) Delete(g *dao.GSLB) error {
	data := g.Encode()
	log.Infof("gslbMapper::Delete gslbName-[%s]\n%s\n", g.Name, string(data))

	if err := m.client.Delete(coordinate.GSLBPath(g.Name, m.product)); err != nil {
		log.Errorln("gslbMapper::Delete delete fail. err:", err)
		return fmt.Errorf("gslbMapper::Delete delete fail. err-[%s]", err.Error())
	}

	m.mutex.Lock()
	delete(m.gslbs, g.Name)
	m.mutex.Unlock()

	return nil
}

func (m *gslbMapper) Info() (dao.GSLBs, error) {
	m.mutex.Lock()
	gslbs := m.gslbs
	m.mutex.Unlock()
	return gslbs, nil
}
