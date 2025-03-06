package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// Different Ollama API response formats
type OllamaGenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type OllamaCompletionResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// ChunkResult stores the result of a processed chunk
type ChunkResult struct {
	Index   int
	Content string
	Error   error
}

const (
	// Maximum chunk size (in bytes) for processing
	chunkSize = 4000
	// Maximum concurrent requests to Ollama
	maxConcurrentRequests = 3
	// Timeout for HTTP requests (in seconds)
	requestTimeout = 120
	// Max retries for failed requests
	maxRetries = 3
	// Delay between retries (in seconds)
	retryDelay = 2
)

func main() {
	// Define command line flags
	inputFile := flag.String("input", "", "Path to the input file (required)")
	modelName := flag.String("model", "llama3.2-vision:latest", "Ollama model name to use")
	outputFile := flag.String("output", "llm.txt", "Output file name")
	apiEndpoint := flag.String("api", "generate", "Ollama API endpoint: 'generate' or 'chat'")
	ollamaBaseURL := flag.String("url", "http://localhost:11434/api", "Ollama API base URL")
	flag.Parse()

	// Check if input file was provided
	if *inputFile == "" {
		fmt.Println("Error: Input file is required")
		flag.Usage()
		os.Exit(1)
	}

	// Validate API endpoint
	if *apiEndpoint != "generate" && *apiEndpoint != "chat" {
		fmt.Println("Error: API endpoint must be 'generate' or 'chat'")
		flag.Usage()
		os.Exit(1)
	}

	// Get file info
	fileInfo, err := os.Stat(*inputFile)
	if err != nil {
		log.Fatalf("Error getting file info: %v", err)
	}
	fileSize := fileInfo.Size()

	fmt.Printf("Processing file: %s (%.2f MB)\n", *inputFile, float64(fileSize)/1024/1024)
	fmt.Printf("Using Ollama model: %s\n", *modelName)
	fmt.Printf("Using API endpoint: %s\n", *apiEndpoint)

	// Construct full API URL
	ollamaURL := fmt.Sprintf("%s/%s", *ollamaBaseURL, *apiEndpoint)

	// Open the input file
	file, err := os.Open(*inputFile)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	// Calculate total chunks
	totalChunks := (int(fileSize) + chunkSize - 1) / chunkSize
	fmt.Printf("Splitting file into %d chunks\n", totalChunks)

	// Create a progress bar for reading and chunking
	bar := progressbar.NewOptions(totalChunks,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(50),
		progressbar.OptionSetDescription("[cyan]Chunking file[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	// Split file into chunks
	chunks := make([]string, 0, totalChunks)
	reader := bufio.NewReader(file)
	for {
		chunk := make([]byte, chunkSize)
		n, err := reader.Read(chunk)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error reading chunk: %v", err)
		}

		// Only include what was actually read
		chunks = append(chunks, string(chunk[:n]))
		bar.Add(1)
	}

	// Create a second progress bar for processing chunks
	processBar := progressbar.NewOptions(len(chunks),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(50),
		progressbar.OptionSetDescription("[cyan]Processing with Ollama[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	// Create a client with timeout
	client := &http.Client{
		Timeout: time.Duration(requestTimeout) * time.Second,
	}

	// Set up concurrency control
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentRequests)
	results := make(chan ChunkResult, len(chunks))

	// Process chunks concurrently
	for i, chunk := range chunks {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(index int, content string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			var result string
			var processErr error

			// Try processing with retries
			for attempt := 0; attempt < maxRetries; attempt++ {
				// If this is a retry, wait before trying again
				if attempt > 0 {
					time.Sleep(time.Duration(retryDelay) * time.Second)
				}

				result, processErr = processChunk(client, ollamaURL, *modelName, content, *apiEndpoint)
				if processErr == nil {
					break
				}

				log.Printf("Attempt %d: Error processing chunk %d: %v", attempt+1, index, processErr)
			}

			if processErr != nil {
				log.Printf("All attempts failed for chunk %d: %v", index, processErr)
				results <- ChunkResult{Index: index, Content: content, Error: processErr} // Use original on error
			} else {
				results <- ChunkResult{Index: index, Content: result, Error: nil}
			}

			processBar.Add(1)
		}(i, chunk)
	}

	// Close results channel when all goroutines are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	processedChunks := make([]string, len(chunks))
	errorCount := 0

	for result := range results {
		if result.Error != nil {
			errorCount++
			// Use original content on error
			processedChunks[result.Index] = result.Content
		} else {
			processedChunks[result.Index] = result.Content
		}
	}

	// Combine processed chunks
	combinedContent := strings.Join(processedChunks, "\n\n")

	// Ensure output directory exists
	outputDir := filepath.Dir(*outputFile)
	if outputDir != "." {
		err = os.MkdirAll(outputDir, 0755)
		if err != nil {
			log.Fatalf("Error creating output directory: %v", err)
		}
	}

	// Write the compressed content to the output file
	err = os.WriteFile(*outputFile, []byte(combinedContent), 0644)
	if err != nil {
		log.Fatalf("Error writing to output file: %v", err)
	}

	fmt.Printf("\nCompression complete: %d of %d chunks processed successfully (%d errors)\n",
		len(chunks)-errorCount, len(chunks), errorCount)
	fmt.Printf("Output saved to %s\n", *outputFile)

	// Calculate compression ratio
	outputInfo, err := os.Stat(*outputFile)
	if err == nil {
		compressionRatio := float64(fileSize) / float64(outputInfo.Size())
		fmt.Printf("Compression ratio: %.2fx (from %.2f MB to %.2f MB)\n",
			compressionRatio,
			float64(fileSize)/1024/1024,
			float64(outputInfo.Size())/1024/1024)
	}
}

// processChunk handles processing a single chunk with the appropriate API
func processChunk(client *http.Client, ollamaURL, modelName, content, apiEndpoint string) (string, error) {
	var requestBody []byte
	var err error

	// Create request based on API endpoint
	if apiEndpoint == "generate" {
		// For generate API
		generateRequest := struct {
			Model  string `json:"model"`
			Prompt string `json:"prompt"`
		}{
			Model:  modelName,
			Prompt: fmt.Sprintf("Compress this text fragment without losing important information: %s", content),
		}
		requestBody, err = json.Marshal(generateRequest)
	} else {
		// For chat API
		chatRequest := struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}{
			Model: modelName,
			Messages: []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				{
					Role:    "user",
					Content: fmt.Sprintf("Compress this text fragment without losing important information: %s", content),
				},
			},
		}
		requestBody, err = json.Marshal(chatRequest)
	}

	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Send request to Ollama
	resp, err := client.Post(ollamaURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("error calling Ollama API: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	// Process response based on API endpoint
	if apiEndpoint == "generate" {
		// Handle streaming response for generate endpoint
		var result strings.Builder
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			var generateResp OllamaGenerateResponse
			if err := json.Unmarshal(line, &generateResp); err != nil {
				return "", fmt.Errorf("error parsing streaming response line: %v", err)
			}
			result.WriteString(generateResp.Response)
			if generateResp.Done {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("error reading streaming response: %v", err)
		}
		return result.String(), nil
	} else {
		// Handle single response for chat endpoint
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error reading response: %v", err)
		}

		var chatResp OllamaCompletionResponse
		if err := json.Unmarshal(body, &chatResp); err == nil && chatResp.Message.Content != "" {
			return chatResp.Message.Content, nil
		}

		// Fallback parsing for unexpected format
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("error parsing chat response: %v", err)
		}
		if message, ok := result["message"].(map[string]interface{}); ok {
			if content, ok := message["content"].(string); ok {
				return content, nil
			}
		}
		return "", fmt.Errorf("could not extract content from chat API response: %s", string(body))
	}
}
