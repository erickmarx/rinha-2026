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

// Cluster representa um grupo de registros similares encontrados pelo k-means.
// Em vez de fazer scan linear em TODO o dataset (1M registros),
// a API scaneia apenas nos registros do cluster mais proximo (~1k registros).
type Cluster struct {
	Centroid [14]float32 // centro geometrico do cluster
	Data     []Registry  // slice apontando para os registros deste cluster no mmap
}

var (
	clusters   []Cluster // todos os clusters (k = 1000)
	totalRegs  int       // numero total de registros (para referencia)
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
	// Diz ao kernel para manter as paginas em RAM.
	syscall.Madvise(bin, syscall.MADV_WILLNEED)

	// Le o header: numero de clusters (k) e numero total de registros (n).
	// O header ocupa os primeiros 8 bytes do arquivo.
	k := *(*uint32)(unsafe.Pointer(&bin[0]))
	n := *(*uint32)(unsafe.Pointer(&bin[4]))
	totalRegs = int(n)

	// Le os k centroides (k x 14 float32 = k x 56 bytes).
	// Comecam no byte 8.
	centroidsPtr := unsafe.Pointer(&bin[8])
	rawCentroids := (*[1 << 20][14]float32)(centroidsPtr)[:k:k]

	// Le os k tamanhos (k x uint32 = k x 4 bytes).
	// Comecam apos os centroides.
	sizesOffset := 8 + int(k)*56
	sizesPtr := unsafe.Pointer(&bin[sizesOffset])
	rawSizes := (*[1 << 20]uint32)(sizesPtr)[:k:k]

	// Os registros comecam apos os tamanhos.
	dataOffset := sizesOffset + int(k)*4
	dataPtr := unsafe.Pointer(&bin[dataOffset])
	allData := (*[1 << 30]Registry)(dataPtr)[:n:n]

	// Cria os slices de Cluster apontando para as regioes corretas de allData.
	clusters = make([]Cluster, k)
	idx := 0
	for i := 0; i < int(k); i++ {
		size := int(rawSizes[i])
		clusters[i] = Cluster{
			Centroid: rawCentroids[i],
			Data:     allData[idx : idx+size],
		}
		idx += size
	}

	// Warm-up: percorre todos os registros para forcar page faults no startup.
	for _, c := range clusters {
		for i := range len(c.Data) {
			_ = c.Data[i].Label
		}
	}
}
