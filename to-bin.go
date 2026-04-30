package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"time"
)

// RegistroJSON representa um registro lido do arquivo JSON de referencias.
type RegistroJSON struct {
	Vector []float32 `json:"vector"`
	Label  string    `json:"label"`
}

// Point representa um ponto no espaco 14D com seu label.
type Point struct {
	Vector [14]float32
	Label  uint32
}

// VPNode representa um no da VP-Tree serializado.
// Deve estar alinhado com a definicao em src/vptree.go.
type VPNode struct {
	Index     int32
	Threshold float32
	Inside    int32
	Outside   int32
}

// vpNodeBuild eh a representacao em memoria durante a construcao.
type vpNodeBuild struct {
	index     int
	threshold float32
	inside    *vpNodeBuild
	outside   *vpNodeBuild
}

// distance32 calcula a distancia euclidiana ao quadrado entre dois vetores float32.
func distance32(a, b [14]float32) float32 {
	var sum float32
	for i := 0; i < 14; i++ {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return sum
}

// buildVPTree constroi recursivamente a VP-Tree a partir dos indices fornecidos.
func buildVPTree(points []Point, indices []int) *vpNodeBuild {
	if len(indices) == 0 {
		return nil
	}

	vpPos := rand.Intn(len(indices))
	vpIdx := indices[vpPos]

	if len(indices) == 1 {
		return &vpNodeBuild{index: vpIdx}
	}

	vpVec := points[vpIdx].Vector
	dists := make([]struct{ idx int; dist float32 }, 0, len(indices)-1)
	for i, idx := range indices {
		if i == vpPos {
			continue
		}
		dists = append(dists, struct{ idx int; dist float32 }{
			idx:  idx,
			dist: distance32(vpVec, points[idx].Vector),
		})
	}

	sort.Slice(dists, func(i, j int) bool {
		return dists[i].dist < dists[j].dist
	})

	median := len(dists) / 2
	threshold := dists[median].dist

	inside := make([]int, 0, median)
	outside := make([]int, 0, len(dists)-median)
	for _, d := range dists {
		if d.dist < threshold {
			inside = append(inside, d.idx)
		} else {
			outside = append(outside, d.idx)
		}
	}

	return &vpNodeBuild{
		index:     vpIdx,
		threshold: threshold,
		inside:    buildVPTree(points, inside),
		outside:   buildVPTree(points, outside),
	}
}

// flattenVPTree converte a arvore recursiva em um array flat (adequado para mmap).
func flattenVPTree(node *vpNodeBuild, nodes *[]VPNode) int32 {
	if node == nil {
		return -1
	}
	idx := int32(len(*nodes))
	*nodes = append(*nodes, VPNode{
		Index:     int32(node.index),
		Threshold: node.threshold,
		Inside:    -1,
		Outside:   -1,
	})
	if node.inside != nil {
		(*nodes)[idx].Inside = flattenVPTree(node.inside, nodes)
	}
	if node.outside != nil {
		(*nodes)[idx].Outside = flattenVPTree(node.outside, nodes)
	}
	return idx
}

func main() {
	bin()
}

func bin() {
	// 1. Le o JSON de referencias.
	content, err := os.ReadFile("references.json")
	if err != nil {
		fmt.Println("Erro ao ler JSON:", err)
		return
	}

	var registrosJSON []RegistroJSON
	json.Unmarshal(content, &registrosJSON)
	fmt.Printf("Carregados %d registros do JSON\n", len(registrosJSON))

	// Converte para o tipo Point.
	points := make([]Point, len(registrosJSON))
	for i, r := range registrosJSON {
		for j := 0; j < 14; j++ {
			points[i].Vector[j] = r.Vector[j]
		}
		if r.Label == "legit" {
			points[i].Label = 1
		} else {
			points[i].Label = 0
		}
	}
	fmt.Printf("Convertidos %d registros para formato interno\n", len(points))

	// 2. Constroi a VP-Tree (offline — performance nao eh critica, corretude sim).
	fmt.Println("[VP-Tree] Construindo indice...")
	rand.Seed(42)
	start := time.Now()

	indices := make([]int, len(points))
	for i := range indices {
		indices[i] = i
	}

	root := buildVPTree(points, indices)

	var nodes []VPNode
	flattenVPTree(root, &nodes)

	fmt.Printf("[VP-Tree] Construido com %d nos em %v\n", len(nodes), time.Since(start))

	// 3. Escreve o arquivo binario no formato: [n][regs][node_count][nodes]
	fmt.Println("[VP-Tree] Escrevendo dataset.bin...")
	binFile, err := os.Create("dataset.bin")
	if err != nil {
		fmt.Println("Erro ao criar binario:", err)
		return
	}
	defer binFile.Close()

	n := uint32(len(points))
	binary.Write(binFile, binary.LittleEndian, n)

	// Registros flat: 14 float32 + 1 uint32 = 60 bytes cada
	for _, p := range points {
		for j := 0; j < 14; j++ {
			binary.Write(binFile, binary.LittleEndian, p.Vector[j])
		}
		binary.Write(binFile, binary.LittleEndian, p.Label)
	}

	// VP-Tree: node_count + nodes (16 bytes cada)
	nodeCount := uint32(len(nodes))
	binary.Write(binFile, binary.LittleEndian, nodeCount)
	for _, node := range nodes {
		binary.Write(binFile, binary.LittleEndian, node.Index)
		binary.Write(binFile, binary.LittleEndian, node.Threshold)
		binary.Write(binFile, binary.LittleEndian, node.Inside)
		binary.Write(binFile, binary.LittleEndian, node.Outside)
	}

	binFile.Sync()
	fmt.Printf("[VP-Tree] dataset.bin gerado! (n=%d, nodes=%d, total=%d bytes)\n",
		n, nodeCount, 4+int(n)*60+4+int(nodeCount)*16)
}
