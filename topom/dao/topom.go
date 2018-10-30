package dao

type Topom struct {
	StartTime   string `json:"startTime"`
	AdminAddr   string `json:"adminAddr"`
	ProductName string `json:"productName"`
	Pid         int    `json:"pid"`
	Pwd         string `json:"pwd"`
	Sys         string `json:"sys"`
}

func (t *Topom) Encode() []byte {
	return jsonEncode("topom", t)
}

func (t *Topom) Decode(data []byte) error {
	return jsonDecode("topom", t, data)
}
