package src

// Detected representa um vizinho proximo encontrado no dataset.
// Distance eh a distancia euclidiana ao quadrado.
type Detected struct {
	Distance float64
	Label    string
}

// centroidDistance calcula a distancia euclidiana ao quadrado entre um input
// e o centroide de um cluster. Usado para decidir qual cluster explorar.
func centroidDistance(input Vector, c [14]float32) float64 {
	var sum float64
	for i := 0; i < 14; i++ {
		diff := input[i] - float64(c[i])
		sum += diff * diff
	}
	return sum
}

// findNearestClusters compara o input com os centroides de todos os clusters
// e retorna os indices dos 3 clusters mais proximos.
// ZERO ALOCACAO: retorna array fixo [3]int ao inves de slice.
func findNearestClusters(input Vector, n int) [3]int {
	type clusterDist struct {
		idx      int
		distance float64
	}
	var topN [3]clusterDist
	var topNLen int

	for i := range len(clusters) {
		d := centroidDistance(input, clusters[i].Centroid)

		pos := topNLen
		for j := 0; j < topNLen; j++ {
			if d < topN[j].distance {
				pos = j
				break
			}
		}

		if pos < n {
			for j := topNLen; j > pos; j-- {
				if j < n {
					topN[j] = topN[j-1]
				}
			}
			topN[pos] = clusterDist{idx: i, distance: d}
			if topNLen < n {
				topNLen++
			}
		}
	}

	var result [3]int
	for i := 0; i < topNLen; i++ {
		result[i] = topN[i].idx
	}
	return result
}

// Detect calcula o score de fraude usando KNN (k=5) com busca em multiplos clusters.
// Retorna o numero de frauds entre os 5 vizinhos mais proximos (0-5).
//
// ESTRATEGIA:
// 1. Encontra os 3 clusters mais proximos do input comparando com os centroides.
// 2. Faz scan linear nos registros dos 3 clusters (~150 registros no total).
// 3. Retorna a contagem de frauds nos 5 vizinhos mais proximos.
func Detect(input Vector) int {
	nearest := findNearestClusters(input, 3)

	var top [5]Detected
	var count int

	for _, clusterIdx := range nearest {
		data := clusters[clusterIdx].Data

		for i := range len(data) {
			reg := &data[i]

			var sum float32
			for j := range 14 {
				diff := float32(input[j]) - reg.Vector[j]
				sum += diff * diff
			}

			pos := count
			for j := range count {
				if float64(sum) < top[j].Distance {
					pos = j
					break
				}
			}

			if pos >= 5 {
				continue
			}

			for j := count; j > pos; j-- {
				if j < 5 {
					top[j] = top[j-1]
				}
			}

			label := "fraud"
			if reg.Label == 1 {
				label = "legit"
			}

			top[pos] = Detected{Distance: float64(sum), Label: label}
			if count < 5 {
				count++
			}
		}
	}

	fraudCount := 0
	for i := range count {
		if top[i].Label == "fraud" {
			fraudCount++
		}
	}

	return fraudCount
}
