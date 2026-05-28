package browsh

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req mcpRequest
	if err := json.Unmarshal(body, &req); err != nil {
		sendMCPError(w, req.ID, -32700, "Parse error")
		return
	}

	switch req.Method {
	case "initialize":
		sendMCPResponse(w, req.ID, map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"serverInfo": map[string]interface{}{
				"name":    "browsh-mcp",
				"version": "1.0.0",
			},
		})
	case "notifications/initialized":
		w.WriteHeader(http.StatusOK)
	case "tools/list":
		sendMCPResponse(w, req.ID, map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "fetch_webpage",
					"description": "Fetch a webpage in plaintext (opens webpage, dumps content, and closes tab immediately)",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"url": map[string]interface{}{"type": "string"},
						},
						"required": []string{"url"},
					},
				},
				{
					"name":        "open_webpage",
					"description": "Open a webpage and fetch it in plaintext (keeps the tab open)",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"url": map[string]interface{}{"type": "string"},
						},
						"required": []string{"url"},
					},
				},
				{
					"name":        "close",
					"description": "Close the current tab or the browser",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "fetch_current_tab",
					"description": "Fetch the current tab's webpage in plaintext",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "list_tabs",
					"description": "List all open tabs",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "switch_tab",
					"description": "Switch to a specific tab by ID",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{"type": "string"},
						},
						"required": []string{"id"},
					},
				},
			},
		})
	case "tools/call":
		var params struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendMCPError(w, req.ID, -32602, "Invalid params")
			return
		}

		if params.Name == "close" {
			if len(tabsOrder) <= 1 {
				sendMessageToWebExtension("/new_tab,about:blank")
				if CurrentTab != nil {
					removeTab(CurrentTab.ID)
				}
				sendMCPResponse(w, req.ID, map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "Closed last tab and opened a new empty tab.",
						},
					},
				})
			} else if CurrentTab != nil {
				removeTab(CurrentTab.ID)
				sendMCPResponse(w, req.ID, map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "Closed current tab",
						},
					},
				})
			} else {
				sendMCPResponse(w, req.ID, map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": "No active tab to close",
						},
					},
				})
			}
			return
		}

		if params.Name == "list_tabs" {
			tabsInfo := ""
			for _, id := range tabsOrder {
				t := Tabs[id]
				activeStr := ""
				if CurrentTab != nil && CurrentTab.ID == id {
					activeStr = " (Active)"
				}
				tabsInfo += fmt.Sprintf("ID: %d%s\nTitle: %s\nURL: %s\n\n", id, activeStr, t.Title, t.URI)
			}
			if tabsInfo == "" {
				tabsInfo = "No open tabs."
			}
			sendMCPResponse(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": tabsInfo,
					},
				},
			})
			return
		}

		if params.Name == "switch_tab" {
			idStr, ok := params.Arguments["id"]
			if !ok || idStr == "" {
				sendMCPError(w, req.ID, -32602, "Missing 'id' argument")
				return
			}
			var id int
			_, err := fmt.Sscanf(idStr, "%d", &id)
			if err != nil || Tabs[id] == nil {
				sendMCPError(w, req.ID, -32602, "Invalid or unknown tab ID")
				return
			}
			sendMessageToWebExtension(fmt.Sprintf("/switch_to_tab,%d", id))
			CurrentTab = Tabs[id]
			sendMCPResponse(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Switched to tab %d", id),
					},
				},
			})
			return
		}

		if params.Name == "fetch_current_tab" {
			if CurrentTab == nil {
				sendMCPError(w, req.ID, -32602, "No active tab")
				return
			}
			text, err := fetchCurrentTabRawText()
			if err != nil {
				sendMCPResponse(w, req.ID, map[string]interface{}{
					"isError": true,
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": fmt.Sprintf("Error fetching current tab: %v", err),
						},
					},
				})
				return
			}
			sendMCPResponse(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": text,
					},
				},
			})
			return
		}

		if params.Name != "fetch_webpage" && params.Name != "open_webpage" {
			sendMCPError(w, req.ID, -32601, "Tool not found")
			return
		}

		urlToFetch, ok := params.Arguments["url"]
		if !ok || urlToFetch == "" {
			sendMCPError(w, req.ID, -32602, "Missing 'url' argument")
			return
		}

		text, err := fetchWebpageRawText(urlToFetch, params.Name == "open_webpage")
		if err != nil {
			sendMCPResponse(w, req.ID, map[string]interface{}{
				"isError": true,
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Error fetching webpage: %v", err),
					},
				},
			})
			return
		}

		sendMCPResponse(w, req.ID, map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": text,
				},
			},
		})
	default:
		sendMCPError(w, req.ID, -32601, "Method not found")
	}
}

func sendMCPResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	resp := mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	json.NewEncoder(w).Encode(resp)
}

func sendMCPError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	resp := mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

func fetchCurrentTabRawText() (string, error) {
	rawTextRequestID := pseudoUUID()
	start := time.Now().Format(time.RFC3339)
	rawTextRequests.store(rawTextRequestID+"-start", start)

	if err := waitForWebExtensionConnection(15 * time.Second); err != nil {
		return "", err
	}

	mode := "PLAIN"
	sendMessageToWebExtension(
		"/raw_text_current_tab," + rawTextRequestID + "," +
			mode)

	var rawTextRequestResponse string
	var ok bool
	maxTime := time.Duration(30) * time.Second
	startTime := time.Now()
	for time.Since(startTime) < maxTime {
		if rawTextRequestResponse, ok = rawTextRequests.load(rawTextRequestID); ok {
			jsonResponse := unpackResponse(rawTextRequestResponse)
			rawTextRequests.remove(rawTextRequestID)
			return jsonResponse.Text, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	rawTextRequests.remove(rawTextRequestID)
	return "", fmt.Errorf("timeout waiting for page render")
}

func fetchWebpageRawText(urlForBrowsh string, keepOpen bool) (string, error) {
	rawTextRequestID := pseudoUUID()
	start := time.Now().Format(time.RFC3339)
	rawTextRequests.store(rawTextRequestID+"-start", start)

	if err := waitForWebExtensionConnection(15 * time.Second); err != nil {
		return "", err
	}

	mode := "PLAIN"
	keepOpenArg := "false"
	if keepOpen {
		keepOpenArg = "true"
	}
	sendMessageToWebExtension(
		"/raw_text_request," + rawTextRequestID + "," +
			mode + "," + keepOpenArg + "," +
			urlForBrowsh)

	var rawTextRequestResponse string
	var ok bool
	maxTime := time.Duration(30) * time.Second
	startTime := time.Now()
	for time.Since(startTime) < maxTime {
		if rawTextRequestResponse, ok = rawTextRequests.load(rawTextRequestID); ok {
			jsonResponse := unpackResponse(rawTextRequestResponse)
			rawTextRequests.remove(rawTextRequestID)
			return jsonResponse.Text, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	rawTextRequests.remove(rawTextRequestID)
	return "", fmt.Errorf("timeout waiting for page render")
}
