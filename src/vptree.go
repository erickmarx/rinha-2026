package src

// VPNode representa um no da VP-Tree (Vantage Point Tree).
// A arvore eh construida offline no to-bin.go e carregada via mmap.
type VPNode struct {
	Index     int32   // indice do vantage point no dataset
	Threshold float32 // mediana das distancias (limite inside/outside)
	Inside    int32   // indice do filho inside no array vpNodes (-1 = nil)
	Outside   int32   // indice do filho outside no array vpNodes (-1 = nil)
}

var (
	vpNodes []VPNode
	vpRoot  int32 = -1
)

// distanceVecReg calcula a distancia euclidiana ao quadrado entre um input
// (Vector = [14]float64) e um registro do dataset ([14]float32).
func distanceVecReg(input Vector, reg Registry) float64 {
	var sum float64
	for i := 0; i < 14; i++ {
		diff := input[i] - float64(reg.Vector[i])
		sum += diff * diff
	}
	return sum
}

// knnInsert insere um candidato no top-k mantendo ordenado por distancia.
func knnInsert(top *[5]Detected, count *int, k int, dist float64, label string, worstDist *float64) {
	pos := *count
	for i := 0; i < *count; i++ {
		if dist < top[i].Distance {
			pos = i
			break
		}
	}
	if pos >= k {
		return
	}
	for i := *count; i > pos; i-- {
		if i < k {
			top[i] = top[i-1]
		}
	}
	top[pos] = Detected{Distance: dist, Label: label}
	if *count < k {
		*count++
	}
	if *count == k {
		*worstDist = top[k-1].Distance
	}
}

// knnSearch percorre a VP-Tree recursivamente encontrando os k vizinhos mais proximos.
// Usa pruning para evitar explorar subtrees que nao podem conter vizinhos melhores.
func knnSearch(nodeIdx int32, input Vector, k int, worstDist *float64, top *[5]Detected, count *int) {
	if nodeIdx < 0 || int(nodeIdx) >= len(vpNodes) {
		return
	}
	node := &vpNodes[nodeIdx]

	// Distancia ao vantage point deste no
	dist := distanceVecReg(input, dataset[node.Index])

	label := "fraud"
	if dataset[node.Index].Label == 1 {
		label = "legit"
	}
	knnInsert(top, count, k, dist, label, worstDist)

	// Decidir ordem de exploracao: filho mais proximo primeiro
	var first, second int32
	if dist < float64(node.Threshold) {
		first = node.Inside
		second = node.Outside
	} else {
		first = node.Outside
		second = node.Inside
	}

	knnSearch(first, input, k, worstDist, top, count)

	// Pruning: so explora o outro filho se a regiao dele pode conter vizinhos melhores
	diff := dist - float64(node.Threshold)
	if diff < 0 {
		diff = -diff
	}
	if *count < k || diff < *worstDist {
		knnSearch(second, input, k, worstDist, top, count)
	}
}
