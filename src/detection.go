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

// Detect calcula o score de fraude usando KNN (k=7) com busca em multiplos clusters.
//
// ESTRATEGIA:
// 1. Encontra os 3 clusters mais proximos do input comparando com os centroides.
// 2. Faz scan linear nos registros dos 3 clusters (~150 registros no total).
// 3. Dos vizinhos encontrados em todos os clusters, pega os 7 mais proximos.
//
// Por que k=7:
// - Mais vizinhos no voto majoritario reduz impacto de outliers/ruido.
// - Threshold ajustado proporcionalmente (~0.57 vs 0.60 anterior).
//
// Por que 3 clusters:
// - O vizinho real pode estar em um cluster adjacente, nao necessariamente
//   no centroide mais proximo. Escannear 3 clusters recupera a maioria da
//   precisao do scan completo com custo minimo.
// - Reducao de 1.000.000 para ~150 comparacoes = ~6700x mais rapido.
func Detect(input Vector) FraudScore {
	// PASSO 1: Descobre os 3 clusters mais proximos.
	nearest := findNearestClusters(input, 3)

	// PASSO 2: KNN scan linear nos registros dos 3 clusters combinados.
	const knnK = 7
	var top [knnK]Detected
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

			if pos >= knnK {
				continue
			}

			for j := count; j > pos; j-- {
				if j < knnK {
					top[j] = top[j-1]
				}
			}

			label := "fraud"
			if reg.Label == 1 {
				label = "legit"
			}

			top[pos] = Detected{Distance: float64(sum), Label: label}
			if count < knnK {
				count++
			}
		}
	}

	// PASSO 3: Classifica baseado nos 7 vizinhos mais proximos dos 3 clusters.
	fraudCount := 0
	for i := range count {
		if top[i].Label == "fraud" {
			fraudCount++
		}
	}

	ratio := float32(fraudCount) / knnK
	// Threshold fixo em 0.60 conforme regras da competicao.
	return FraudScore{
		Approved:   ratio < float32(0.60),
		FraudScore: ratio,
	}
}
