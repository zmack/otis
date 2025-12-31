package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zmack/otis/aggregator"
	"github.com/zmack/otis/collector"
	"github.com/zmack/otis/config"
)

func main() {
	cfg := config.Load()

	// Start OTLP collector
	collectorServer, err := collector.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create collector server: %v", err)
	}

	go func() {
		if err := collectorServer.Start(); err != nil {
			log.Fatalf("Failed to start collector server: %v", err)
		}
	}()

	// Start aggregator if enabled
	var aggStore *aggregator.Store
	var aggEngine *aggregator.Engine
	var aggProcessor *aggregator.Processor
	var aggAPI *aggregator.APIServer

	if cfg.AggregatorEnabled {
		log.Println("Starting aggregator...")

		// Initialize store
		aggStore, err = aggregator.NewStore(cfg.DBPath)
		if err != nil {
			log.Fatalf("Failed to create aggregator store: %v", err)
		}

		// Initialize engine
		aggEngine = aggregator.NewEngine(aggStore)

		// Initialize processor
		aggProcessor = aggregator.NewProcessor(cfg.OutputDir, aggStore, aggEngine, cfg.ProcessingInterval)
		aggProcessor.Start()

		// Initialize API server
		aggAPI = aggregator.NewAPIServer(cfg.AggregatorPort, aggStore, aggEngine)
		go func() {
			if err := aggAPI.Start(); err != nil {
				log.Fatalf("Failed to start aggregator API: %v", err)
			}
		}()
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Shutting down services...")

	// Shutdown collector
	if err := collectorServer.Shutdown(ctx); err != nil {
		log.Printf("Collector shutdown error: %v", err)
	}

	// Shutdown aggregator components
	if cfg.AggregatorEnabled {
		if aggProcessor != nil {
			aggProcessor.Stop()
		}

		if aggEngine != nil {
			aggEngine.FlushCache()
		}

		if aggAPI != nil {
			if err := aggAPI.Shutdown(ctx); err != nil {
				log.Printf("Aggregator API shutdown error: %v", err)
			}
		}

		if aggStore != nil {
			if err := aggStore.Close(); err != nil {
				log.Printf("Store close error: %v", err)
			}
		}
	}

	log.Println("All services stopped gracefully")
}
