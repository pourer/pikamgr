package dao

import (
	"encoding/json"

	"github.com/pourer/pikamgr/utils/log"
)

func jsonEncode(flag string, v interface{}) []byte {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		log.Panicln("encode to json failed. flag:", flag, "err:", err)
	}
	return data
}

func jsonDecode(flag string, v interface{}, data []byte) error {
	if err := json.Unmarshal(data, v); err != nil {
		log.Errorln("decode from json failed. flag:", flag, "err:", err)
		return err
	}
	return nil
}
