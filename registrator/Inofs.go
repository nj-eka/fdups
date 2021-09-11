package registrator

import (
	fs "github.com/nj-eka/fdups/filestat"
	"sort"
)

type Inofs map[fs.Inode][]fs.FileStat

func (m Inofs) Length() (result int) {
	for _, fss := range m {
		result += len(fss)
	}
	return
}

func (m Inofs) GetFileStatSorted() []fs.FileStat {
	ifss := make([]fs.FileStat, 0, m.Length())
	for _, fss := range m {
		ifss = append(ifss, fss...)
	}
	sort.SliceStable(ifss, func(i, j int) bool {
		return ifss[i].SortingKey() < ifss[j].SortingKey()
	})
	return ifss
}
