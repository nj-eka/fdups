package registrator

import (
	. "github.com/nj-eka/fdups/filestat"
	"sync"
)

type MifsRegister interface {
	CheckIn(fs FileStat) map[Inode][]FileStat
	GetRegs(copy bool) Mifs
	GetSizesCounter() Encounter
}

func NewMifsRegister(initCap int) MifsRegister {
	return &mifsRegistry{regs: make(Mifs, initCap), sizesCounter: NewEncounter(initCap)}
}

type mifsRegistry struct {
	sync.RWMutex
	regs         Mifs
	sizesCounter Encounter
}

func (w *mifsRegistry) CheckIn(fileStat FileStat) map[Inode][]FileStat {
	w.Lock()
	defer w.Unlock()
	if _, ok := w.regs[fileStat.MetaKey()]; !ok {
		w.regs[fileStat.MetaKey()] = make(map[Inode][]FileStat)
	}
	w.regs[fileStat.MetaKey()][fileStat.Inode()] = append(w.regs[fileStat.MetaKey()][fileStat.Inode()], fileStat)
	w.sizesCounter.CheckIn(fileStat.Size())
	return w.regs[fileStat.MetaKey()]
}

func (w *mifsRegistry) GetRegs(copy bool) Mifs {
	if !copy {
		return w.regs
	}
	w.RLock()
	defer w.RUnlock()
	return w.regs.Copy()
}

func (w *mifsRegistry) GetSizesCounter() Encounter {
	return w.sizesCounter
}
