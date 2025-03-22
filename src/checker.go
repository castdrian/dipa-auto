package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// IPAFile represents an IPA file in the directory listing
type IPAFile struct {
	Name    string    `json:"name"`
	ModTime time.Time `json:"mod_time"`
}

// BranchHashes represents the hash data for each branch
type BranchHashes struct {
	// Map of branch names to branch data
	Branches map[string]BranchData `json:"branches"`
}

// BranchData represents the hash and dispatch data for a branch
type BranchData struct {
	Hash      string              `json:"hash"`
	Dispatches map[string][]string `json:"dispatches"`
}

// DipaChecker is the main checker for IPA updates
type DipaChecker struct {
	Config     *Config
	HashFile   string
	BranchData BranchHashes
	Client     *http.Client
}

// NewChecker creates a new DipaChecker
func NewChecker(cfg *Config) (*DipaChecker, error) {
	hashDir := "/var/lib/dipa-auto"
	if err := os.MkdirAll(hashDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create hash directory: %w", err)
	}

	checker := &DipaChecker{
		Config:   cfg,
		HashFile: filepath.Join(hashDir, "branch_hashes.json"),
		Client:   &http.Client{Timeout: 30 * time.Second},
		BranchData: BranchHashes{
			Branches: make(map[string]BranchData),
		},
	}

	// Initialize the hash file (either load it or create it)
	if err := checker.InitHashFile(); err != nil {
		return nil, fmt.Errorf("failed to initialize hash file: %w", err)
	}

	return checker, nil
}

// InitHashFile initializes the hash file - either loads existing one or creates new
func (c *DipaChecker) InitHashFile() error {
	// Check if the file exists
	if _, err := os.Stat(c.HashFile); os.IsNotExist(err) {
		// File doesn't exist, create a new one
		log.Printf("Hash file not found, creating new one at %s", c.HashFile)
		c.BranchData.Branches["stable"] = BranchData{
			Hash:      "",
			Dispatches: make(map[string][]string),
		}
		c.BranchData.Branches["testflight"] = BranchData{
			Hash:      "",
			Dispatches: make(map[string][]string),
		}
		
		return c.SaveHashes()
	}
	
	// File exists, load it
	err := c.LoadHashes()
	if err != nil {
		return fmt.Errorf("failed to load hash file: %w", err)
	}
	
	// Make sure both branches exist
	if _, ok := c.BranchData.Branches["stable"]; !ok {
		c.BranchData.Branches["stable"] = BranchData{
			Hash:      "",
			Dispatches: make(map[string][]string),
		}
	}
	
	if _, ok := c.BranchData.Branches["testflight"]; !ok {
		c.BranchData.Branches["testflight"] = BranchData{
			Hash:      "",
			Dispatches: make(map[string][]string),
		}
	}
	
	return nil
}

// LoadHashes loads the branch hashes from the hash file
func (c *DipaChecker) LoadHashes() error {
	file, err := os.Open(c.HashFile)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(&c.BranchData)
}

// SaveHashes saves the branch hashes to the hash file
func (c *DipaChecker) SaveHashes() error {
	file, err := os.Create(c.HashFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(&c.BranchData)
}

// FetchIPAList fetches the IPA list for a branch and calculates its hash
func (c *DipaChecker) FetchIPAList(branch string) ([]IPAFile, string, error) {
	url := fmt.Sprintf("%s/%s/", c.Config.IPABaseURL, branch)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	
	var files []IPAFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, "", err
	}
	
	// Sort the data to ensure consistent hashing
	sortedData, err := sortAndMarshal(files)
	if err != nil {
		return nil, "", err
	}
	
	// Calculate hash
	hasher := sha256.New()
	hasher.Write(sortedData)
	hash := hex.EncodeToString(hasher.Sum(nil))
	
	return files, hash, nil
}

// sortAndMarshal sorts the IPA files and marshals them to JSON
func sortAndMarshal(files []IPAFile) ([]byte, error) {
	// Sort files by name for consistent hashing
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	
	return json.Marshal(files)
}

// GetLatestVersion returns the latest version from the IPA list
func (c *DipaChecker) GetLatestVersion(files []IPAFile) *IPAFile {
	if len(files) == 0 {
		return nil
	}
	
	latest := files[0]
	for _, file := range files[1:] {
		if file.ModTime.After(latest.ModTime) {
			latest = file
		}
	}
	
	return &latest
}

