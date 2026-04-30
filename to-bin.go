package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"
)

// RegistroJSON representa um registro lido do arquivo JSON de referencias.
type RegistroJSON struct {
	Vector []float32 `json:"vector"`
	Label  string    `json:"label"`
}

// Point representa um ponto no espaco 14D com seu label.
type Point struct {
	Vector [14]float32
	Label  uint32
}

// KMeans implementa o algoritmo de Lloyd para clustering.
// Roda OFFLINE (no to-bin.go), entao performance nao eh critica —
// precisamos de corretude e uma boa divisao dos dados.
type KMeans struct {
	Points      []Point
	Centroids   [][14]float64
	Assignments []int
	K           int
}

// NewKMeans inicializa o k-means com k clusters.
// Os centroides iniciais sao amostrados aleatoriamente do proprio dataset.
func NewKMeans(points []Point, k int) *KMeans {
	km := &KMeans{
		Points:      points,
		Centroids:   make([][14]float64, k),
		Assignments: make([]int, len(points)),
		K:           k,
	}

	// Semente fixa para geracao deterministica: toda execucao gera
	// os mesmos clusters, garantindo reprodutibilidade entre builds.
	rand.Seed(42)
	for i := 0; i < k; i++ {
		idx := rand.Intn(len(points))
		for j := 0; j < 14; j++ {
			km.Centroids[i][j] = float64(points[idx].Vector[j])
		}
	}

	return km
}

// distanceSquared calcula a distancia euclidiana ao quadrado.
func distanceSquared(p Point, c [14]float64) float64 {
	var sum float64
	for i := 0; i < 14; i++ {
		diff := float64(p.Vector[i]) - c[i]
		sum += diff * diff
	}
	return sum
}

// Run executa o algoritmo de Lloyd por no maximo maxIter iteracoes.
func (km *KMeans) Run(maxIter int) {
	for iter := 0; iter < maxIter; iter++ {
		changed := 0
		iterStart := time.Now()

		// PASSO 1: Atribuir cada ponto ao centroide mais proximo.
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

		// Se nenhum ponto mudou de cluster, convergiu.
		if changed == 0 {
			fmt.Printf("[K-Means] Iteracao %d/%d: convergiu (%v)\n", iter+1, maxIter, time.Since(iterStart))
			break
		}

		fmt.Printf("[K-Means] Iteracao %d/%d: %d pontos mudaram de cluster (%v)\n",
			iter+1, maxIter, changed, time.Since(iterStart))

		// PASSO 2: Recalcular centroides como media dos pontos atribuidos.
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

// ClusterResult contem os pontos reorganizados por cluster e os tamanhos.
type ClusterResult struct {
	Points []Point
	Sizes  []int
}

// Reorganize agrupa os pontos por cluster para escrita sequencial no binario.
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
	// 1. Le o JSON de referencias.
	content, err := os.ReadFile("references.json")
	if err != nil {
		fmt.Println("Erro ao ler JSON:", err)
		return
	}

	var registrosJSON []RegistroJSON
	json.Unmarshal(content, &registrosJSON)
	fmt.Printf("Carregados %d registros do JSON\n", len(registrosJSON))

	// Converte para o tipo Point.
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
	}
	fmt.Printf("Convertidos %d registros para formato interno\n", len(points))

	// 2. Executa K-Means com k=15000 clusters.
	// Clusters menores (~67 registros cada) reduzem a chance de misturar
	// fraudes e legitimos no mesmo grupo, melhorando a precisao do KNN.
	const k = 15000
	fmt.Printf("Iniciando K-Means com k=%d...\n", k)
	start := time.Now()

	km := NewKMeans(points, k)
	km.Run(20)

	fmt.Println("[K-Means] Reorganizando registros por cluster...")
	result := km.Reorganize()
	fmt.Printf("[K-Means] Concluido em %v\n", time.Since(start))

	// Estatisticas dos clusters.
	minSize, maxSize := math.MaxInt64, 0
	total := 0
	for _, s := range result.Sizes {
		if s < minSize {
			minSize = s
		}
		if s > maxSize {
			maxSize = s
		}
		total += s
	}
	fmt.Printf("Clusters: min=%d, max=%d, media=%d\n", minSize, maxSize, total/k)

	// 3. Escreve o arquivo binario no novo formato.
	fmt.Println("[K-Means] Escrevendo dataset.bin...")
	binFile, err := os.Create("dataset.bin")
	if err != nil {
		fmt.Println("Erro ao criar binario:", err)
		return
	}
	defer binFile.Close()

	// Header: k, n
	binary.Write(binFile, binary.LittleEndian, uint32(k))
	binary.Write(binFile, binary.LittleEndian, uint32(len(points)))

	// Centroides: k x 14 floats
	for i := 0; i < k; i++ {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, float32(km.Centroids[i][j]))
		}
	}

	// Tamanhos dos clusters: k x uint32
	for i := 0; i < k; i++ {
		binary.Write(binFile, binary.LittleEndian, uint32(result.Sizes[i]))
	}

	// Registros agrupados por cluster
	for _, p := range result.Points {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, p.Vector[j])
		}
		binary.Write(binFile, binary.LittleEndian, p.Label)
	}

	binFile.Sync()
	fmt.Printf("[K-Means] dataset.bin gerado com sucesso! (k=%d, n=%d)\n", k, len(points))
}
