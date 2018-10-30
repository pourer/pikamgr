package dao

type Sentinel struct {
	Servers   []string `json:"servers,omitempty"`
	OutOfSync bool     `json:"outOfSync"`
}

func (s *Sentinel) Encode() []byte {
	return jsonEncode("sentinel", s)
}

func (s *Sentinel) Decode(data []byte) error {
	return jsonDecode("sentinel", s, data)
}