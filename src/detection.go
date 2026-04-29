package src

// Detected representa um vizinho proximo encontrado no dataset.
// Distance eh a distancia euclidiana ao quadrado (evitamos sqrt porque
// so precisamos comparar quem eh menor, e sqrt preserva a ordem).
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

func Detect(input Vector) FraudScore {
	var top [5]Detected
	var count int

	for i := range len(dataset) {
		reg := &dataset[i]

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

		// Shift manual dos elementos para a direita, abrindo espaco.
		// Fazemos de tras para frente para nao sobrescrever dados.
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
