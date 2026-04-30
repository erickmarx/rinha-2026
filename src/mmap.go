package src

import (
	"encoding/binary"
	"math"
	"os"
	"syscall"
	"unsafe"
)

var (
	clusters     []Cluster
	totalRegs    int
	dimData      [14][]int16
	labelsData   []uint8
	origIDsData  []uint32
)

type Cluster struct {
	Centroid [14]float32
	BboxMin  [14]int16
	BboxMax  [14]int16
	Start    int
	End      int
}

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

	if string(bin[0:4]) != "IVFG" {
		panic("dataset.bin: magic invalido, esperado IVFG")
	}

	n := binary.LittleEndian.Uint32(bin[4:])
	k := binary.LittleEndian.Uint32(bin[8:])
	// d := binary.LittleEndian.Uint32(bin[12:])
	// scale := math.Float32frombits(binary.LittleEndian.Uint32(bin[16:]))
	totalRegs = int(n)

	offset := 20

	// Centroids
	centroids := make([][14]float32, k)
	for i := uint32(0); i < k; i++ {
		for j := 0; j < 14; j++ {
			centroids[i][j] = math.Float32frombits(binary.LittleEndian.Uint32(bin[offset:]))
			offset += 4
		}
	}

	// BboxMin
	bboxMin := make([][14]int16, k)
	for i := uint32(0); i < k; i++ {
		for j := 0; j < 14; j++ {
			bboxMin[i][j] = int16(binary.LittleEndian.Uint16(bin[offset:]))
			offset += 2
		}
	}

	// BboxMax
	bboxMax := make([][14]int16, k)
	for i := uint32(0); i < k; i++ {
		for j := 0; j < 14; j++ {
			bboxMax[i][j] = int16(binary.LittleEndian.Uint16(bin[offset:]))
			offset += 2
		}
	}

	// Offsets
	offsets := make([]uint32, k+1)
	for i := uint32(0); i <= k; i++ {
		offsets[i] = binary.LittleEndian.Uint32(bin[offset:])
		offset += 4
	}

	// Dim SoA int16
	for j := 0; j < 14; j++ {
		ptr := unsafe.Pointer(&bin[offset])
		dimData[j] = unsafe.Slice((*int16)(ptr), int(n))
		offset += int(n) * 2
	}

	// OrigIDs
	origPtr := unsafe.Pointer(&bin[offset])
	origIDsData = unsafe.Slice((*uint32)(origPtr), int(n))
	offset += int(n) * 4

	// Labels
	labelsPtr := unsafe.Pointer(&bin[offset])
	labelsData = unsafe.Slice((*uint8)(labelsPtr), int(n))
	offset += int(n)

	// Warm-up: touch all pages via labels (pequeno e linear)
	var sum uint8
	for i := range labelsData {
		sum += labelsData[i]
	}
	_ = sum

	// Build clusters
	clusters = make([]Cluster, k)
	for i := 0; i < int(k); i++ {
		clusters[i] = Cluster{
			Centroid: centroids[i],
			BboxMin:  bboxMin[i],
			BboxMax:  bboxMax[i],
			Start:    int(offsets[i]),
			End:      int(offsets[i+1]),
		}
	}
}
