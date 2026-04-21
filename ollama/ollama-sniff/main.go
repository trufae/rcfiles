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
	"strings"
	"sync"
	"time"
)

var listenAddr = flag.String("listen", ":11435", "address to listen on")
var targetAddr = flag.String("target", "http://127.0.0.1:11434", "target Ollama server address")
var anthropicAddr = flag.String("anthropic", "https://api.anthropic.com", "target Anthropic server address")
var rawText = flag.Bool("raw-text", false, "show raw text from JSON contents instead of full JSON for responses")
var noLicense = flag.Bool("no-license", false, "skip messages containing 'license' key")

var colors = []string{
	"\033[31m", // red
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[35m", // magenta
	"\033[36m", // cyan
}

var clientColors = make(map[string]string)
var colorIndex = 0
var colorMutex sync.Mutex

func getColor(client string) string {
	colorMutex.Lock()
	defer colorMutex.Unlock()
	if c, ok := clientColors[client]; ok {
		return c
	}
	c := colors[colorIndex%len(colors)]
	colorIndex++
	clientColors[client] = c
	return c
}

func getTargetURL(r *http.Request) string {
	// Check if this is an Anthropic endpoint
	if strings.HasPrefix(r.URL.Path, "/v1/") {
		// Strip the /v1/ prefix and route to Anthropic
		path := strings.TrimPrefix(r.URL.Path, "/v1/")
		targetURL := *anthropicAddr + "/v1/" + path
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
		return targetURL
	} else if strings.HasPrefix(r.URL.Path, "/api/") {
		// Check if it's an Ollama API endpoint or should go to Anthropic
		// Most /api/ endpoints that aren't Ollama-specific should go to Anthropic
		if strings.HasPrefix(r.URL.Path, "/api/tags") ||
		   strings.HasPrefix(r.URL.Path, "/api/pull") ||
		   strings.HasPrefix(r.URL.Path, "/api/push") ||
		   strings.HasPrefix(r.URL.Path, "/api/create") ||
		   strings.HasPrefix(r.URL.Path, "/api/delete") ||
		   strings.HasPrefix(r.URL.Path, "/api/copy") ||
		   strings.HasPrefix(r.URL.Path, "/api/show") {
			// Ollama-specific API endpoints
			targetURL := *targetAddr + r.URL.Path
			if r.URL.RawQuery != "" {
				targetURL += "?" + r.URL.RawQuery
			}
			return targetURL
		} else {
			// Route to Anthropic (like /api/event_logging/batch)
			path := strings.TrimPrefix(r.URL.Path, "/api/")
			targetURL := *anthropicAddr + "/api/" + path
			if r.URL.RawQuery != "" {
				targetURL += "?" + r.URL.RawQuery
			}
			return targetURL
		}
	}

	// Default to Ollama
	targetURL := *targetAddr + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}
	return targetURL
}

func getClientType(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/v1/") {
		return "anthropic"
	} else if strings.HasPrefix(r.URL.Path, "/api/") {
		// Check if it's an Ollama API endpoint or should go to Anthropic
		if strings.HasPrefix(r.URL.Path, "/api/tags") ||
		   strings.HasPrefix(r.URL.Path, "/api/pull") ||
		   strings.HasPrefix(r.URL.Path, "/api/push") ||
		   strings.HasPrefix(r.URL.Path, "/api/create") ||
		   strings.HasPrefix(r.URL.Path, "/api/delete") ||
		   strings.HasPrefix(r.URL.Path, "/api/copy") ||
		   strings.HasPrefix(r.URL.Path, "/api/show") {
			return "ollama"
		} else {
			return "anthropic"
		}
	}
	return "ollama"
}