// DispatchGitHubWorkflow dispatches a GitHub workflow for an IPA update
func (c *DipaChecker) DispatchGitHubWorkflow(ipaURL, branch, currentHash string) ([]string, []string, error) {
	successfulDispatches := []string{}
	failedDispatches := []string{}
	
	// Get branch data
	branchData, ok := c.BranchData.Branches[branch]
	if !ok {
		branchData = BranchData{
			Hash:      "",
			Dispatches: make(map[string][]string),
		}
	}
	
	// Get dispatches for current hash
	dispatches, ok := branchData.Dispatches[currentHash]
	if !ok {
		dispatches = []string{}
	}
	
	for _, target := range c.Config.Targets {
		repo := target.GitHubRepo
		
		// Skip if already successfully dispatched for this hash
		alreadyDispatched := false
		for _, dispatched := range dispatches {
			if dispatched == repo {
				alreadyDispatched = true
				break
			}
		}
		
		if alreadyDispatched {
			log.Printf("Skipping %s for %s - already dispatched for current version", repo, branch)
			successfulDispatches = append(successfulDispatches, repo)
			continue
		}
		
		log.Printf("Dispatching workflow for %s update %s to %s", branch, ipaURL, repo)
		
		// Prepare dispatch payload
		payload := map[string]interface{}{
			"event_type": "ipa-update",
			"client_payload": map[string]interface{}{
				"ipa_url":      ipaURL,
				"is_testflight": branch == "testflight",
			},
		}
		
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error marshaling payload for %s: %v", repo, err)
			failedDispatches = append(failedDispatches, repo)
			continue
		}
		
		// Create request
		url := fmt.Sprintf("https://api.github.com/repos/%s/dispatches", repo)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			log.Printf("Error creating request for %s: %v", repo, err)
			failedDispatches = append(failedDispatches, repo)
			continue
		}
		
		// Set headers
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", target.GitHubToken))
		req.Header.Set("Content-Type", "application/json")
		
		// Send request
		resp, err := c.Client.Do(req)
		if err != nil {
			log.Printf("Error sending request to %s: %v", repo, err)
			failedDispatches = append(failedDispatches, repo)
			continue
		}
		
		// Check response
		if resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("Failed to dispatch %s workflow to %s: Status %d, Details: %s", 
				branch, repo, resp.StatusCode, trimString(string(body), 200))
			failedDispatches = append(failedDispatches, repo)
		} else {
			log.Printf("Successfully dispatched %s workflow to %s", branch, repo)
			successfulDispatches = append(successfulDispatches, repo)
		}
		
		resp.Body.Close()
	}
	
	return successfulDispatches, failedDispatches, nil
}

// trimString trims a string to the specified length
func trimString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// CheckBranch checks a branch for updates
func (c *DipaChecker) CheckBranch(branch string) error {
	log.Printf("Checking %s branch...", branch)
	
	files, currentHash, err := c.FetchIPAList(branch)
	if err != nil {
		return fmt.Errorf("error fetching IPA list: %w", err)
	}
	
	// Ensure branch exists in hash structure
	branchData, ok := c.BranchData.Branches[branch]
	if !ok {
		branchData = BranchData{
			Hash:      "",
			Dispatches: make(map[string][]string),
		}
	}
	
	storedHash := branchData.Hash
	
	if currentHash != storedHash {
		latestVersion := c.GetLatestVersion(files)
		if latestVersion != nil {
			finalURL := fmt.Sprintf("%s/%s/%s", c.Config.IPABaseURL, branch, latestVersion.Name)
			log.Printf("New version found in %s: %s", branch, finalURL)
			
			successful, failed, err := c.DispatchGitHubWorkflow(finalURL, branch, currentHash)
			if err != nil {
				return fmt.Errorf("error dispatching workflow: %w", err)
			}
			
			// Update hash and dispatched repositories if there are successful dispatches
			if len(successful) > 0 {
				branchData.Hash = currentHash
				
				// Initialize dispatches map if needed
				if branchData.Dispatches == nil {
					branchData.Dispatches = make(map[string][]string)
				}
				
				// Initialize array for current hash if needed
				if _, ok := branchData.Dispatches[currentHash]; !ok {
					branchData.Dispatches[currentHash] = []string{}
				}
				
				// Add successful dispatches to the list
				existingDispatches := branchData.Dispatches[currentHash]
				for _, repo := range successful {
					// Check if repo is already in the list
					found := false
					for _, existing := range existingDispatches {
						if existing == repo {
							found = true
							break
						}
					}
					
					if !found {
						existingDispatches = append(existingDispatches, repo)
					}
				}
				
				branchData.Dispatches[currentHash] = existingDispatches
				c.BranchData.Branches[branch] = branchData
				
				if err := c.SaveHashes(); err != nil {
					return fmt.Errorf("error saving hashes: %w", err)
				}
				
				log.Printf("Updated hash for %s and tracked %d successful dispatches", 
					branch, len(successful))
			}
			
			if len(failed) > 0 {
				log.Printf("Failed to dispatch %s to %d repositories: %v", 
					branch, len(failed), failed)
			}
		}
	} else {
		log.Printf("No changes detected in %s", branch)
	}
	
	return nil
}
