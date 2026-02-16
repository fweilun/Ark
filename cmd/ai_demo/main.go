package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"ark/internal/ai"
	"ark/internal/maps"
	"ark/internal/service"
)

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable not set")
	}

	ctx := context.Background()
	provider, err := ai.NewGeminiProvider(ctx, apiKey)
	if err != nil {
		log.Fatalf("Failed to initialize AI provider: %v", err)
	}
	defer provider.Close()

	// Simulated context

	// Initialize Maps Service
	mapsApiKey := os.Getenv("GOOGLE_MAPS_API_KEY")
	if mapsApiKey == "" {
		log.Fatal("GOOGLE_MAPS_API_KEY environment variable not set")
	}
	routeService, err := maps.NewRouteService(mapsApiKey)
	if err != nil {
		log.Fatalf("Failed to create route service: %v", err)
	}

	// Initialize Trip Planner
	planner, err := service.NewTripPlanner(provider, routeService)
	if err != nil {
		log.Fatalf("Failed to create trip planner: %v", err)
	}

	// Interactive Loop
	reader := bufio.NewScanner(os.Stdin)
	var history strings.Builder

	// Simulated User Context
	userContextInfo := "Home: No. 1, Sec 1, Yonghe Rd, Yonghe Dist, New Taipei City (永和家); Office: No. 1, Ruiguang Rd, Neihu Dist, Taipei City (內湖公司)"

	fmt.Println("ZooZoo: 您好！請問今天要幫您安排什麼行程？")
	fmt.Print("User: ")

	var lastFailedInput string

	for reader.Scan() {
		// Rate Limiting: Prevent rapid-fire requests
		time.Sleep(1 * time.Second)

		userInput := strings.TrimSpace(reader.Text())
		if userInput == "exit" || userInput == "quit" {
			fmt.Println("ZooZoo: 再見！")
			break
		}

		// Input Recovery Logic
		if userInput == "r" {
			if lastFailedInput != "" {
				userInput = lastFailedInput
				fmt.Printf("ZooZoo: 重試指令: %s\n", userInput)
			} else {
				fmt.Println("ZooZoo: 沒有可重試的指令。")
				fmt.Print("User: ")
				continue
			}
		}

		// Prepare context with history
		fullMessage := userInput
		if history.Len() > 0 {
			fullMessage = fmt.Sprintf("Context: %s\nUser Input: %s", history.String(), userInput)
		}

		// Retry Logic (Exponential Backoff)
		maxRetries := 3
		backoff := 1 * time.Second
		var response string
		var err error

		for i := 0; i < maxRetries; i++ {
			response, err = planner.PlanTrip(ctx, fullMessage, "Taipei Main Station", userContextInfo)
			if err == nil {
				break // Success
			}

			if i < maxRetries-1 {
				fmt.Println("(連線繁忙，正在重試...)")
				time.Sleep(backoff)
				backoff *= 2
			}
		}

		if err != nil {
			lastFailedInput = userInput // Save for retry
			fmt.Printf("ZooZoo: 抱歉，連線持續失敗 (%v)。\n", err)
			fmt.Println("您可以輸入 'r' 重新嘗試發送剛才的內容，或直接輸入新指令。")
			fmt.Print("User: ")
			continue // Do NOT update history on failure
		}

		// Success - Clear failed state
		lastFailedInput = ""
		fmt.Printf("ZooZoo: %s\n", response)

		// Update History only on success
		history.WriteString(fmt.Sprintf("User: %s\nZooZoo: %s\n", userInput, response))

		fmt.Print("User: ")
	}

	if err := reader.Err(); err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
}
