package src

import (
	"runtime"
	"sync"
)

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

// detectChunk processa um pedaço do dataset (de start ate end, exclusivo)
// e retorna os 5 vizinhos mais proximos dentro desse chunk.
func detectChunk(input Vector, start, end int) []Detected {
	var top []Detected

	for i := start; i < end; i++ {
		reg := &dataset[i]
		var sum float64

		for j := range 14 {
			diff := input[j] - float64(reg.Vector[j])
			sum += diff * diff
		}

		// Encontra a posicao de insercao (ordem crescente de distancia)
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

	return top
}

// mergeTops combina varios tops locais em um unico top 5 global.
func mergeTops(tops [][]Detected) []Detected {
	var global []Detected
	for _, top := range tops {
		for _, d := range top {
			pos := len(global)
			for j := range global {
				if d.Distance < global[j].Distance {
					pos = j
					break
				}
			}
			if pos < 5 {
				global = append(global, Detected{})
				copy(global[pos+1:], global[pos:])
				global[pos] = d
				if len(global) > 5 {
					global = global[:5]
				}
			}
		}
	}
	return global
}

// Detect calcula o score de fraude usando KNN (k=5).
// O dataset eh dividido em chunks e processado em paralelo por goroutines,
// uma por CPU logica disponivel. Isso reduz drasticamente o tempo de resposta
// comparado ao scan linear sequencial.
func Detect(input Vector) FraudScore {
	n := runtime.GOMAXPROCS(0) // numero de CPUs logicas
	total := len(dataset)
	if total == 0 {
		return FraudScore{Approved: true, FraudScore: 0}
	}

	chunkSize := (total + n - 1) / n // arredonda para cima

	// Slice para armazenar o resultado de cada goroutine.
	locals := make([][]Detected, n)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		start := i * chunkSize
		if start >= total {
			break
		}
		end := start + chunkSize
		if end > total {
			end = total
		}

		wg.Add(1)
		go func(idx, s, e int) {
			defer wg.Done()
			locals[idx] = detectChunk(input, s, e)
		}(i, start, end)
	}

	wg.Wait()

	// Junta os tops locais em um top 5 global.
	top := mergeTops(locals)

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
