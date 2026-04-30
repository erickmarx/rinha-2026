package src

import "sort"

// Detected representa um vizinho proximo encontrado no dataset.
type Detected struct {
	Distance float64
	Label    string
	OrigID   uint32
}

// centroidDistance calcula a distancia euclidiana ao quadrado entre um input
// e o centroide de um cluster.
func centroidDistance(input Vector, c [14]float32) float64 {
	var sum float64
	for i := 0; i < 14; i++ {
		diff := input[i] - float64(c[i])
		sum += diff * diff
	}
	return sum
}

// lowerBoundBoundingBox calcula a distancia minima possivel entre o input
// e qualquer ponto dentro da bounding box do cluster.
func lowerBoundBoundingBox(input Vector, cluster Cluster) float64 {
	var sum float64
	for i := 0; i < 14; i++ {
		v := input[i]
		minVal := float64(cluster.BboxMin[i])
		maxVal := float64(cluster.BboxMax[i])
		if v < minVal {
			diff := v - minVal
			sum += diff * diff
		} else if v > maxVal {
			diff := v - maxVal
			sum += diff * diff
		}
	}
	return sum
}

// knnInsert insere um candidato no top-k mantendo ordenado por distancia.
// Em caso de empate na distancia, desempata pelo OrigID menor.
func knnInsert(top *[5]Detected, count *int, k int, dist float64, label string, origID uint32, worstDist *float64) {
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

// scanCluster escaneia todos os registros de um cluster.
func scanCluster(input32 *[14]float32, clusterIdx int, top *[5]Detected, count *int, worstDist *float64) {
	data := clusters[clusterIdx].Data
	for i := range len(data) {
		reg := &data[i]

		var sum float32
		for j := range 14 {
			diff := input32[j] - reg.Vector[j]
			sum += diff * diff
		}

		label := "fraud"
		if reg.Label == 1 {
			label = "legit"
		}
		knnInsert(top, count, 5, float64(sum), label, reg.OrigID, worstDist)
	}
}

// Detect calcula o score de fraude usando IVF com bounding-box repair limitado.
func Detect(input Vector) int {
	// PRE-CONVERSAO: float64 -> float32 uma unica vez.
	var input32 [14]float32
	for i := 0; i < 14; i++ {
		input32[i] = float32(input[i])
	}

	// PASSO 1: Encontra o cluster mais proximo (zero alocacao).
	bestCluster := 0
	bestDist := centroidDistance(input, clusters[0].Centroid)
	for i := 1; i < len(clusters); i++ {
		d := centroidDistance(input, clusters[i].Centroid)
		if d < bestDist {
			bestDist = d
			bestCluster = i
		}
	}

	// PASSO 2: Escaneia o cluster inicial.
	var top [5]Detected
	var count int
	var worstDist float64 = 1e308

	var visited [256]bool
	scanCluster(&input32, bestCluster, &top, &count, &worstDist)
	visited[bestCluster] = true

	// PASSO 3: Repair limitado — so se ainda nao temos 5 vizinhos.
	if count < 5 {
		type repairCand struct {
			idx int
			lb  float64
		}
		var cands [256]repairCand
		candCount := 0
		for i := range clusters {
			if visited[i] {
				continue
			}
			lb := lowerBoundBoundingBox(input, clusters[i])
			cands[candCount] = repairCand{idx: i, lb: lb}
			candCount++
		}
		// Ordena pelos mais promissores (menor lower_bound)
		sort.Slice(cands[:candCount], func(i, j int) bool {
			return cands[i].lb < cands[j].lb
		})
		// Escaneia no maximo 20 clusters adicionais
		limit := 20
		if candCount < limit {
			limit = candCount
		}
		for i := 0; i < limit && count < 5; i++ {
			scanCluster(&input32, cands[i].idx, &top, &count, &worstDist)
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
