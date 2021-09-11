package filtering

import (
	. "github.com/nj-eka/fdups/filestat"
	"github.com/nj-eka/fdups/registrator"
	"sync"
)

type ContentFilterStats struct {
	sync.RWMutex
	isCompleted     bool
	MetaRegister    registrator.MifsRegister
	StageRegisters  []registrator.McifsRegister
	StageInodeStats []registrator.InodeChecksums
	ContentRegister registrator.McifsRegister
	result          registrator.Mcifs
}

func (r *ContentFilterStats) IsCompleted() bool {
	r.RLock()
	defer r.RUnlock()
	return r.isCompleted
}

func (r *ContentFilterStats) setCompleted() {
	r.Lock()
	defer r.Unlock()
	r.isCompleted = true
	r.result = mergeRegisters(r, true)
}

func (r *ContentFilterStats) GetResult() (registrator.Mcifs, bool) {
	isCompleted := r.IsCompleted()
	if r.result != nil {
		return r.result, isCompleted
	}
	return mergeRegisters(r, isCompleted), isCompleted
}

func mergeRegisters(r *ContentFilterStats, isCompleted bool) registrator.Mcifs {
	mRegs := r.MetaRegister.GetRegs(!isCompleted)
	mcRegs := r.ContentRegister.GetRegs(!isCompleted)
	result := make(registrator.Mcifs, len(mcRegs))
	for mcKey, inodes := range mcRegs {
		result[mcKey] = make(map[Inode][]FileStat, len(inodes))
		for inode := range inodes {
			result[mcKey][inode] = mRegs[mcKey.Mid][inode]
		}
	}
	return result
}
