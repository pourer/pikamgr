package protocol

type SentinelGroup struct {
	Master map[string]string   `json:"master"`
	Slaves []map[string]string `json:"slaves,omitempty"`
}

type RedisStats struct {
	Error    error                     `json:"error"`
	Stats    map[string]string         `json:"stats,omitempty"`
	Sentinel map[string]*SentinelGroup `json:"sentinel,omitempty"`
	UnixTime int64                     `json:"unixtime"`
	Timeout  bool                      `json:"timeout,omitempty"`
}

type GroupServer struct {
	Addr         string `json:"server"`
	ReplicaGroup bool   `json:"replicaGroup"`
}

type Group struct {
	Name      string         `json:"name"`
	Servers   []*GroupServer `json:"servers"`
	Promoting struct {
		Index int    `json:"index,omitempty"`
		State string `json:"state,omitempty"`
	} `json:"promoting"`
	OutOfSync      bool `json:"outOfSync"`
	ProxyReadPort  int  `json:"proxyReadPort"`
	ProxyWritePort int  `json:"proxyWritePort"`
}

type Sentinel struct {
	Servers   []string `json:"servers,omitempty"`
	OutOfSync bool     `json:"outOfSync"`
}

type GSLBStats struct {
	Error    error `json:"error"`
	UnixTime int64 `json:"unixtime"`
	Timeout  bool  `json:"timeout,omitempty"`
}

type GSLB struct {
	Servers []string `json:"servers,omitempty"`
}

type Stats struct {
	Closed bool `json:"closed"`
	Group  struct {
		Models []*Group               `json:"models"`
		Stats  map[string]*RedisStats `json:"stats"`
	} `json:"group"`
	HA struct {
		Model   *Sentinel              `json:"model"`
		Stats   map[string]*RedisStats `json:"stats"`
		Masters map[string]string      `json:"masters"`
	} `json:"sentinels"`
	GSLB struct {
		Models map[string]*GSLB      `json:"models"`
		Stats  map[string]*GSLBStats `json:"stats"`
	} `json:"gslbs"`
	Template struct {
		FileNames []string `json:"fileNames"`
	} `json:"template"`
}

type Topom struct {
	StartTime   string `json:"startTime"`
	AdminAddr   string `json:"adminAddr"`
	ProductName string `json:"productName"`
	Pid         int    `json:"pid"`
	Pwd         string `json:"pwd"`
	Sys         string `json:"sys"`
}

type Overview struct {
	Version string      `json:"version"`
	Compile string      `json:"compile"`
	Config  interface{} `json:"config"`
	Model   *Topom      `json:"model,omitempty"`
	Stats   *Stats      `json:"stats,omitempty"`
}
