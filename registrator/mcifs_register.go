package registrator

import (
	. "github.com/nj-eka/fdups/filestat"
	"sync"
)

type McifsRegister interface {
	CheckIn(fs FileStat, contentKey string) map[Inode][]FileStat
	GetRegs(copy bool) Mcifs
	GetKeysCounter() Encounter
}

func NewMcifsRegister(initCap int) McifsRegister {
	return &mcifsRegistry{regs: make(Mcifs, initCap), keysCounter: NewEncounter(initCap)}
}

type mcifsRegistry struct {
	sync.RWMutex
	regs        Mcifs
	keysCounter Encounter
}

func (r *mcifsRegistry) CheckIn(fileStat FileStat, contentKey string) map[Inode][]FileStat {
	r.Lock()
	defer r.Unlock()
	mcKey := MCKey{fileStat.MetaKey(), contentKey}
	if _, ok := r.regs[mcKey]; !ok {
		r.regs[mcKey] = make(map[Inode][]FileStat)
	}
	r.regs[mcKey][fileStat.Inode()] = append(r.regs[mcKey][fileStat.Inode()], fileStat)
	r.keysCounter.CheckIn(KeySize{Key: mcKey, Size: fileStat.Size()})
	return r.regs[mcKey]
}

func (r *mcifsRegistry) GetRegs(copy bool) Mcifs {
	if !copy {
		return r.regs
	}
	r.RLock()
	defer r.RUnlock()
	return r.regs.Copy()
}

func (r *mcifsRegistry) GetKeysCounter() Encounter {
	return r.keysCounter
}
