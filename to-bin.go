package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"
)

type RegistroJSON struct {
	Vector []float32 `json:"vector"`
	Label  string    `json:"label"`
}

type Point struct {
	Vector [14]float32
	Label  uint32
	OrigID uint32
}

type KMeans struct {
	Points      []Point
	Centroids   [][14]float64
	Assignments []int
	K           int
}

func NewKMeans(points []Point, k int) *KMeans {
	km := &KMeans{
		Points:      points,
		Centroids:   make([][14]float64, k),
		Assignments: make([]int, len(points)),
		K:           k,
	}
	rand.Seed(42)
	for i := 0; i < k; i++ {
		idx := rand.Intn(len(points))
		for j := 0; j < 14; j++ {
			km.Centroids[i][j] = float64(points[idx].Vector[j])
		}
	}
	return km
}

func distanceSquared(p Point, c [14]float64) float64 {
	var sum float64
	for i := 0; i < 14; i++ {
		diff := float64(p.Vector[i]) - c[i]
		sum += diff * diff
	}
	return sum
}

func (km *KMeans) Run(maxIter int) {
	for iter := 0; iter < maxIter; iter++ {
		changed := 0
		iterStart := time.Now()

		for i, p := range km.Points {
			bestCluster := 0
			bestDist := distanceSquared(p, km.Centroids[0])

			for j := 1; j < km.K; j++ {
				d := distanceSquared(p, km.Centroids[j])
				if d < bestDist {
					bestDist = d
					bestCluster = j
				}
			}

			if km.Assignments[i] != bestCluster {
				km.Assignments[i] = bestCluster
				changed++
			}
		}

		if changed == 0 {
			fmt.Printf("[K-Means] Iteracao %d/%d: convergiu (%v)\n", iter+1, maxIter, time.Since(iterStart))
			break
		}

		fmt.Printf("[K-Means] Iteracao %d/%d: %d pontos mudaram de cluster (%v)\n",
			iter+1, maxIter, changed, time.Since(iterStart))

		newCentroids := make([][14]float64, km.K)
		counts := make([]int, km.K)

		for i, p := range km.Points {
			c := km.Assignments[i]
			counts[c]++
			for j := 0; j < 14; j++ {
				newCentroids[c][j] += float64(p.Vector[j])
			}
		}

		for i := 0; i < km.K; i++ {
			if counts[i] > 0 {
				for j := 0; j < 14; j++ {
					km.Centroids[i][j] = newCentroids[i][j] / float64(counts[i])
				}
			}
		}
	}
}

type ClusterResult struct {
	Points []Point
	Sizes  []int
}

func (km *KMeans) Reorganize() ClusterResult {
	sizes := make([]int, km.K)
	for _, c := range km.Assignments {
		sizes[c]++
	}

	clusters := make([][]Point, km.K)
	for i := 0; i < km.K; i++ {
		clusters[i] = make([]Point, 0, sizes[i])
	}

	for i, p := range km.Points {
		c := km.Assignments[i]
		clusters[c] = append(clusters[c], p)
	}

	var allPoints []Point
	for i := 0; i < km.K; i++ {
		allPoints = append(allPoints, clusters[i]...)
	}

	return ClusterResult{
		Points: allPoints,
		Sizes:  sizes,
	}
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
	fmt.Printf("Iniciando K-Means com k=%d...\n", k)
	start := time.Now()

	km := NewKMeans(points, k)
	km.Run(20)

	fmt.Println("[K-Means] Reorganizando registros por cluster...")
	result := km.Reorganize()
	fmt.Printf("[K-Means] Concluido em %v\n", time.Since(start))

	// Calcular bounding boxes
	bboxMin := make([][14]float32, k)
	bboxMax := make([][14]float32, k)
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			bboxMin[i][j] = math.MaxFloat32
			bboxMax[i][j] = -math.MaxFloat32
		}
	}

	idx := 0
	for i := 0; i < k; i++ {
		for r := 0; r < result.Sizes[i]; r++ {
			p := result.Points[idx]
			for j := 0; j < 14; j++ {
				if p.Vector[j] < bboxMin[i][j] {
					bboxMin[i][j] = p.Vector[j]
				}
				if p.Vector[j] > bboxMax[i][j] {
					bboxMax[i][j] = p.Vector[j]
				}
			}
			idx++
		}
	}

	fmt.Println("[K-Means] Escrevendo dataset.bin...")
	binFile, err := os.Create("dataset.bin")
	if err != nil {
		fmt.Println("Erro ao criar binario:", err)
		return
	}
	defer binFile.Close()

	n := uint32(len(points))
	binary.Write(binFile, binary.LittleEndian, uint32(k))
	binary.Write(binFile, binary.LittleEndian, n)

	// Centroides
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, float32(km.Centroids[i][j]))
		}
	}

	// Bounding boxes
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, bboxMin[i][j])
		}
	}
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, bboxMax[i][j])
		}
	}

	// Tamanhos
	for i := 0; i < k; i++ {
		binary.Write(binFile, binary.LittleEndian, uint32(result.Sizes[i]))
	}

	// Registros
	for _, p := range result.Points {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, p.Vector[j])
		}
		binary.Write(binFile, binary.LittleEndian, p.Label)
		binary.Write(binFile, binary.LittleEndian, p.OrigID)
	}

	binFile.Sync()
	fmt.Printf("[K-Means] dataset.bin gerado com sucesso! (k=%d, n=%d)\n", k, len(points))
}
