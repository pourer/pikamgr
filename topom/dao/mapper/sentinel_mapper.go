package mapper

import (
	"fmt"
	"sync"

	"github.com/pourer/pikamgr/coordinate"
	"github.com/pourer/pikamgr/topom/dao"
	"github.com/pourer/pikamgr/utils/log"
)

type sentinelMapper struct {
	product  string
	client   Client
	mutex    *sync.Mutex
	sentinel *dao.Sentinel
}

func NewSentinelMapper(product string, client Client) (*sentinelMapper, error) {
	s := &sentinelMapper{
		product:  product,
		client:   client,
		mutex:    new(sync.Mutex),
		sentinel: &dao.Sentinel{},
	}
	if err := s.init(); err != nil {
		return nil, err
	}

	return s, nil
}

func (m *sentinelMapper) init() error {
	data, err := m.client.Read(coordinate.SentinelPath(m.product), false)
	if err != nil {
		return err
	}

	if data != nil {
		sentinel := &dao.Sentinel{}
		if err := sentinel.Decode(data); err != nil {
			return err
		}

		m.mutex.Lock()
		m.sentinel = sentinel
		m.mutex.Unlock()
	}

	return nil
}

func (m *sentinelMapper) Update(sentinel *dao.Sentinel) error {
	data := sentinel.Encode()
	log.Infof("sentinelMapper::UpdateSentinel \n%s\n", string(data))

	if err := m.client.Update(coordinate.SentinelPath(m.product), data); err != nil {
		log.Errorln("sentinelMapper::UpdateSentinel update fail. err:", err)
		return fmt.Errorf("sentinelMapper::UpdateSentinel update fail. err-[%s]", err.Error())
	}

	m.mutex.Lock()
	m.sentinel = sentinel
	m.mutex.Unlock()

	return nil
}

func (m *sentinelMapper) Info() (*dao.Sentinel, error) {
	m.mutex.Lock()
	sentinel := m.sentinel
	m.mutex.Unlock()
	return sentinel, nil
}
