package registrator

import (
	"fmt"
	. "github.com/nj-eka/fdups/filestat"
	"sort"
)

type MCKey struct {
	Mid, Cid string
}

func (dk MCKey) String() string {
	return fmt.Sprintf("mid{%s};cid{%s}", dk.Mid, dk.Cid)
}

type Mcifs map[MCKey]map[Inode][]FileStat

func (v Mcifs) Copy() Mcifs {
	c := make(Mcifs, len(v))
	for key, inodes := range v {
		c[key] = make(map[Inode][]FileStat, len(inodes))
		for inode, fss := range inodes {
			fssCopy := make([]FileStat, len(fss))
			copy(fssCopy, fss)
			c[key][inode] = fssCopy
		}
	}
	return c
}

func (v Mcifs) GetKeysSortedByMid() []MCKey {
	result := make([]MCKey, 0, len(v))
	for key := range v {
		result = append(result, key)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Mid > result[j].Mid
	})
	return result
}
