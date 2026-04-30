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
	OrigID uint32
}

type Cluster struct {
	Centroid [14]float32
	BboxMin  [14]float32
	BboxMax  [14]float32
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

	k := binary.LittleEndian.Uint32(bin[0:])
	n := binary.LittleEndian.Uint32(bin[4:])
	totalRegs = int(n)

	// Centroides
	centroids := make([][14]float32, k)
	for i := uint32(0); i < k; i++ {
		off := 8 + int(i)*56
		for j := 0; j < 14; j++ {
			centroids[i][j] = math.Float32frombits(binary.LittleEndian.Uint32(bin[off+j*4:]))
		}
	}

	// BboxMin
	bboxMin := make([][14]float32, k)
	offset := 8 + int(k)*56
	for i := uint32(0); i < k; i++ {
		off := offset + int(i)*56
		for j := 0; j < 14; j++ {
			bboxMin[i][j] = math.Float32frombits(binary.LittleEndian.Uint32(bin[off+j*4:]))
		}
	}

	// BboxMax
	bboxMax := make([][14]float32, k)
	offset += int(k) * 56
	for i := uint32(0); i < k; i++ {
		off := offset + int(i)*56
		for j := 0; j < 14; j++ {
			bboxMax[i][j] = math.Float32frombits(binary.LittleEndian.Uint32(bin[off+j*4:]))
		}
	}

	// Sizes
	sizes := make([]uint32, k)
	offset += int(k) * 56
	for i := uint32(0); i < k; i++ {
		sizes[i] = binary.LittleEndian.Uint32(bin[offset+int(i)*4:])
	}

	// Registros
	dataOffset := offset + int(k)*4
	dataPtr := unsafe.Pointer(&bin[dataOffset])
	allData := unsafe.Slice((*Registry)(dataPtr), int(n))

	// Warm-up
	for i := range allData {
		_ = allData[i].Label
	}

	clusters = make([]Cluster, k)
	idx := 0
	for i := 0; i < int(k); i++ {
		size := int(sizes[i])
		clusters[i] = Cluster{
			Centroid: centroids[i],
			BboxMin:  bboxMin[i],
			BboxMax:  bboxMax[i],
			Data:     allData[idx : idx+size],
		}
		idx += size
	}
}
