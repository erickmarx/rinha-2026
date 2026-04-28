package src

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
	var top []Detected

	for i := range len(dataset) {
		reg := &dataset[i]
		var sum float64

		for j := range 14 {
			diff := input[j] - float64(reg.Vector[j])
			sum += diff * diff
		}

		// Encontra a posição de inserção (ordem crescente de distância)
		pos := len(top)

		for j := range top {
			if sum < top[j].Distance {
				pos = j
				break
			}
		}

		// Se cabe no top 5, insere ordenado
		if pos < 5 {
			top = append(top, Detected{})
			copy(top[pos+1:], top[pos:])
			top[pos] = Detected{
				Distance: sum,
				Label:    labelString(reg.Label),
			}
			if len(top) > 5 {
				top = top[:5]
			}
		}
	}

	count := 0

	for _, score := range top {
		if score.Label == "fraud" {
			count++
		}
	}

	ratio := float32(count) / 5.0

	return FraudScore{
		Approved:   ratio < float32(0.6),
		FraudScore: ratio,
	}
}
