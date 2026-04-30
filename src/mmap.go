package src

import (
	"encoding/binary"
	"math"
	"os"
	"syscall"
	"unsafe"
)

type Registry struct {
	Vector [14]float32
	Label  uint32
}

type Cluster struct {
	Centroid [14]float32
	Data     []Registry
}

var (
	clusters  []Cluster
	totalRegs int
)

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
	syscall.Madvise(bin, syscall.MADV_WILLNEED)

	// Le o header: k (clusters), n (registros)
	k := binary.LittleEndian.Uint32(bin[0:])
	n := binary.LittleEndian.Uint32(bin[4:])
	totalRegs = int(n)

	// Le os k centroides
	centroids := make([][14]float32, k)
	for i := uint32(0); i < k; i++ {
		off := 8 + int(i)*56
		for j := 0; j < 14; j++ {
			centroids[i][j] = math.Float32frombits(binary.LittleEndian.Uint32(bin[off+j*4:]))
		}
	}

	// Le os k tamanhos
	sizes := make([]uint32, k)
	for i := uint32(0); i < k; i++ {
		off := 8 + int(k)*56 + int(i)*4
		sizes[i] = binary.LittleEndian.Uint32(bin[off:])
	}

	// Os registros comecam apos os tamanhos
	dataOffset := 8 + int(k)*56 + int(k)*4
	dataPtr := unsafe.Pointer(&bin[dataOffset])

	// Cria slice de Registry apontando diretamente para o mmap.
	// Registry tem alignment 4; dataOffset eh garantido multiplo de 4
	// porque 56 e 4 sao multiplos de 4.
	allData := unsafe.Slice((*Registry)(dataPtr), int(n))

	// Cria os clusters
	clusters = make([]Cluster, k)
	idx := 0
	for i := 0; i < int(k); i++ {
		size := int(sizes[i])
		clusters[i] = Cluster{
			Centroid: centroids[i],
			Data:     allData[idx : idx+size],
		}
		idx += size
	}

	// Warm-up: toca todos os registros para forcar page faults no startup.
	for _, c := range clusters {
		for i := range len(c.Data) {
			_ = c.Data[i].Label
		}
	}
}
