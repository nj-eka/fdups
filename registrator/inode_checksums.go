package registrator

import (
	. "github.com/nj-eka/fdups/filestat"
	"sync"
)

const empty = ""

type InodeChecksums interface {
	CheckIn(fileStat FileStat) (string, bool, bool, <-chan struct{})
	Update(fileStat FileStat, c string, written int64)
	Delete(fileStat FileStat)
	GetStats() (inodesCount int, totalSize int64)
}

func NewInodeChecksums(initCap int) InodeChecksums {
	return &inodeChecksums{regs: make(map[Inode]string, initCap), pendings: make(map[Inode]chan struct{}, initCap), nongrata: make(map[Inode]bool, initCap)}
}

type inodeChecksums struct {
	sync.RWMutex
	regs        map[Inode]string
	pendings    map[Inode]chan struct{}
	nongrata    map[Inode]bool
	inodesCount int
	totalSizes  int64
}

func (iS *inodeChecksums) CheckIn(fileStat FileStat) (string, bool, bool, <-chan struct{}) {
	iS.Lock()
	defer iS.Unlock()
	if iS.nongrata[fileStat.Inode()] { // todo: make rlock
		return empty, true, false, nil
	}
	if value, ok := iS.regs[fileStat.Inode()]; ok { // rlock
		return value, true, true, nil
	}
	if pending, ok := iS.pendings[fileStat.Inode()]; ok { // lock
		return empty, false, false, pending
	}
	iS.pendings[fileStat.Inode()] = make(chan struct{}) // lock
	return empty, false, false, nil
}

func (iS *inodeChecksums) Update(fileStat FileStat, c string, written int64) {
	iS.Lock()
	defer iS.Unlock()
	iS.regs[fileStat.Inode()] = c // todo: check c != empty ...
	iS.inodesCount++
	iS.totalSizes += written
	if pending, ok := iS.pendings[fileStat.Inode()]; ok {
		close(pending)
		delete(iS.pendings, fileStat.Inode())
	}
}

func (iS *inodeChecksums) Delete(fileStat FileStat) {
	iS.Lock()
	defer iS.Unlock()
	iS.nongrata[fileStat.Inode()] = true
	delete(iS.regs, fileStat.Inode()) //
	if pending, ok := iS.pendings[fileStat.Inode()]; ok {
		close(pending)
		delete(iS.pendings, fileStat.Inode())
	}
	// to collect statistics of all processing the following lines are commented out
	//iS.inodesCount--
	//iS.totalSizes -= fileStat.Size()
}

func (iS *inodeChecksums) GetStats() (inodesCount int, totalSize int64) {
	iS.RLock()
	defer iS.RUnlock()
	return iS.inodesCount, iS.totalSizes
}
