package src

import "math"

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

// Detect calcula o score de fraude usando KNN (k=5) com VP-Tree.
//
// ESTRATEGIA:
// 1. Percorre a VP-Tree recursivamente com pruning.
// 2. Coleta os 5 vizinhos mais proximos de TODOS os nos visitados.
// 3. Classifica baseado nos 5 vizinhos.
//
// A VP-Tree reduz a busca de O(n) para O(log n) em media, com pruning
// que evita explorar regioes do espaco que nao podem conter vizinhos
// melhores que os ja encontrados.
func Detect(input Vector) FraudScore {
	var top [5]Detected
	var count int
	var worstDist float64 = math.MaxFloat64

	knnSearch(vpRoot, input, 5, &worstDist, &top, &count)

	fraudCount := 0
	for i := 0; i < count; i++ {
		if top[i].Label == "fraud" {
			fraudCount++
		}
	}

	ratio := float32(fraudCount) / 5.0
	// Threshold fixo em 0.60 conforme regras da competicao.
	return FraudScore{
		Approved:   ratio < float32(0.60),
		FraudScore: ratio,
	}
}
