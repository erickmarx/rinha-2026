package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"
)

const fixScale = 10000.0

type RegistroJSON struct {
	Vector []float32 `json:"vector"`
	Label  string    `json:"label"`
}

type Point struct {
	Vector [14]float32
	Label  uint32
	OrigID uint32
}

func clampFloat32(v float32) float32 {
	if v < -1.0 {
		return -1.0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}

func nearestCentroid(p Point, centroids [][14]float64) int {
	best := 0
	bestDist := math.MaxFloat64
	for c := range centroids {
		var d float64
		for j := 0; j < 14; j++ {
			diff := float64(p.Vector[j]) - centroids[c][j]
			d += diff * diff
		}
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	return best
}

func trainKMeans(points []Point, k int, sampleN int, iters int) [][14]float64 {
	n := len(points)
	if sampleN > n {
		sampleN = n
	}

	// Amostra regular (igual ao C)
	sample := make([]int, sampleN)
	for i := 0; i < sampleN; i++ {
		sample[i] = int((uint64(i) * uint64(n)) / uint64(sampleN))
	}

	// Inicializa centroides a partir da amostra regular
	centroids := make([][14]float64, k)
	for c := 0; c < k; c++ {
		si := int((uint64(c) * uint64(sampleN)) / uint64(k))
		if si >= sampleN {
			si = sampleN - 1
		}
		idx := sample[si]
		for j := 0; j < 14; j++ {
			centroids[c][j] = float64(points[idx].Vector[j])
		}
	}

	sums := make([][14]float64, k)
	counts := make([]int, k)

	for it := 0; it < iters; it++ {
		for c := 0; c < k; c++ {
			counts[c] = 0
			for j := 0; j < 14; j++ {
				sums[c][j] = 0
			}
		}

		for _, idx := range sample {
			p := points[idx]
			c := nearestCentroid(p, centroids)
			counts[c]++
			for j := 0; j < 14; j++ {
				sums[c][j] += float64(p.Vector[j])
			}
		}

		empty := 0
		for c := 0; c < k; c++ {
			if counts[c] == 0 {
				empty++
				continue
			}
			inv := 1.0 / float64(counts[c])
			for j := 0; j < 14; j++ {
				centroids[c][j] = sums[c][j] * inv
			}
		}
		fmt.Printf("[K-Means] Iter %d/%d sample=%d empty=%d\n", it+1, iters, sampleN, empty)
	}

	return centroids
}

func quantize(v float32) int16 {
	x := float64(clampFloat32(v)) * fixScale
	if x >= 0 {
		x += 0.5
	} else {
		x -= 0.5
	}
	if x > 10000.0 {
		x = 10000.0
	}
	if x < -10000.0 {
		x = -10000.0
	}
	return int16(x)
}

func main() {
	bin()
}

func bin() {
	content, err := os.ReadFile("references.json")
	if err != nil {
		fmt.Println("Erro ao ler JSON:", err)
		return
	}

	var registrosJSON []RegistroJSON
	json.Unmarshal(content, &registrosJSON)
	fmt.Printf("Carregados %d registros do JSON\n", len(registrosJSON))

	points := make([]Point, len(registrosJSON))
	for i, r := range registrosJSON {
		for j := 0; j < 14; j++ {
			points[i].Vector[j] = r.Vector[j]
		}
		if r.Label == "legit" {
			points[i].Label = 1
		} else {
			points[i].Label = 0
		}
		points[i].OrigID = uint32(i)
	}
	fmt.Printf("Convertidos %d registros para formato interno\n", len(points))

	const k = 256
	const sampleN = 131072
	const iters = 10

	fmt.Printf("Iniciando K-Means com k=%d sample=%d iters=%d...\n", k, sampleN, iters)
	start := time.Now()

	centroids := trainKMeans(points, k, sampleN, iters)

	fmt.Println("[K-Means] Atribuindo todos os pontos aos clusters...")
	assignments := make([]int, len(points))
	for i, p := range points {
		assignments[i] = nearestCentroid(p, centroids)
	}

	fmt.Println("[K-Means] Reorganizando registros por cluster...")
	sizes := make([]int, k)
	for _, c := range assignments {
		sizes[c]++
	}

	clusters := make([][]Point, k)
	for i := 0; i < k; i++ {
		clusters[i] = make([]Point, 0, sizes[i])
	}
	for i, p := range points {
		c := assignments[i]
		clusters[c] = append(clusters[c], p)
	}

	var allPoints []Point
	for i := 0; i < k; i++ {
		allPoints = append(allPoints, clusters[i]...)
	}

	n := len(points)

	// Quantizar vetores e calcular bounding boxes em int16
	bboxMin := make([][14]int16, k)
	bboxMax := make([][14]int16, k)
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			bboxMin[i][j] = 32767
			bboxMax[i][j] = -32768
		}
	}

	quantized := make([][14]int16, n)
	idx := 0
	for i := 0; i < k; i++ {
		for r := 0; r < sizes[i]; r++ {
			p := allPoints[idx]
			for j := 0; j < 14; j++ {
				qv := quantize(p.Vector[j])
				quantized[idx][j] = qv
				if qv < bboxMin[i][j] {
					bboxMin[i][j] = qv
				}
				if qv > bboxMax[i][j] {
					bboxMax[i][j] = qv
				}
			}
			idx++
		}
	}

	fmt.Printf("[K-Means] Concluido em %v\n", time.Since(start))

	fmt.Println("[K-Means] Escrevendo dataset.bin...")
	binFile, err := os.Create("dataset.bin")
	if err != nil {
		fmt.Println("Erro ao criar binario:", err)
		return
	}
	defer binFile.Close()

	// Header
	binary.Write(binFile, binary.LittleEndian, [4]byte{'I', 'V', 'F', 'G'})
	binary.Write(binFile, binary.LittleEndian, uint32(n))
	binary.Write(binFile, binary.LittleEndian, uint32(k))
	binary.Write(binFile, binary.LittleEndian, uint32(14))
	binary.Write(binFile, binary.LittleEndian, float32(fixScale))

	// Centroids
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, float32(centroids[i][j]))
		}
	}

	// BboxMin
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, bboxMin[i][j])
		}
	}

	// BboxMax
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, bboxMax[i][j])
		}
	}

	// Offsets
	offsets := make([]uint32, k+1)
	offsets[0] = 0
	for i := 0; i < k; i++ {
		offsets[i+1] = offsets[i] + uint32(sizes[i])
	}
	for i := 0; i <= k; i++ {
		binary.Write(binFile, binary.LittleEndian, offsets[i])
	}

	// Dims SoA (int16)
	for j := 0; j < 14; j++ {
		for i := 0; i < n; i++ {
			binary.Write(binFile, binary.LittleEndian, quantized[i][j])
		}
	}

	// OrigIDs
	for i := 0; i < n; i++ {
		binary.Write(binFile, binary.LittleEndian, allPoints[i].OrigID)
	}

	// Labels
	for i := 0; i < n; i++ {
		binary.Write(binFile, binary.LittleEndian, uint8(allPoints[i].Label))
	}

	binFile.Sync()
	fmt.Printf("[K-Means] dataset.bin gerado com sucesso! (k=%d, n=%d, scale=%.0f)\n", k, n, fixScale)
}
