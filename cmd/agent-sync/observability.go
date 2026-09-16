package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type hookErrorEvent struct {
	Code  string `json:"code"`
	Stage string `json:"stage"`
}

func printHookObservability() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	path := filepath.Join(cacheDir, "agent-sync", "hooks", "errors.jsonl")
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		fmt.Println("Nenhum erro de hook persistido.")
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	counts := map[string]int{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event hookErrorEvent
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			counts[event.Stage+":"+event.Code]++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Printf("📊 Erros de hooks (%s)\n", path)
	for _, key := range keys {
		fmt.Printf(" - %-35s %d\n", key, counts[key])
	}
	if len(keys) == 0 {
		fmt.Println("Nenhum evento válido encontrado.")
	}
	return nil
}
