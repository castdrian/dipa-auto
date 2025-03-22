package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
)

func main() {
	log.Println("Starting dipa-auto...")

	// Load configuration
	cfg, err := LoadConfig("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create checker
	dipaChecker, err := NewChecker(cfg)
	if err != nil {
		log.Fatalf("Failed to create checker: %v", err)
	}

	// Set up cron scheduler
	c := cron.New()
	
	// Define the check function without referencing entryID yet
	checkFunc := func() {
		log.Println("Starting scheduled check...")
		
		// Check both branches
		if err := dipaChecker.CheckBranch("stable"); err != nil {
			log.Printf("Error checking stable branch: %v", err)
		}
		
		// Add a small delay between checks
		time.Sleep(5 * time.Second)
		
		if err := dipaChecker.CheckBranch("testflight"); err != nil {
			log.Printf("Error checking testflight branch: %v", err)
		}
		
		// Log next scheduled run
		entries := c.Entries()
		if len(entries) > 0 {
			nextRun := entries[0].Next
			log.Printf("Check complete. Next run scheduled at: %s", nextRun.Format(time.RFC1123))
		}
	}
	
	// Add the function to the scheduler
	entryID, err := c.AddFunc(cfg.RefreshSchedule, checkFunc)
	if err != nil {
		log.Fatalf("Failed to schedule cron job: %v", err)
	}
	
	// Start the scheduler
	c.Start()
	
	// Display the next scheduled run
	entry := c.Entry(entryID)
	nextRun := entry.Next
	log.Printf("Scheduler started with cron expression: %s", cfg.RefreshSchedule)
	log.Printf("Next check scheduled at: %s", nextRun.Format(time.RFC1123))

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	<-sigCh
	log.Println("Shutdown signal received, stopping scheduler...")
	c.Stop()
	log.Println("dipa-auto stopped")
}
