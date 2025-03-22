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
	// Parse flags early
	flag.Parse()

	// Check if we should run in debug mode
	if len(os.Args) > 1 && os.Args[1] == "debug" {
		DebugHashFile()
		return
	}

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
	c := cron.New(cron.WithSeconds())
	
	_, err = c.AddFunc(cfg.RefreshSchedule, func() {
		// Check both branches
		if err := dipaChecker.CheckBranch("stable"); err != nil {
			log.Printf("Error checking stable branch: %v", err)
		}
		
		// Add a small delay between checks
		time.Sleep(5 * time.Second)
		
		if err := dipaChecker.CheckBranch("testflight"); err != nil {
			log.Printf("Error checking testflight branch: %v", err)
		}
	})
	
	if err != nil {
		log.Fatalf("Failed to schedule cron job: %v", err)
	}

	// Start the scheduler
	c.Start()
	log.Printf("Scheduler started with cron expression: %s", cfg.RefreshSchedule)

	// Run an initial check immediately
	go func() {
		log.Println("Performing initial check...")
		if err := dipaChecker.CheckBranch("stable"); err != nil {
			log.Printf("Error during initial stable check: %v", err)
		}
		
		time.Sleep(5 * time.Second)
		
		if err := dipaChecker.CheckBranch("testflight"); err != nil {
			log.Printf("Error during initial testflight check: %v", err)
		}
	}()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for termination signal
	<-sigCh
	log.Println("Shutdown signal received, stopping scheduler...")
	c.Stop()
	log.Println("dipa-auto stopped")
}
