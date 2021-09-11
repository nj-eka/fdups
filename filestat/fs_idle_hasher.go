package filestat

type idleHasher struct{}

func (h idleHasher) Write([]byte) (n int, err error) {
	return 0, nil
}

func (h idleHasher) Sum([]byte) []byte {
	return []byte(EMPTY_CHECKSUM)
}

func (h idleHasher) Reset() {
}

func (h idleHasher) Size() int {
	return 0
}

func (h idleHasher) BlockSize() int {
	return 0
}
