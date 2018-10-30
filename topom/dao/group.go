package dao

const MAXGroupNameBytesLength = 32

type GroupServer struct {
	Addr string `json:"addr"`
}

func (g *GroupServer) Encode() []byte {
	return jsonEncode("group-server", g)
}

func (g *GroupServer) Decode(data []byte) error {
	return jsonDecode("group-server", g, data)
}

const (
	ActionNothing   = ""
	ActionPreparing = "preparing"
	ActionPrepared  = "prepared"
	ActionFinished  = "finished"
)

type Group struct {
	Name      string         `json:"name"`
	Servers   []*GroupServer `json:"servers"`
	Promoting struct {
		Index int    `json:"index,omitempty"`
		State string `json:"state,omitempty"`
	} `json:"promoting"`
	OutOfSync      bool   `json:"outOfSync"`
	ProxyReadPort  int    `json:"proxyReadPort"`
	ProxyWritePort int    `json:"proxyWritePort"`
	CreateTime     string `json:"createTime"`
}

func (g Group) GetMaster() string {
	if len(g.Servers) == 0 {
		return ""
	}
	return g.Servers[0].Addr
}

func (g Group) GetServerIndex(addr string) int {
	for i, v := range g.Servers {
		if v.Addr == addr {
			return i
		}
	}
	return -1
}

func (g *Group) Encode() []byte {
	return jsonEncode("group", g)
}

func (g *Group) Decode(data []byte) error {
	return jsonDecode("group", g, data)
}

type Groups map[string]*Group

func (g Groups) GetMasters() map[string]string {
	masters := make(map[string]string)
	for k, v := range g {
		if m := v.GetMaster(); len(m) > 0 {
			masters[k] = m
		}
	}
	return masters
}

func (g *Groups) Encode() []byte {
	return jsonEncode("groups", g)
}

func (g *Groups) Decode(data []byte) error {
	return jsonDecode("groups", g, data)
}
