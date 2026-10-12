package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dankedev/tulis-go/config"
	"github.com/dankedev/tulis-go/domain/mcp"
	"github.com/dankedev/tulis-go/domain/media"
	"github.com/dankedev/tulis-go/domain/post"
	"github.com/dankedev/tulis-go/domain/workspace"
	"github.com/dankedev/tulis-go/storage"
	"github.com/google/uuid"
)

func main() {
	apiURL := os.Getenv("TULIS_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	apiURL = strings.TrimRight(apiURL, "/")

	apiKey := os.Getenv("TULIS_API_KEY")
	wsIDStr := os.Getenv("TULIS_WORKSPACE_ID")
	var initialWsID uuid.UUID
	if wsIDStr != "" {
		if u, err := uuid.Parse(wsIDStr); err == nil {
			initialWsID = u
		}
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Try probing HTTP endpoint first
	mcpEndpoint := apiURL + "/mcp"
	httpHealthy := checkHTTPHealth(httpClient, mcpEndpoint)

	var directService mcp.Service
	if !httpHealthy {
		// Attempt direct database fallback if HTTP is offline
		fmt.Fprintf(os.Stderr, "[tulis-mcp] HTTP backend at %s not responding. Attempting in-process database mode...\n", apiURL)
		directService = initDirectService()
		if directService != nil {
			fmt.Fprintf(os.Stderr, "[tulis-mcp] In-process database mode initialized successfully.\n")
		} else {
			fmt.Fprintf(os.Stderr, "[tulis-mcp] In-process database unavailable. Will proxy or retry HTTP on calls.\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "[tulis-mcp] Connected to Tulis HTTP backend at %s\n", mcpEndpoint)
	}

	scanner := bufio.NewScanner(os.Stdin)
	// Support large payloads (up to 10MB for articles/images)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	activeWsID := initialWsID
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var req mcp.JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(os.Stdout, nil, -32700, "Parse error: "+err.Error())
			continue
		}

		// JSON-RPC Notification: MUST NOT respond
		isNotification := req.ID == nil

		// If direct in-process service is active
		if directService != nil {
			resp := directService.HandleRequest(context.Background(), req, activeWsID, uuid.Nil)
			if !isNotification {
				writeJSONRPC(os.Stdout, resp)
			}
			continue
		}

		// Forward via HTTP proxy
		forwardReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, mcpEndpoint, bytes.NewReader(line))
		if err != nil {
			if !isNotification {
				sendError(os.Stdout, req.ID, -32603, "Internal request error: "+err.Error())
			}
			continue
		}

		forwardReq.Header.Set("Content-Type", "application/json")
		forwardReq.Header.Set("Accept", "application/json")
		if apiKey != "" {
			forwardReq.Header.Set("X-API-Key", apiKey)
		}
		if activeWsID != uuid.Nil {
			forwardReq.Header.Set("X-Workspace-ID", activeWsID.String())
		}

		resp, err := httpClient.Do(forwardReq)
		if err != nil {
			// If HTTP failed, try initializing direct in-process service once
			if directService == nil {
				directService = initDirectService()
			}
			if directService != nil {
				rpcResp := directService.HandleRequest(context.Background(), req, activeWsID, uuid.Nil)
				if !isNotification {
					writeJSONRPC(os.Stdout, rpcResp)
				}
				continue
			}

			if !isNotification {
				sendError(os.Stdout, req.ID, -32000, fmt.Sprintf("Failed to connect to Tulis backend at %s: %v", mcpEndpoint, err))
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if isNotification {
			continue
		}

		if err != nil {
			sendError(os.Stdout, req.ID, -32603, "Error reading backend response: "+err.Error())
			continue
		}

		if len(bytes.TrimSpace(body)) > 0 {
			os.Stdout.Write(bytes.TrimSpace(body))
			os.Stdout.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[tulis-mcp] Stdin read error: %v\n", err)
	}
}

func checkHTTPHealth(client *http.Client, endpoint string) bool {
	probeBody := []byte(`{"jsonrpc":"2.0","id":"_probe","method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(probeBody))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func initDirectService() mcp.Service {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[tulis-mcp] Recovered during in-process init: %v\n", r)
		}
	}()

	config.LoadConfig()
	if config.DB == nil {
		config.ConnectDB()
	}
	if config.DB == nil {
		return nil
	}

	wsRepo := workspace.NewWorkspaceRepository(config.DB)
	wsSvc := workspace.NewWorkspaceService(wsRepo)

	postRepo := post.NewPostRepository(config.DB)
	postSvc := post.NewPostService(postRepo, nil)

	mediaRepo := media.NewMediaRepository(config.DB)
	localStorage := storage.NewLocalStorage("./uploads")
	mediaSvc := media.NewMediaService(mediaRepo, localStorage)

	return mcp.NewService(postSvc, wsSvc, mediaSvc)
}

func sendError(w io.Writer, id any, code int, msg string) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcp.RPCError{
			Code:    code,
			Message: msg,
		},
	}
	writeJSONRPC(w, resp)
}

func writeJSONRPC(w io.Writer, val any) {
	data, _ := json.Marshal(val)
	w.Write(data)
	w.Write([]byte("\n"))
}
