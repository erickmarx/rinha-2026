package src

type Detected struct {
	Distance float64
	Label    string
}

func labelString(code uint32) string {
	if code == 1 {
		return "legit"
	}
	return "fraud"
}

func Detect(input Vector) []Detected {
	// Para simplificar o estudo, vamos focar em encontrar o Top 1 mais similar
	var bestDist float64 = 3.4e38 // Infinito (Max Float32)
	var bestLabel uint32

	for i := range len(dataset) {
		reg := &dataset[i] // Ponteiro para evitar cópia da struct no loop
		var sum float64

		// Cálculo da Euclidiana Quadrática
		for j := range 14 {
			diff := input[j] - float64(reg.Vector[j])
			sum += diff * diff
		}

		if sum < bestDist {
			bestDist = sum
			bestLabel = reg.Label
		}
	}

	return []Detected{{Distance: bestDist, Label: labelString(bestLabel)}}
}
