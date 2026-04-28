package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
)

type RegistroJSON struct {
	Vector []float32 `json:"vector"`
	Label  string    `json:"label"`
}

func bin() {
	// 1. Abre o JSON
	content, err := os.ReadFile("references.json")
	if err != nil {
		fmt.Println("Erro ao ler JSON:", err)
		return
	}

	var registros []RegistroJSON
	json.Unmarshal(content, &registros)

	// 2. Cria o arquivo binário
	binFile, err := os.Create("dataset.bin")
	if err != nil {
		fmt.Println("Erro ao criar binário:", err)
		return
	}
	
	// GARANTE O FECHAMENTO: O defer garante que o arquivo feche ao fim da função main
	defer binFile.Close()

	for _, r := range registros {
		// Escreve os 14 floats
		for i := 0; i < 14; i++ {
			binary.Write(binFile, binary.LittleEndian, r.Vector[i])
		}
		
		// Escreve o Label (0 ou 1)
		var labelCode uint32 = 0
		if r.Label == "legit" {
			labelCode = 1
		}
		binary.Write(binFile, binary.LittleEndian, labelCode)
	}

	// Opcional: Forçar a gravação física no disco antes de sair
	binFile.Sync() 
	fmt.Println("Conversão concluída e arquivo fechado com sucesso!")
}