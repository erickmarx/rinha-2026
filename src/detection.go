package src

import "sort"

// Detected representa um vizinho proximo encontrado no dataset.
type Detected struct {
	Distance uint64
	Label    string
	OrigID   uint32
}

// centroidDistance calcula a distancia euclidiana ao quadrado entre um input
// e o centroide de um cluster (em float64, so usado para escolher cluster).
func centroidDistance(input Vector, c [14]float32) float64 {
	var sum float64
	for i := 0; i < 14; i++ {
		diff := input[i] - float64(c[i])
		sum += diff * diff
	}
	return sum
}

// lowerBoundBoundingBox calcula a distancia minima possivel entre a query
// quantizada e qualquer ponto dentro da bounding box do cluster.
func lowerBoundBoundingBox(q [14]int16, cluster Cluster) uint64 {
	var sum uint64
	for i := 0; i < 14; i++ {
		v := int64(q[i])
		minVal := int64(cluster.BboxMin[i])
		maxVal := int64(cluster.BboxMax[i])
		if v < minVal {
			diff := v - minVal
			sum += uint64(diff * diff)
		} else if v > maxVal {
			diff := v - maxVal
			sum += uint64(diff * diff)
		}
	}
	return sum
}

// knnInsert insere um candidato no top-k mantendo ordenado por distancia.
// Em caso de empate na distancia, desempata pelo OrigID menor.
func knnInsert(top *[5]Detected, count *int, k int, dist uint64, label string, origID uint32, worstDist *uint64) {
	pos := *count
	for i := 0; i < *count; i++ {
		if dist < top[i].Distance {
			pos = i
			break
		}
		if dist == top[i].Distance && origID < top[i].OrigID {
			pos = i
			break
		}
	}
	if pos >= k {
		return
	}
	for i := *count; i > pos; i-- {
		if i < k {
			top[i] = top[i-1]
		}
	}
	top[pos] = Detected{Distance: dist, Label: label, OrigID: origID}
	if *count < k {
		*count++
	}
	if *count == k {
		*worstDist = top[k-1].Distance
	}
}

// dimScanOrder é a ordem de dimensões do projeto C para early termination.
var dimScanOrder = [14]int{5, 6, 2, 0, 7, 8, 11, 12, 9, 10, 1, 13, 3, 4}

// scanCluster escaneia todos os registros de um cluster usando SoA int16
// com early termination após cada dimensão (ordem empírica do C).
func scanCluster(q [14]int16, clusterIdx int, top *[5]Detected, count *int, worstDist *uint64) {
	c := clusters[clusterIdx]
	for i := c.Start; i < c.End; i++ {
		var sum uint64
		skip := false
		for _, dj := range dimScanOrder {
			diff := int64(q[dj]) - int64(dimData[dj][i])
			sum += uint64(diff * diff)
			if sum > *worstDist {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		label := "fraud"
		if labelsData[i] == 1 {
			label = "legit"
		}
		knnInsert(top, count, 5, sum, label, origIDsData[i], worstDist)
	}
}

const fixScale = 10000.0

func quantizeInput(v float64) int16 {
	x := v * fixScale
	if x > 32767.0 {
		return 32767
	}
	if x < -32768.0 {
		return -32768
	}
	if x >= 0 {
		return int16(x + 0.5)
	}
	return int16(x - 0.5)
}

// Detect calcula o score de fraude usando IVF com bounding-box repair limitado.
func Detect(input Vector) int {
	// Quantiza query para int16 uma unica vez.
	var q [14]int16
	var qGrid [14]float64
	for i := 0; i < 14; i++ {
		q[i] = quantizeInput(input[i])
		qGrid[i] = float64(q[i]) / fixScale
	}

	// PASSO 1: Encontra o cluster mais proximo usando qGrid (igual ao C).
	bestCluster := 0
	bestDist := centroidDistance(qGrid, clusters[0].Centroid)
	for i := 1; i < len(clusters); i++ {
		d := centroidDistance(qGrid, clusters[i].Centroid)
		if d < bestDist {
			bestDist = d
			bestCluster = i
		}
	}

	// PASSO 2: Escaneia o cluster inicial.
	var top [5]Detected
	var count int
	var worstDist uint64 = 1<<63 - 1 // max uint64 aprox

	var visited [256]bool
	scanCluster(q, bestCluster, &top, &count, &worstDist)
	visited[bestCluster] = true

	// PASSO 3: Repair limitado — so se ainda nao temos 5 vizinhos.
	if count < 5 {
		type repairCand struct {
			idx int
			lb  uint64
		}
		var cands [256]repairCand
		candCount := 0
		for i := range clusters {
			if visited[i] {
				continue
			}
			lb := lowerBoundBoundingBox(q, clusters[i])
			cands[candCount] = repairCand{idx: i, lb: lb}
			candCount++
		}
		// Ordena pelos mais promissores (menor lower_bound)
		sort.Slice(cands[:candCount], func(i, j int) bool {
			return cands[i].lb < cands[j].lb
		})
		// Escaneia todos os clusters promissores ate encontrar 5 vizinhos
		for i := 0; i < candCount && count < 5; i++ {
			scanCluster(q, cands[i].idx, &top, &count, &worstDist)
		}
	}

	// PASSO 4: Conta frauds.
	fraudCount := 0
	for i := 0; i < count; i++ {
		if top[i].Label == "fraud" {
			fraudCount++
		}
	}

	return fraudCount
}
