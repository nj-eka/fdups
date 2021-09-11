package filestat

import (
	"fmt"
	"strings"
)

type PriorFunc func(path string) string

func NewPriorFunc(roots []string) PriorFunc {
	return func(path string) string {
		prior := len(roots) - 1
		for p := prior; p >= 0; p-- {
			if strings.HasPrefix(path, roots[p]) {
				prior = p
				break
			}
		}
		return fmt.Sprintf("%2d", prior)
	}
}
