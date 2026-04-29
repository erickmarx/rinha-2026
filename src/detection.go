package src

import (
	"runtime"
	"sync"
)

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

// ============================================================================
// DETECAO SEQUENCIAL OTIMIZADA (uso quando ha apenas 1 CPU)
// ============================================================================
//
// Por que uma versao sequencial separada?
// - Com apenas 1 CPU, goroutines nao executam de verdade em paralelo;
//   elas apenas se alternam via context-switching, que consome ciclos preciosos.
// - Versao paralela aloca slices e usa WaitGroup: overhead puro em 1 CPU.
// - Aqui usamos um array fixo [5]Detected no stack (zero alocacoes no heap)
//   e evitamos copy/append no hot path.
//
// Otimizacoes aplicadas:
// 1. Array fixo [5]Detected em vez de slice dinamico → zero alocacoes.
// 2. Sem append/copy no hot path → shift manual com atribuicoes diretas.
// 3. Loop de distancia com float32 → o dataset original eh float32;
//    trabalhar com float32 melhora cache locality (menos bytes trafegados
//    entre CPU e RAM) e deixa o calculo mais leve.
// 4. Label inline (sem chamada de funcao) → o compilador Go faz inline
//    de funcoes pequenas, mas garantimos que nao haja overhead de call.

func detectSequential(input Vector) FraudScore {
	// top eh um array fixo de 5 posicoes no stack.
	// Diferente de slice, array tem tamanho conhecido em tempo de compilacao
	// e nao precisa de alocacao dinamica no heap.
	var top [5]Detected
	var count int

	// Scan linear no dataset inteiro. Com 1 CPU, nao ha como escapar do scan,
	// mas fazemos o minimo de trabalho possivel por registro.
	for i := range len(dataset) {
		reg := &dataset[i]

		// Distancia euclidiana ao quadrado entre o input e o registro.
		// Usamos float32 porque o dataset.Vector eh [14]float32.
		// Menor precisao = menor uso de memoria bandwidth = mais registros
		// processados por segundo (cache-friendly).
		var sum float32
		for j := range 14 {
			diff := float32(input[j]) - reg.Vector[j]
			sum += diff * diff
		}

		// Encontra posicao de insercao no top 5 (ordem crescente de distancia).
		// Quanto menor a distancia, mais "proximo" o vizinho eh.
		pos := count
		for j := 0; j < count; j++ {
			if float64(sum) < top[j].Distance {
				pos = j
				break
			}
		}

		// Se a distancia eh maior que todos os 5 atuais, descarta.
		if pos >= 5 {
			continue
		}

		// Shift manual dos elementos para a direita, abrindo espaco.
		// Fazemos de tras para frente para nao sobrescrever dados.
		// Isso evita a chamada a copy() que a versao antiga usava.
		for j := count; j > pos; j-- {
			if j < 5 {
				top[j] = top[j-1]
			}
		}

		// Decide o label sem chamar funcao (inline).
		label := "fraud"
		if reg.Label == 1 {
			label = "legit"
		}

		// Insere o novo vizinho na posicao correta.
		top[pos] = Detected{Distance: float64(sum), Label: label}
		if count < 5 {
			count++
		}
	}

	// Classificacao final: conta quantos dos 5 vizinhos mais proximos sao fraudes.
	fraudCount := 0
	for i := 0; i < count; i++ {
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

// ============================================================================
// DETECAO PARALELA (uso quando ha 2+ CPUs)
// ============================================================================
//
// Divide o dataset em N pedacos (onde N = numero de CPUs logicas).
// Cada pedaco eh processado por uma goroutine independente.
// Ao final, mergeamos os tops 5 locais em um unico top 5 global.
//
// Por que funciona bem com multiplas CPUs?
// - O dataset eh read-only (mmap), entao nao ha condicao de corrida.
// - Cada goroutine trabalha em um subconjunto disjunto de dados,
//   maximizando uso do cache L1/L2 de cada core.

// detectChunk processa um pedaco do dataset (indices [start, end))
// e retorna os 5 vizinhos mais proximos dentro desse pedaco.
func detectChunk(input Vector, start, end int) []Detected {
	// Aqui usamos slice porque o tamanho varia de 0 a 5.
	// Como o chunk eh processado uma unica vez, a allocacao eh aceitavel.
	var top []Detected

	for i := start; i < end; i++ {
		reg := &dataset[i]
		var sum float64
		for j := range 14 {
			diff := input[j] - float64(reg.Vector[j])
			sum += diff * diff
		}

		pos := len(top)
		for j := range top {
			if sum < top[j].Distance {
				pos = j
				break
			}
		}

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

// mergeTops combina N tops locais (cada um com no maximo 5 elementos)
// em um unico top 5 global. Como o total de elementos eh pequeno
// (5 * N, onde N = numero de CPUs), o merge eh trivial.
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

// detectParallel divide o trabalho entre varias goroutines.
func detectParallel(input Vector) FraudScore {
	n := runtime.GOMAXPROCS(0) // numero de CPUs logicas disponiveis
	total := len(dataset)
	if total == 0 {
		return FraudScore{Approved: true, FraudScore: 0}
	}

	// Divide o dataset em N chunks de tamanho aproximadamente igual.
	// O +n-1 com divisao inteira faz arredondamento para cima,
	// garantindo que nenhum registro fique de fora.
	chunkSize := (total + n - 1) / n

	// Slice para receber o resultado de cada goroutine.
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
		// Cada goroutine processa seu chunk independentemente.
		go func(idx, s, e int) {
			defer wg.Done()
			locals[idx] = detectChunk(input, s, e)
		}(i, start, end)
	}

	// Aguarda todas as goroutines terminarem antes de mergear.
	wg.Wait()

	// Combina os resultados parciais em um unico top 5 global.
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

// ============================================================================
// ENTRYPOINT: Detect escolhe a estrategia correta automaticamente
// ============================================================================
// Detect seleciona entre a versao sequencial (1 CPU) ou paralela (2+ CPUs).
// Isso evita overhead desnecessario quando o hardware nao tem paralelismo real.
func Detect(input Vector) FraudScore {
	if runtime.GOMAXPROCS(0) > 1 {
		return detectParallel(input)
	}
	return detectSequential(input)
}
