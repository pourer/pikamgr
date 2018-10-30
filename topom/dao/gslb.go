package dao

type ServeState int

const (
	ServeStateUnavailable ServeState = iota
	ServeStateRead
	ServerStateWrite
)

func (s ServeState) String() string {
	switch s {
	case ServeStateUnavailable:
		return "Unavailable"
	case ServeStateRead:
		return "Read"
	case ServerStateWrite:
		return "Write"
	default:
		return "Unknown"
	}
}

type GSLBBackend struct {
	Servers []string `json:"servers"`
	Port    int      `json:"port"`
}

type GSLBBackends map[string]*GSLBBackend

type GSLBBackendGroup struct {
	Name        string       `json:"name"`
	Servers     []string     `json:"servers,omitempty"`
	ServerGroup GSLBBackends `json:"serverGroup"`
}

type GSLBBackendGroups []*GSLBBackendGroup

type GSLBMonitors []string

type GSLB struct {
	Name     string            `json:"name"`
	Servers  []string          `json:"servers,omitempty"`
	Monitors GSLBMonitors      `json:"monitors,omitempty"`
	Backends GSLBBackendGroups `json:"backends,omitempty"`
}

func (g *GSLB) Encode() []byte {
	return jsonEncode("gslb", g)
}

func (g *GSLB) Decode(data []byte) error {
	return jsonDecode("gslb", g, data)
}

type GSLBs map[string]*GSLB

func (g *GSLBs) Encode() []byte {
	return jsonEncode("gslbs", g)
}

func (g *GSLBs) Decode(data []byte) error {
	return jsonDecode("gslbs", g, data)
}
