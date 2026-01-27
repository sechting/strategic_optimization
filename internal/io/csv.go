package io

import (
	"encoding/csv"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

type usageGroup struct {
	gas, cpuWin, cpuUbu int
	contracts           []string
}

func WriteTransferCostsCSV(path string, transferCosts map[string][]string) error {
	groups := make([]usageGroup, 0, len(transferCosts))

	// Build structured slice from map: parse key as numbers
	for key, contracts := range transferCosts {
		parts := strings.Split(key, "|")
		if len(parts) != 3 {
			log.Printf("warning: unexpected key format %q", key)
			continue
		}

		gas, err1 := strconv.Atoi(parts[0])
		cpuWin, err2 := strconv.Atoi(parts[1])
		cpuUbu, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			log.Printf("warning: parse error for key %q", key)
			log.Printf("errors: %v %v %v", err1, err2, err3)
			log.Printf("contracts: %v", contracts[0])
			continue
		}

		groups = append(groups, usageGroup{
			gas:       gas,
			cpuWin:    cpuWin,
			cpuUbu:    cpuUbu,
			contracts: contracts,
		})
	}

	// Sort numerically by gas, then cpuWin, then cpuUbu
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].gas != groups[j].gas {
			return groups[i].gas < groups[j].gas
		}
		if groups[i].cpuWin != groups[j].cpuWin {
			return groups[i].cpuWin < groups[j].cpuWin
		}
		return groups[i].cpuUbu < groups[j].cpuUbu
	})

	// Create CSV file
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	if err := w.Write([]string{"gas", "cpuwin", "cpuubu", "contract"}); err != nil {
		return err
	}

	// Rows
	for _, g := range groups {
		gasStr := strconv.Itoa(g.gas)
		cpuWinStr := strconv.Itoa(g.cpuWin)
		cpuUbuStr := strconv.Itoa(g.cpuUbu)

		for _, contract := range g.contracts {
			record := []string{gasStr, cpuWinStr, cpuUbuStr, contract}
			if err := w.Write(record); err != nil {
				return err
			}
		}
	}
	// Check for any error during flush
	if err := w.Error(); err != nil {
		return err
	}

	return nil
}
