package src

// Detected representa um vizinho proximo encontrado no dataset.
// Distance eh a distancia euclidiana ao quadrado.
type Detected struct {
	Distance float64
	Label    string
}

type FraudScore struct {
	Approved   bool    `json:"approved"`
	FraudScore float32 `json:"fraud_score"`
}

func labelString(code uint32) string {
	if code == 1 {
		return "legit"
	}
	return "fraud"
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
// e retorna os indices dos N clusters mais proximos.
// Essa operacao eh O(k) onde k=1000, muito rapida comparada ao scan de 1M.
func findNearestClusters(input Vector, n int) []int {
	// topN mantem os N clusters mais proximos em ordem crescente de distancia.
	type clusterDist struct {
		idx      int
		distance float64
	}
	var topN []clusterDist

	for i := range len(clusters) {
		d := centroidDistance(input, clusters[i].Centroid)

		pos := len(topN)
		for j := range topN {
			if d < topN[j].distance {
				pos = j
				break
			}
		}

		if pos < n {
			topN = append(topN, clusterDist{})
			copy(topN[pos+1:], topN[pos:])
			topN[pos] = clusterDist{idx: i, distance: d}
			if len(topN) > n {
				topN = topN[:n]
			}
		}
	}

	result := make([]int, len(topN))
	for i, cd := range topN {
		result[i] = cd.idx
	}
	return result
}

// Detect calcula o score de fraude usando KNN (k=5) com busca em multiplos clusters.
//
// ESTRATEGIA:
// 1. Encontra os 5 clusters mais proximos do input comparando com os centroides.
// 2. Faz scan linear nos registros dos 5 clusters (~250 registros no total).
// 3. Dos vizinhos encontrados em todos os clusters, pega os 5 mais proximos.
//
// Por que 5 clusters:
// - O vizinho real pode estar em um cluster adjacente, nao necessariamente
//   no centroide mais proximo. Escannear 5 clusters recupera mais da
//   precisao do scan completo com custo ainda baixo.
// - Reducao de 1.000.000 para ~250 comparacoes = ~4000x mais rapido.
func Detect(input Vector) FraudScore {
	// PASSO 1: Descobre os 5 clusters mais proximos.
	nearest := findNearestClusters(input, 5)

	// PASSO 2: KNN scan linear nos registros dos 5 clusters combinados.
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

	// PASSO 3: Classifica baseado nos 5 vizinhos mais proximos dos 3 clusters.
	fraudCount := 0
	for i := range count {
		if top[i].Label == "fraud" {
			fraudCount++
		}
	}

	ratio := float32(fraudCount) / 5.0
	return FraudScore{
		Approved:   ratio < float32(0.6),
		FraudScore: ratio,
	}
}
