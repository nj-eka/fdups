package registrator

import (
	. "github.com/nj-eka/fdups/filestat"
)

type Mifs map[string]map[Inode][]FileStat

func (r *Mifs) Copy() Mifs {
	c := make(Mifs, len(*r))
	for key, inodes := range *r {
		c[key] = make(map[Inode][]FileStat, len(inodes))
		for inode, fss := range inodes {
			fssCopy := make([]FileStat, len(fss))
			copy(fssCopy, fss)
			c[key][inode] = fssCopy
		}
	}
	return c
}

func (r *Mifs) GetGroupsTotal() int {
	return len(*r)
}

func (r *Mifs) GetDupGroupsCount() (count int) {
	for _, inodes := range *r {
		// if to count dup groups only by inodes
		//if len(inodes) > 1{
		//	count++
		//}
		// if to count dup groups by files (iow - inluding links)
		for _, fss := range inodes {
			if len(fss) > 0 {
				count++
			}
		}
	}
	return
}
