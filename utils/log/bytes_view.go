package log

type BytesView []byte

func (b BytesView) String() string {
	return string(b)
}