func main() {
	flag.Parse()

	http.HandleFunc("/", proxyHandler)

	log.Printf("Starting proxy on %s", *listenAddr)
	log.Printf("Ollama endpoints -> %s", *targetAddr)
	log.Printf("Anthropic endpoints -> %s", *anthropicAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	client := r.RemoteAddr
	color := getColor(client)
	reset := "\033[0m"

	// Create target URL
	targetURL := getTargetURL(r)

	// Determine client type for logging
	clientType := getClientType(r)

	// Log HTTP request method and path
	log.Printf("[%s] %s %s (%s)", client, r.Method, r.URL.Path, clientType)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Print request
	if *rawText {
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		if *noLicense {
			if _, hasLicense := req["license"]; hasLicense {
				return
			}
		}
		if messages, ok := req["messages"].([]interface{}); ok {
			fmt.Printf("%s[%s -> server]%s\n", color, client, reset)
			for _, msg := range messages {
				if m, ok := msg.(map[string]interface{}); ok {
					if role, ok := m["role"].(string); ok {
						if content, ok := m["content"].(string); ok {
							fmt.Printf("%s: %s\n", role, content)
						}
					}
				}
			}
		} else {
			fmt.Printf("%s[%s -> server]%s %s\n", color, client, reset, string(body))
		}
	} else {
		fmt.Printf("%s[%s -> server]%s %s\n", color, client, reset, string(body))
	}

	// Parse request to check if stream
	var req map[string]interface{}
	json.Unmarshal(body, &req)
	isStream := false
	if s, ok := req["stream"].(bool); ok {
		isStream = s
	}

	// New request
	req2, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Copy headers
	for k, v := range r.Header {
		req2.Header[k] = v
	}

	// Do request
	startTime := time.Now()
	resp, err := http.DefaultClient.Do(req2)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)

	// Handle body
	if isStream || clientType == "anthropic" {
		// Streaming response
		scanner := bufio.NewScanner(resp.Body)
		totalLen := 0
		for scanner.Scan() {
			line := scanner.Text()
			// Write to client
			fmt.Fprintln(w, line)

			// Parse and print content based on client type
			if clientType == "anthropic" {
				// Handle Anthropic streaming format
				if strings.HasPrefix(line, "data: ") {
					data := line[6:]
					if data == "[DONE]" {
						continue
					}
					var chunk map[string]interface{}
					json.Unmarshal([]byte(data), &chunk)

					// Extract content from Anthropic format
					if delta, ok := chunk["delta"].(map[string]interface{}); ok {
						if text, ok := delta["text"].(string); ok {
							if *rawText {
								fmt.Print(text)
								totalLen += len(text)
							} else {
								fmt.Printf("%s%s%s", color, text, reset)
								totalLen += len(text)
							}
						}
					}
					// Also check for content field
					if content, ok := chunk["content"].(string); ok {
						if *rawText {
							fmt.Print(content)
							totalLen += len(content)
						} else {
							fmt.Printf("%s%s%s", color, content, reset)
							totalLen += len(content)
						}
					}
				}
			} else {
				// Handle Ollama streaming format
				if strings.HasPrefix(line, "data: ") {
					data := line[6:]
					if data == "[DONE]" {
						continue
					}
					var chunk map[string]interface{}
					json.Unmarshal([]byte(data), &chunk)
					if message, ok := chunk["message"].(map[string]interface{}); ok {
						if content, ok := message["content"].(string); ok {
							if *rawText {
								fmt.Print(content)
								totalLen += len(content)
							} else {
								fmt.Printf("%s%s%s", color, content, reset)
								totalLen += len(content)
							}
						}
					}
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading response: %v", err)
		}
		fmt.Println() // Newline after stream
		if *rawText && totalLen > 0 {
			tokens := totalLen / 4
			fmt.Printf("Length: %d chars, Tokens: ~%d\n", totalLen, tokens)
		}
		fmt.Printf("Time: %s\n", time.Since(startTime))
	} else {
		// Non-streaming response
		respBody, err := io.ReadAll(resp.Body)
		duration := time.Since(startTime)
		if err != nil {
			log.Printf("Error reading response: %v", err)
			return
		}
		if *rawText {
			// Extract content based on client type
			var respJSON map[string]interface{}
			json.Unmarshal(respBody, &respJSON)

			if clientType == "anthropic" {
				// Handle Anthropic response format
				if content, ok := respJSON["content"].([]interface{}); ok {
					var fullContent string
					for _, item := range content {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if text, ok := itemMap["text"].(string); ok {
								fullContent += text
							}
						}
					}
					if fullContent != "" {
						fmt.Printf("%s[server -> %s]%s %s\n", color, client, reset, fullContent)
						tokens := len(fullContent) / 4
						fmt.Printf("Length: %d chars, Tokens: ~%d\n", len(fullContent), tokens)
						fmt.Printf("Time: %s\n", duration)
					} else {
						fmt.Printf("%s[server -> %s]%s %s\n", color, client, reset, string(respBody))
						fmt.Printf("Time: %s\n", duration)
					}
				} else {
					fmt.Printf("%s[server -> %s]%s %s\n", color, client, reset, string(respBody))
					fmt.Printf("Time: %s\n", duration)
				}
			} else {
				// Handle Ollama response format
				if message, ok := respJSON["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok {
						fmt.Printf("%s[server -> %s]%s %s\n", color, client, reset, content)
						tokens := len(content) / 4
						fmt.Printf("Length: %d chars, Tokens: ~%d\n", len(content), tokens)
						fmt.Printf("Time: %s\n", duration)
					} else {
						fmt.Printf("%s[server -> %s]%s %s\n", color, client, reset, string(respBody))
						fmt.Printf("Time: %s\n", duration)
					}
				} else {
					fmt.Printf("%s[server -> %s]%s %s\n", color, client, reset, string(respBody))
					fmt.Printf("Time: %s\n", duration)
				}
			}
		} else {
			fmt.Printf("%s[server -> %s]%s %s (Time: %s)\n", color, client, reset, string(respBody), duration)
		}
		w.Write(respBody)
	}
}
