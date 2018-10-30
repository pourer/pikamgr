package topom

import (
	"sort"

	"github.com/pourer/pikamgr/topom/dao"
)

func sortGroups(groups dao.Groups) []*dao.Group {
	slice := make([]*dao.Group, 0, len(groups))
	for _, g := range groups {
		slice = append(slice, g)
	}
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].CreateTime > slice[j].CreateTime
	})
	return slice
}

func sortTemplateFiles(tfs dao.TemplateFiles) []string {
	files := make([]string, 0, len(tfs))
	for fileName := range tfs {
		files = append(files, fileName)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i] < files[j]
	})
	return files
}
