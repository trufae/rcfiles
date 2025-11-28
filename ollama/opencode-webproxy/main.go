package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const opencodeURL = "http://127.0.0.1:55124"

type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []OpenAIChatMessage `json:"messages"`
	Stream   bool                `json:"stream,omitempty"`
}

type OpenAIChatChoice struct {
	Index        int               `json:"index"`
	Message      OpenAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type OpenAIChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []OpenAIChatChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type OpencodeProvider struct {
	ID     string                   `json:"id"`
	Name   string                   `json:"name"`
	Models map[string]OpencodeModel `json:"models"`
}

type OpencodeModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type OpencodeProvidersResponse struct {
	Providers map[string]OpencodeProvider `json:"providers"`
	Default   map[string]string           `json:"default"`
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.status = code
	l.ResponseWriter.WriteHeader(code)
}

type loggingMux struct {
	*http.ServeMux
}

func (l *loggingMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read body for logging
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	// Build curl command
	var curl strings.Builder
	curl.WriteString("curl -X ")
	curl.WriteString(r.Method)
	curl.WriteString(" http://localhost:8080")
	curl.WriteString(r.URL.String())
	for k, v := range r.Header {
		curl.WriteString(" -H \"")
		curl.WriteString(k)
		curl.WriteString(": ")
		curl.WriteString(strings.Join(v, ","))
		curl.WriteString("\"")
	}
	if len(body) > 0 {
		curl.WriteString(" -d '")
		curl.WriteString(string(body))
		curl.WriteString("'")
	}
	log.Printf("Request: %s", curl.String())

	lw := &loggingResponseWriter{ResponseWriter: w}
	l.ServeMux.ServeHTTP(lw, r)

	// Log all responses with method, URL, and status code
	if lw.status == 0 {
		lw.status = 200 // default status if WriteHeader wasn't called
	}
	log.Printf("Response: method=%s url=%s status=%d", r.Method, r.URL.Path, lw.status)
}

func main() {
	mux := &loggingMux{http.NewServeMux()}
	mux.HandleFunc("/v1/models", handleModels)
	mux.HandleFunc("/v1/chat/completions", handleChatCompletions)

	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func handleModels(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received models request: method=%s, url=%s", r.Method, r.URL.Path)
	if r.Method != http.MethodGet {
		log.Printf("Invalid method for models: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Query opencode /config/providers
	log.Printf("Querying opencode /config/providers")
	resp, err := http.Get(opencodeURL + "/config/providers")
	if err != nil {
		log.Printf("Failed to query providers: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	log.Printf("Providers response status: %s", resp.Status)

	var providersResp struct {
		Providers []OpencodeProvider `json:"providers"`
		Default   map[string]string  `json:"default"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&providersResp); err != nil {
		log.Printf("Failed to decode providers response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Decoded providers: %d providers", len(providersResp.Providers))

	var models []OpenAIModel
	for _, provider := range providersResp.Providers {
		for modelID := range provider.Models {
			models = append(models, OpenAIModel{
				ID:      provider.ID + "/" + modelID,
				Object:  "model",
				Created: 1677649963, // dummy
				OwnedBy: provider.Name,
			})
		}
	}

	response := OpenAIModelsResponse{
		Object: "list",
		Data:   models,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received chat completion request: method=%s, url=%s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OpenAIChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode request body: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("Parsed request: model=%s, messages=%d, stream=%v", req.Model, len(req.Messages), req.Stream)

	// For now, assume single user message
	if len(req.Messages) == 0 {
		http.Error(w, "No messages", http.StatusBadRequest)
		return
	}

	// Parse model
	var providerID, modelID string
	if parts := strings.Split(req.Model, "/"); len(parts) == 2 {
		providerID = parts[0]
		modelID = parts[1]
	} else {
		log.Printf("Invalid model format: %s", req.Model)
		http.Error(w, "Invalid model format", http.StatusBadRequest)
		return
	}
	log.Printf("Parsed model: providerID=%s, modelID=%s", providerID, modelID)

	// Create session
	log.Printf("Creating session with opencode")
	sessionResp, err := http.Post(opencodeURL+"/session", "application/json", strings.NewReader("{}"))
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer sessionResp.Body.Close()

	var session struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(sessionResp.Body).Decode(&session); err != nil {
		log.Printf("Failed to decode session response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Created session: %s", session.ID)

	// Prepare prompt
	var system string
	var prompt strings.Builder
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			system = msg.Content
		} else {
			prompt.WriteString(msg.Content + "\n")
		}
	}

	// Send prompt
	promptBody := map[string]interface{}{
		"model": map[string]string{
			"providerID": providerID,
			"modelID":    modelID,
		},
		"parts": []map[string]interface{}{
			{"type": "text", "text": prompt.String()},
		},
	}
	if system != "" {
		promptBody["system"] = system
	}
	promptJSON, _ := json.Marshal(promptBody)
	log.Printf("Sending prompt to session %s: %s", session.ID, string(promptJSON))
	promptResp, err := http.Post(opencodeURL+"/session/"+session.ID+"/message", "application/json", bytes.NewReader(promptJSON))
	if err != nil {
		log.Printf("Failed to send prompt: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer promptResp.Body.Close()
	log.Printf("Prompt response status: %s", promptResp.Status)

	var promptResult struct {
		Info  map[string]interface{}   `json:"info"`
		Parts []map[string]interface{} `json:"parts"`
	}
	if err := json.NewDecoder(promptResp.Body).Decode(&promptResult); err != nil {
		log.Printf("Failed to decode prompt result: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Received prompt result: info=%v, parts=%d", promptResult.Info, len(promptResult.Parts))

	// Extract text from parts
	var content strings.Builder
	for _, part := range promptResult.Parts {
		if part["type"] == "text" {
			content.WriteString(part["text"].(string))
		}
	}
	log.Printf("Extracted content length: %d", content.Len())

	if req.Stream {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		// Simple streaming: send the content in one chunk
		chunk := map[string]interface{}{
			"id":      "chatcmpl-" + session.ID,
			"object":  "chat.completion.chunk",
			"created": 1677649963,
			"model":   req.Model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]string{
						"content": content.String(),
					},
					"finish_reason": nil,
				},
			},
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		fmt.Fprint(w, "data: [DONE]\n")
	} else {
		response := OpenAIChatResponse{
			ID:      "chatcmpl-" + session.ID,
			Object:  "chat.completion",
			Created: 1677649963,
			Model:   req.Model,
			Choices: []OpenAIChatChoice{
				{
					Index: 0,
					Message: OpenAIChatMessage{
						Role:    "assistant",
						Content: content.String(),
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     len(prompt.String()),
				CompletionTokens: len(content.String()),
				TotalTokens:      len(prompt.String()) + len(content.String()),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
