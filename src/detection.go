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

// findNearestCluster compara o input com os centroides de todos os clusters
// e retorna o indice do cluster mais proximo.
// Essa operacao eh O(k) onde k=1000, muito rapida comparada ao scan de 1M.
func findNearestCluster(input Vector) int {
	best := 0
	bestDist := centroidDistance(input, clusters[0].Centroid)

	for i := 1; i < len(clusters); i++ {
		d := centroidDistance(input, clusters[i].Centroid)
		if d < bestDist {
			bestDist = d
			best = i
		}
	}

	return best
}

// Detect calcula o score de fraude usando KNN (k=5) com busca por cluster.
//
// ESTRATEGIA:
// 1. Encontra o cluster mais proximo do input comparando com os centroides.
// 2. Faz scan linear APENAS nos registros daquele cluster (~1k registros).
// 3. Dos vizinhos encontrados no cluster, pega os 5 mais proximos.
//
// Por que funciona:
// - O k-means offline agrupou registros similares em clusters.
// - O centroide representa o "centro" do grupo.
// - Se o input esta proximo de um centroide, seus vizinhos mais proximos
//   provavelmente estao no mesmo cluster.
// - Reducao de 1.000.000 para ~1.000 comparacoes = 1000x mais rapido.
func Detect(input Vector) FraudScore {
	// PASSO 1: Descobre qual cluster olhar.
	clusterIdx := findNearestCluster(input)
	data := clusters[clusterIdx].Data

	// PASSO 2: KNN scan linear apenas dentro do cluster.
	var top [5]Detected
	var count int

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

	// PASSO 3: Classifica baseado nos 5 vizinhos mais proximos do cluster.
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
