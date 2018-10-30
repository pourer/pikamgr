package dao

type TemplateFile struct {
	Data, MD5 []byte
}

type TemplateFiles map[string]*TemplateFile

func (t *TemplateFiles) Encode() []byte {
	return jsonEncode("template-files", t)
}

func (t *TemplateFiles) Decode(data []byte) error {
	return jsonDecode("template-files", t, data)
}
