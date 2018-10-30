package config

import (
	"bytes"
	"errors"
	"regexp"

	"github.com/pourer/pikamgr/utils/log"

	"github.com/BurntSushi/toml"
	"github.com/CodisLabs/codis/pkg/utils/timesize"
)

const DefaultDashboardConfig = `
##################################################
#                                                #
#                  Pika-Dashboard                #
#                                                #
##################################################

# Set Coordinator, only accept "zookeeper" & "etcd".
# for zookeeper/etcd, coorinator_auth accept "user:password" 
# Quick Start
coordinator_name = "zookeeper"
coordinator_addr = "127.0.0.1:2181"
coordinator_auth = ""

# Set Codis Product Name/Auth.
product_name = "codis-demo"
product_auth = ""

# Set bind address for admin(rpc), tcp only.
admin_addr = "0.0.0.0:18080"

# Set configs for redis sentinel.
sentinel_client_timeout = "10s"
sentinel_quorum = 2
sentinel_parallel_syncs = 1
sentinel_down_after = "30s"
sentinel_failover_timeout = "5m"
sentinel_notification_script = ""
sentinel_client_reconfig_script = ""

# Set configs for template-file
template_file_scan_dir = "/tmp/template"
template_file_scan_interval = "30s"

# Set configs for log
log_print_screen = false
log_file_path = ""
log_max_size = 100
log_reserve_days = 7
log_level = "info"
`

type DashboardConfig struct {
	CoordinatorName string `toml:"coordinator_name" json:"coordinator_name"`
	CoordinatorAddr string `toml:"coordinator_addr" json:"coordinator_addr"`
	CoordinatorAuth string `toml:"coordinator_auth" json:"coordinator_auth"`

	AdminAddr   string `toml:"admin_addr" json:"admin_addr"`
	ProductName string `toml:"product_name" json:"product_name"`
	ProductAuth string `toml:"product_auth" json:"-"`

	SentinelClientTimeout        timesize.Duration `toml:"sentinel_client_timeout" json:"sentinel_client_timeout"`
	SentinelQuorum               int               `toml:"sentinel_quorum" json:"sentinel_quorum"`
	SentinelParallelSyncs        int               `toml:"sentinel_parallel_syncs" json:"sentinel_parallel_syncs"`
	SentinelDownAfter            timesize.Duration `toml:"sentinel_down_after" json:"sentinel_down_after"`
	SentinelFailoverTimeout      timesize.Duration `toml:"sentinel_failover_timeout" json:"sentinel_failover_timeout"`
	SentinelNotificationScript   string            `toml:"sentinel_notification_script" json:"sentinel_notification_script"`
	SentinelClientReconfigScript string            `toml:"sentinel_client_reconfig_script" json:"sentinel_client_reconfig_script"`

	TemplateFileScanDir      string            `toml:"template_file_scan_dir" json:"template_file_scan_dir"`
	TemplateFileScanInterval timesize.Duration `toml:"template_file_scan_interval" json:"template_file_scan_interval"`

	LogPrintScreen bool   `toml:"log_print_screen" json:"log_print_screen"`
	LogFilePath    string `toml:"log_file_path" json:"log_file_path"`
	LogMaxSize     int    `toml:"log_max_size" json:"log_max_size"`
	LogReserveDays int    `toml:"log_reserve_days" json:"log_reserve_days"`
	LogLevel       string `toml:"log_level" json:"log_level"`
}

func NewDashboardDefaultConfig() *DashboardConfig {
	c := &DashboardConfig{}
	if _, err := toml.Decode(DefaultDashboardConfig, c); err != nil {
		log.Panicln("NewDashboardDefaultConfig decode toml failed. err:", err)
	}
	if err := c.Validate(); err != nil {
		log.Panicln("NewDashboardDefaultConfig validate config failed. err:", err)
	}
	return c
}

func (c *DashboardConfig) LoadFromFile(path string) error {
	_, err := toml.DecodeFile(path, c)
	if err != nil {
		return err
	}
	return c.Validate()
}

func (c *DashboardConfig) String() string {
	var b bytes.Buffer
	e := toml.NewEncoder(&b)
	e.Indent = "    "
	e.Encode(c)
	return b.String()
}

func (c *DashboardConfig) Validate() error {
	if c.CoordinatorName == "" {
		return errors.New("invalid coordinator_name")
	}
	if c.CoordinatorAddr == "" {
		return errors.New("invalid coordinator_addr")
	}
	if c.AdminAddr == "" {
		return errors.New("invalid admin_addr")
	}
	if c.ProductName == "" || !validateProduct(c.ProductName) {
		return errors.New("invalid product_name")
	}
	if c.SentinelClientTimeout <= 0 {
		return errors.New("invalid sentinel_client_timeout")
	}
	if c.SentinelQuorum <= 0 {
		return errors.New("invalid sentinel_quorum")
	}
	if c.SentinelParallelSyncs <= 0 {
		return errors.New("invalid sentinel_parallel_syncs")
	}
	if c.SentinelDownAfter <= 0 {
		return errors.New("invalid sentinel_down_after")
	}
	if c.SentinelFailoverTimeout <= 0 {
		return errors.New("invalid sentinel_failover_timeout")
	}
	if c.TemplateFileScanDir == "" {
		return errors.New("invalid template_file_scan_dir")
	}
	if c.TemplateFileScanInterval <= 0 {
		return errors.New("invalid template_file_scan_interval")
	}
	return nil
}

func validateProduct(name string) bool {
	if regexp.MustCompile(`^\w[\w\.\-]*$`).MatchString(name) {
		return true
	}
	return false
}
