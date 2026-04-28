package src

import (
	"os"
	"syscall"
	"unsafe"
)

type Registry struct {
	Vector [14]float32
	Label  uint32
}

var dataset []Registry

func Mmap(f *os.File) bool {
	info, _ := f.Stat()
	size := int(info.Size())

	bin, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ, syscall.MAP_SHARED)

	if err != nil {
		panic(err)
	}

	RegisterBinary(bin, size)

	return len(bin) > 0
}

func RegisterBinary(bin []byte, size int) {
	syscall.Madvise(bin, syscall.MADV_WILLNEED)
	totalRegistros := size / 60

	dataset = (*[1 << 30]Registry)(unsafe.Pointer(&bin[0]))[:totalRegistros:totalRegistros]
}
