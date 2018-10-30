package mapper

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pourer/pikamgr/coordinate"
	"github.com/pourer/pikamgr/topom/dao"
	"github.com/pourer/pikamgr/utils/log"
)

type topomMapper struct {
	product string
	client  Client
	mutex   *sync.Mutex
	topom   *dao.Topom
}

func NewTopomMapper(product, adminAddr string, client Client) (*topomMapper, error) {
	t := &topomMapper{
		product: product,
		client:  client,
		mutex:   new(sync.Mutex),
		topom:   &dao.Topom{},
	}
	if err := t.init(adminAddr); err != nil {
		return nil, err
	}

	return t, nil
}

func (m *topomMapper) init(adminAddr string) error {
	t := &dao.Topom{
		StartTime:   time.Now().String(),
		AdminAddr:   adminAddr,
		ProductName: m.product,
		Pid:         os.Getpid(),
	}

	t.Pwd, _ = os.Getwd()
	if b, err := exec.Command("uname", "-a").Output(); err != nil {
		log.Errorln("topomMapper::init run command uname failed. err:", err)
	} else {
		t.Sys = strings.TrimSpace(string(b))
	}

	m.mutex.Lock()
	m.topom = t
	m.mutex.Unlock()

	log.Infoln("topomMapper::init new topom:", string(t.Encode()))
	return nil
}

func (m *topomMapper) Create() error {
	data := m.topom.Encode()
	log.Infof("topomMapper::Create \n%s\n", string(data))

	if err := m.client.Create(coordinate.TopomPath(m.product), data); err != nil {
		log.Errorln("topomMapper::Create create fail. err:", err)
		return fmt.Errorf("topomMapper::Create create fail. err-[%s]", err.Error())
	}

	log.Infof("topomMapper::Create Suc. \n%s\n", string(data))
	return nil
}

func (m *topomMapper) Delete() error {
	data := m.topom.Encode()
	log.Infof("topomMapper::Delete \n%s\n", string(data))

	if err := m.client.Delete(coordinate.TopomPath(m.product)); err != nil {
		log.Errorln("topomMapper::Delete delete fail. err:", err)
		return fmt.Errorf("topomMapper::Delete delete fail. err-[%s]", err.Error())
	}

	log.Infof("topomMapper::Delete Suc. \n%s\n", string(data))
	return nil
}

func (m *topomMapper) Info() (*dao.Topom, error) {
	m.mutex.Lock()
	t := m.topom
	m.mutex.Unlock()
	return t, nil
}
