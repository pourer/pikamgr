package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/pourer/pikamgr/utils/log"

	"github.com/BurntSushi/toml"
)

const DefaultFEConfig = `
##################################################
#                                                #
#                  Pika-FE               		 #
#                                                #
##################################################

# Set Coordinator, only accept "zookeeper" & "etcd" & "filesystem".
# for zookeeper/etcd, coorinator_auth accept "user:password" 
# Quick Start
#coordinator_name = "filesystem"
#coordinator_addr = "/tmp/dashboard-list.json"
coordinator_name = "zookeeper"
coordinator_addr = "127.0.0.1:2181"
coordinator_auth = ""

# Set bind address for visitor, tcp only.
listen_addr = "0.0.0.0:8080"

# Set configs for assets-files
assets_dir = ""

# Set configs for log
log_print_screen = false
log_file_path = ""
log_max_size = 100
log_reserve_days = 7
log_level = "info"
`

type FEConfig struct {
	CoordinatorName string `toml:"coordinator_name" json:"coordinator_name"`
	CoordinatorAddr string `toml:"coordinator_addr" json:"coordinator_addr"`
	CoordinatorAuth string `toml:"coordinator_auth" json:"coordinator_auth"`

	ListenAddr string `toml:"listen_addr" json:"listen_addr"`

	AssetsDir string `toml:"assets_dir" json:"assets_dir"`

	LogPrintScreen bool   `toml:"log_print_screen" json:"log_print_screen"`
	LogFilePath    string `toml:"log_file_path" json:"log_file_path"`
	LogMaxSize     int    `toml:"log_max_size" json:"log_max_size"`
	LogReserveDays int    `toml:"log_reserve_days" json:"log_reserve_days"`
	LogLevel       string `toml:"log_level" json:"log_level"`
}

func NewFEDefaultConfig() *FEConfig {
	c := &FEConfig{}
	if _, err := toml.Decode(DefaultFEConfig, c); err != nil {
		log.Panicln("NewFEDefaultConfig decode toml failed. err:", err)
	}

	if c.AssetsDir == "" {
		binpath, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Panicln("NewFEDefaultConfig get path of binary failed. err:", err)
		}
		c.AssetsDir = filepath.Join(binpath, "assets")
	}

	if err := c.Validate(); err != nil {
		log.Panicln("NewFEDefaultConfig validate config failed. err:", err)
	}
	return c
}

func (c *FEConfig) LoadFromFile(path string) error {
	_, err := toml.DecodeFile(path, c)
	if err != nil {
		return err
	}
	return c.Validate()
}

func (c *FEConfig) String() string {
	var b bytes.Buffer
	e := toml.NewEncoder(&b)
	e.Indent = "    "
	e.Encode(c)
	return b.String()
}

func (c *FEConfig) Validate() error {
	if c.CoordinatorName == "" {
		return errors.New("invalid coordinator_name")
	}
	if c.CoordinatorAddr == "" {
		return errors.New("invalid coordinator_addr")
	}
	if c.ListenAddr == "" {
		return errors.New("invalid listen_addr")
	}
	if c.AssetsDir == "" {
		return errors.New("invalid assets_dir")
	}
	return nil
}
