package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// Command line flags for hash debugger
var (
	debugHashFile = flag.String("debug-hash", "/var/lib/dipa-auto/branch_hashes.json", "Path to branch_hashes.json file to debug")
)

// DebugHashFile analyzes a branch_hashes.json file
func DebugHashFile() {
	if len(os.Args) > 1 && os.Args[1] == "debug" {
		fmt.Printf("=== Branch hashes file analysis ===\n")
		fmt.Printf("File: %s\n", *debugHashFile)

		// Check if file exists
		if _, err := os.Stat(*debugHashFile); os.IsNotExist(err) {
			fmt.Printf("Error: File not found: %s\n", *debugHashFile)
			os.Exit(1)
		}

		// Read file
		data, err := os.ReadFile(*debugHashFile)
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			os.Exit(1)
		}

		// Try to parse as new format first
		var branchHashes BranchHashes
		err = json.Unmarshal(data, &branchHashes)
		if err == nil && len(branchHashes.Branches) > 0 {
			// Successfully parsed as new format
			fmt.Printf("\nFormat: NEW (branches map with dispatches)\n")
			for branch, branchData := range branchHashes.Branches {
				fmt.Printf("\n[Branch: %s]\n", branch)
				fmt.Printf("  Hash: %s\n", branchData.Hash)
				fmt.Printf("  Tracked hash count: %d\n", len(branchData.Dispatches))
				
				for hash, repos := range branchData.Dispatches {
					shortHash := hash
					if len(hash) > 8 {
						shortHash = hash[:8] + "..."
					}
					fmt.Printf("    Hash: %s (%d repos)\n", shortHash, len(repos))
					for _, repo := range repos {
						fmt.Printf("      - %s\n", repo)
					}
				}
			}
			os.Exit(0)
		}

		// Try to parse as old format
		var oldFormat map[string]interface{}
		if err := json.Unmarshal(data, &oldFormat); err != nil {
			fmt.Printf("Error: Invalid JSON in file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nFormat: OLD (direct branch mapping)\n")
		for branch, branchData := range oldFormat {
			fmt.Printf("\n[Branch: %s]\n", branch)
			
			switch bd := branchData.(type) {
			case string:
				fmt.Printf("  Format: OLD (string hash)\n")
				fmt.Printf("  Hash: %s\n", bd)
				fmt.Printf("  WARNING: Old format detected, needs migration\n")
			case map[string]interface{}:
				if hash, ok := bd["hash"]; ok {
					fmt.Printf("  Hash: %v\n", hash)
				} else {
					fmt.Printf("  Hash: None (missing)\n")
				}
				
				if dispatched, ok := bd["dispatched"]; ok {
					fmt.Printf("  Format: OLD (flat dispatched list)\n")
					if dispatchedArr, ok := dispatched.([]interface{}); ok {
						fmt.Printf("  Dispatched repos: %d\n", len(dispatchedArr))
						for _, repo := range dispatchedArr {
							fmt.Printf("    - %v\n", repo)
						}
					}
					fmt.Printf("  WARNING: Old format detected, needs migration\n")
				} else if dispatches, ok := bd["dispatches"]; ok {
					fmt.Printf("  Format: INTERMEDIATE (dispatches map)\n")
					if dispatchesMap, ok := dispatches.(map[string]interface{}); ok {
						fmt.Printf("  Tracked hash count: %d\n", len(dispatchesMap))
						for hash, repos := range dispatchesMap {
							shortHash := hash
							if len(hash) > 8 {
								shortHash = hash[:8] + "..."
							}
							if reposArr, ok := repos.([]interface{}); ok {
								fmt.Printf("    Hash: %s (%d repos)\n", shortHash, len(reposArr))
								for _, repo := range reposArr {
									fmt.Printf("      - %v\n", repo)
								}
							}
						}
					}
				}
			default:
				fmt.Printf("  Format: UNKNOWN\n")
				fmt.Printf("  Data: %v\n", branchData)
			}
		}
		os.Exit(0)
	}
}
