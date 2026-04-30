package src

import (
	"encoding/binary"
	"math"
	"os"
	"syscall"
)

// Registry representa um registro do dataset.
type Registry struct {
	Vector [14]float32
	Label  uint32
}

// dataset global — carregado do mmap e acessado pela VP-Tree.
var dataset []Registry

func Mmap(f *os.File) bool {
	info, _ := f.Stat()
	size := int(info.Size())

	bin, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		panic(err)
	}

	RegisterBinary(bin)
	return len(bin) > 0
}

func RegisterBinary(bin []byte) {
	// Diz ao kernel para manter as paginas em RAM.
	syscall.Madvise(bin, syscall.MADV_WILLNEED)

	// Header: numero de registros (uint32)
	n := binary.LittleEndian.Uint32(bin[0:])

	// Copia registros do mmap para memoria Go (evita page faults durante queries).
	// Cada registro: 14 float32 (56 bytes) + 1 uint32 (4 bytes) = 60 bytes.
	dataset = make([]Registry, n)
	for i := uint32(0); i < n; i++ {
		off := 4 + int(i)*60
		for j := 0; j < 14; j++ {
			dataset[i].Vector[j] = math.Float32frombits(binary.LittleEndian.Uint32(bin[off+j*4:]))
		}
		dataset[i].Label = binary.LittleEndian.Uint32(bin[off+56:])
	}

	// Warm-up: toca todos os registros para garantir que estao em RAM.
	for i := range dataset {
		_ = dataset[i].Label
	}

	// VP-Tree: le o numero de nos e os nos em si.
	nodeOffset := 4 + int(n)*60
	nodeCount := binary.LittleEndian.Uint32(bin[nodeOffset:])
	vpNodes = make([]VPNode, nodeCount)

	for i := uint32(0); i < nodeCount; i++ {
		off := nodeOffset + 4 + int(i)*16
		vpNodes[i].Index = int32(binary.LittleEndian.Uint32(bin[off:]))
		vpNodes[i].Threshold = math.Float32frombits(binary.LittleEndian.Uint32(bin[off+4:]))
		vpNodes[i].Inside = int32(binary.LittleEndian.Uint32(bin[off+8:]))
		vpNodes[i].Outside = int32(binary.LittleEndian.Uint32(bin[off+12:]))
	}
	vpRoot = 0

	// Warm-up da VP-Tree: toca todos os nos.
	for i := range vpNodes {
		_ = vpNodes[i].Index
	}
}
