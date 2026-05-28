package browsh

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

var mcpTaskMutex sync.Mutex

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

var (
	mcpCachedText    string
	mcpActionRequests = newRequestsMap()
)

const mcpKeyboardInputHelp = "Supported keys: Enter, Escape, Space, Tab, Backspace, Delete, Insert, Up, Down, Left, Right, Home, End, PgUp, PgDn, F1-F24, and any single printable character. Modifiers: Shift+, Ctrl+, Alt+ (example: Shift+Ctrl+Alt+A)."

type mcpToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type mcpKeyboardInputArgs struct {
	Text string   `json:"text"`
	Keys []string `json:"keys"`
}

type mcpSearchOnPageArgs struct {
	Text string `json:"text"`
}

type mcpMouseCoordinatesInput struct {
	X *int `json:"x"`
	Y *int `json:"y"`
}

type mcpMouseCoordinates struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type mcpMouseArgs struct {
	Action      string                    `json:"action"`
	Coordinates *mcpMouseCoordinatesInput `json:"coordinates"`
	Text        string                    `json:"text"`
}

type mcpMouseActionResult struct {
	CursorText string `json:"cursor_text,omitempty"`
	Error      string `json:"error,omitempty"`
}

func getMCPOpenTabsList() string {
	// Clean up stale entries from tabsOrder
	validOrder := tabsOrder[:0]
	for _, id := range tabsOrder {
		if _, ok := Tabs[id]; ok {
			validOrder = append(validOrder, id)
		}
	}
	tabsOrder = validOrder

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
	return strings.TrimSpace(tabsInfo)
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

	mcpTaskMutex.Lock()
	defer mcpTaskMutex.Unlock()

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
					"description": "Close the current tab",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "fetch_current_tab",
					"description": "Fetch the current tab's webpage in plaintext",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "list_tabs",
					"description": "List all open tabs",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "pagedown",
					"description": "Scroll down one page and fetch the updated tab content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "pageup",
					"description": "Scroll up one page and fetch the updated tab content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "home",
					"description": "Scroll to the top of the page and fetch the updated tab content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "end",
					"description": "Scroll to the bottom of the page and fetch the updated tab content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "search_on_page",
					"description": "Find text in the current page and return a surrounding fragment with equal-length context on both sides.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"text": map[string]interface{}{
								"type":        "string",
								"description": "Literal text to search for in the current page.",
							},
						},
						"required": []string{"text"},
					},
				},
				{
					"name":        "mouse",
					"description": "Move or click using Browsh text coordinates. Provide either coordinates or text to search for. Coordinates must point to visible page text.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"action": map[string]interface{}{
								"type":        "string",
								"description": "Mouse action: move (default), click, or right_click.",
							},
							"coordinates": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"x": map[string]interface{}{"type": "integer"},
									"y": map[string]interface{}{"type": "integer"},
								},
							},
							"text": map[string]interface{}{
								"type":        "string",
								"description": "Visible page text to locate before moving or clicking.",
							},
						},
					},
				},
				{
					"name":        "keyboard_input",
					"description": "Insert literal text into the focused editable element and/or send ordered key presses to the page. " + mcpKeyboardInputHelp,
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"text": map[string]interface{}{
								"type":        "string",
								"description": "Literal text to insert into the focused editable element before any keys are sent.",
							},
							"keys": map[string]interface{}{
								"type":        "array",
								"description": "Ordered key presses to send after text. " + mcpKeyboardInputHelp,
								"items": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
				{
					"name":        "close",
					"description": "Close the current tab",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "list_tabs",
					"description": "List all open tabs",
					"inputSchema": map[string]interface{}{
						"type":       "object",
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
				{
					"name":        "fetch_current_tab",
					"description": "Fetch the current tab's webpage in plaintext",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "pagedown",
					"description": "Scroll down one page and fetch the updated tab content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "pageup",
					"description": "Scroll up one page and fetch the updated tab content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "home",
					"description": "Scroll to the top of the page and fetch the updated tab content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "end",
					"description": "Scroll to the bottom of the page and fetch the updated tab content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "search_on_page",
					"description": "Find text in the current page and return a surrounding fragment with equal-length context on both sides.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"text": map[string]interface{}{
								"type":        "string",
								"description": "Literal text to search for in the current page.",
							},
						},
						"required": []string{"text"},
					},
				},
				{
					"name":        "mouse",
					"description": "Move or click using Browsh text coordinates. Provide either coordinates or text to search for. Coordinates must point to visible page text.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"action": map[string]interface{}{
								"type":        "string",
								"description": "Mouse action: move (default), click, or right_click.",
							},
							"coordinates": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"x": map[string]interface{}{"type": "integer"},
									"y": map[string]interface{}{"type": "integer"},
								},
							},
							"text": map[string]interface{}{
								"type":        "string",
								"description": "Visible page text to locate before moving or clicking.",
							},
						},
					},
				},
				{
					"name":        "keyboard_input",
					"description": "Insert literal text into the focused editable element and/or send ordered key presses to the page. " + mcpKeyboardInputHelp,
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"text": map[string]interface{}{
								"type":        "string",
								"description": "Literal text to insert into the focused editable element before any keys are sent.",
							},
							"keys": map[string]interface{}{
								"type":        "array",
								"description": "Ordered key presses to send after text. " + mcpKeyboardInputHelp,
								"items": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
				{
					"name":        "backward",
					"description": "Navigate backward in the current tab's history and fetch the updated page content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
				{
					"name":        "forward",
					"description": "Navigate forward in the current tab's history and fetch the updated page content",
					"inputSchema": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			},
		})
	case "tools/call":



		var params mcpToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendMCPError(w, req.ID, -32602, "Invalid params")
			return
		}

		if params.Name == "close" {
			mcpCachedText = ""
			statusMsg := ""
			
			currentTabID := -1
			if CurrentTab != nil {
				currentTabID = CurrentTab.ID
			}
			
			if currentTabID != -1 {
				removeTab(currentTabID)
				statusMsg = "Closed current tab."
			} else {
				statusMsg = "No active tab to close."
			}
			
			sendMCPResponse(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("%s\n\n%s", statusMsg, getMCPOpenTabsList()),
					},
				},
			})
			return
		}

		if params.Name == "list_tabs" {
			sendMCPResponse(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": getMCPOpenTabsList(),
					},
				},
			})
			return
		}

		if params.Name == "switch_tab" {
			var args struct {
				ID string `json:"id"`
			}
			if err := decodeMCPArguments(params.Arguments, &args); err != nil {
				sendMCPError(w, req.ID, -32602, "Invalid params")
				return
			}
			if args.ID == "" {
				sendMCPError(w, req.ID, -32602, "Missing 'id' argument")
				return
			}
			var id int
			_, err := fmt.Sscanf(args.ID, "%d", &id)
			if err != nil || Tabs[id] == nil {
				sendMCPError(w, req.ID, -32602, "Invalid or unknown tab ID")
				return
			}
			sendMessageToWebExtension(fmt.Sprintf("/switch_to_tab,%d", id))
			CurrentTab = Tabs[id]
			mcpCachedText = ""
			sendMCPResponse(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": fmt.Sprintf("Switched to tab %d\n\n%s", id, getMCPOpenTabsList()),
					},
				},
			})
			return
		}

		if params.Name == "search_on_page" {
			var args mcpSearchOnPageArgs
			if err := decodeMCPArguments(params.Arguments, &args); err != nil {
				sendMCPError(w, req.ID, -32602, "Invalid params")
				return
			}
			if args.Text == "" {
				sendMCPError(w, req.ID, -32602, "Missing 'text' argument")
				return
			}
			if CurrentTab == nil {
				sendMCPError(w, req.ID, -32602, "No active tab")
				return
			}

			text, fromCache, err := getCurrentMCPText()
			if err != nil {
				sendMCPResponse(w, req.ID, map[string]interface{}{
					"isError": true,
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": fmt.Sprintf("Error searching current tab: %v", err),
						},
					},
				})
				return
			}

			snippet, _, found := searchCurrentPageText(text, args.Text)
			if !found && fromCache {
				text, err = refreshCurrentMCPText()
				if err != nil {
					sendMCPResponse(w, req.ID, map[string]interface{}{
						"isError": true,
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": fmt.Sprintf("Error searching current tab: %v", err),
							},
						},
					})
					return
				}
				snippet, _, found = searchCurrentPageText(text, args.Text)
			}
			if !found {
				sendMCPResponse(w, req.ID, map[string]interface{}{
					"isError": true,
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": fmt.Sprintf("Text '%s' was not found on the current page.", args.Text),
						},
					},
				})
				return
			}

			sendMCPResponse(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": snippet,
					},
				},
			})
			return
		}

		if params.Name == "mouse" {
			var args mcpMouseArgs
			if err := decodeMCPArguments(params.Arguments, &args); err != nil {
				sendMCPError(w, req.ID, -32602, "Invalid params")
				return
			}
			if CurrentTab == nil {
				sendMCPError(w, req.ID, -32602, "No active tab")
				return
			}

			action := normaliseMCPMouseAction(args.Action)
			if !isSupportedMCPMouseAction(action) {
				sendMCPError(w, req.ID, -32602, "Mouse 'action' must be one of 'move', 'click', or 'right_click'")
				return
			}
			hasText := strings.TrimSpace(args.Text) != ""
			hasCoordinates := args.Coordinates != nil
			if hasText == hasCoordinates {
				sendMCPError(w, req.ID, -32602, "Provide exactly one of 'text' or 'coordinates'")
				return
			}

			currentText, fromCache, err := getCurrentMCPText()
			if err != nil {
				sendMCPResponse(w, req.ID, map[string]interface{}{
					"isError": true,
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": fmt.Sprintf("Error preparing mouse action: %v", err),
						},
					},
				})
				return
			}

			var (
				targetCoordinates mcpMouseCoordinates
				textUnderCursor   string
			)
			if hasText {
				_, coordinates, found := searchCurrentPageText(currentText, args.Text)
				if !found && fromCache {
					currentText, err = refreshCurrentMCPText()
					if err == nil {
						_, coordinates, found = searchCurrentPageText(currentText, args.Text)
					}
				}
				if !found || coordinates == nil {
					sendMCPResponse(w, req.ID, map[string]interface{}{
						"isError": true,
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": fmt.Sprintf("Text '%s' was not found on the current page.", args.Text),
							},
						},
					})
					return
				}
				targetCoordinates = *coordinates
			} else {
				validatedCoordinates, resolvedText, err := validateMCPMouseCoordinates(currentText, args.Coordinates)
				if err != nil && fromCache {
					currentText, err = refreshCurrentMCPText()
					if err == nil {
						validatedCoordinates, resolvedText, err = validateMCPMouseCoordinates(currentText, args.Coordinates)
					}
				}
				if err != nil {
					sendMCPResponse(w, req.ID, map[string]interface{}{
						"isError": true,
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": err.Error(),
							},
						},
					})
					return
				}
				targetCoordinates = validatedCoordinates
				textUnderCursor = resolvedText
			}

			cursorText := ""
			pageChanged := false
			cursorText, pageChanged, err = performMCPMouseAction(action, targetCoordinates, currentText)
				if err != nil {
					sendMCPResponse(w, req.ID, map[string]interface{}{
						"isError": true,
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": err.Error(),
							},
						},
					})
					return
				}

			sendMCPResponse(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": formatMCPMouseResponse(action, hasText, targetCoordinates, textUnderCursor, cursorText, pageChanged),
					},
				},
			})
			return
		}

		isKeyboardInput := params.Name == "keyboard_input"
		isNavigationAction := params.Name == "backward" || params.Name == "forward"
		isScrollAction := params.Name == "pagedown" || params.Name == "pageup" || params.Name == "home" || params.Name == "end"
		isPostActionFetch := isScrollAction || isKeyboardInput || isNavigationAction
		var keyboardArgs mcpKeyboardInputArgs
		if isKeyboardInput {
			if err := decodeMCPArguments(params.Arguments, &keyboardArgs); err != nil {
				sendMCPError(w, req.ID, -32602, "Invalid params")
				return
			}
			if keyboardArgs.Text == "" && len(keyboardArgs.Keys) == 0 {
				sendMCPError(w, req.ID, -32602, "Missing 'text' or 'keys' argument")
				return
			}
			for _, keySpec := range keyboardArgs.Keys {
				if !isSupportedMCPKeySpec(keySpec) {
					sendMCPError(w, req.ID, -32602, fmt.Sprintf("Unsupported key '%s'. %s", keySpec, mcpKeyboardInputHelp))
					return
				}
			}
		}

		if params.Name == "fetch_current_tab" || isPostActionFetch {
			if CurrentTab == nil {
				sendMCPError(w, req.ID, -32602, "No active tab")
				return
			}

			if isScrollAction {
				sendMessageToWebExtension("/tab_command,/mcp_action," + params.Name)
				// Wait for layout to settle or infinite scroll network requests to complete
				time.Sleep(2 * time.Second)
			} else if isNavigationAction {
				sendMessageToWebExtension("/tab_command,/mcp_action," + params.Name)
				// Wait for the history navigation to begin before requesting the new raw text
				time.Sleep(2 * time.Second)
			} else if isKeyboardInput {
				if err := sendMCPKeyboardInput(keyboardArgs); err != nil {
					sendMCPResponse(w, req.ID, map[string]interface{}{
						"isError": true,
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": fmt.Sprintf("Error sending keyboard input to current tab: %v", err),
							},
						},
					})
					return
				}
				// Wait for layout to settle after the DOM receives the synthetic input
				time.Sleep(2 * time.Second)
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

			if isPostActionFetch && !isSignificantChange(mcpCachedText, text) {
				sendMCPResponse(w, req.ID, map[string]interface{}{
					"isError": true,
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": unchangedMCPActionMessage(params.Name),
						},
					},
				})
				return
			}

			mcpCachedText = text

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

		var args struct {
			URL string `json:"url"`
		}
		if err := decodeMCPArguments(params.Arguments, &args); err != nil {
			sendMCPError(w, req.ID, -32602, "Invalid params")
			return
		}
		if args.URL == "" {
			sendMCPError(w, req.ID, -32602, "Missing 'url' argument")
			return
		}

		text, err := fetchWebpageRawText(args.URL, params.Name == "open_webpage")
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

		if params.Name == "open_webpage" {
			mcpCachedText = text
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

	mode := string(RawTextModeMCP)
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

	mode := string(RawTextModeMCP)
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

func decodeMCPArguments(raw json.RawMessage, target interface{}) error {
	if len(raw) == 0 {
		return fmt.Errorf("missing arguments")
	}
	return json.Unmarshal(raw, target)
}

func sendMCPKeyboardInput(args mcpKeyboardInputArgs) error {
	action := map[string]interface{}{
		"name": "keyboard_input",
	}
	if args.Text != "" {
		action["text"] = args.Text
	}
	if len(args.Keys) > 0 {
		action["keys"] = args.Keys
	}
	payload, err := json.Marshal(action)
	if err != nil {
		return err
	}
	sendMessageToWebExtension("/tab_command,/mcp_action," + string(payload))
	return nil
}

func unchangedMCPActionMessage(actionName string) string {
	if actionName == "keyboard_input" {
		return "Keyboard input succeeded, but the page contents did not change significantly. If you still need the current page text, call 'fetch_current_tab'. If you want to send more input, call 'keyboard_input' again."
	}
	if actionName == "backward" || actionName == "forward" {
		return fmt.Sprintf("Navigation command '%s' succeeded, but the page contents did not change significantly. If you still need the current page text, call 'fetch_current_tab'. If you want to try navigation again, call '%s' again.", actionName, actionName)
	}
	return fmt.Sprintf("Scroll command '%s' succeeded, but the page contents did not change significantly. If you still need the current page text, call 'fetch_current_tab'. If you want to try another scroll action, call one of 'pagedown', 'pageup', 'home', or 'end' again.", actionName)
}

func searchCurrentPageText(rawText, query string) (string, *mcpMouseCoordinates, bool) {
	bodyText := rawTextBody(rawText)
	matchByteIndex := strings.Index(bodyText, query)
	if matchByteIndex == -1 {
		matchByteIndex = strings.Index(strings.ToLower(bodyText), strings.ToLower(query))
	}
	if matchByteIndex == -1 {
		return "", nil, false
	}
	matchRuneIndex := utf8.RuneCountInString(bodyText[:matchByteIndex])
	matchRuneLength := utf8.RuneCountInString(query)
	line, column := runeIndexToLineColumn(bodyText, matchRuneIndex)
	snippet := excerptAroundMatch(bodyText, matchRuneIndex, matchRuneLength)
	return snippet, &mcpMouseCoordinates{X: column, Y: line}, true
}

func validateMCPMouseCoordinates(rawText string, coordinates *mcpMouseCoordinatesInput) (mcpMouseCoordinates, string, error) {
	if coordinates == nil || coordinates.X == nil || coordinates.Y == nil {
		return mcpMouseCoordinates{}, "", fmt.Errorf("Mouse coordinates must include both 'x' and 'y'")
	}
	lines := rawTextLines(rawText)
	x := *coordinates.X
	y := *coordinates.Y
	if y < 0 || y >= len(lines) {
		return mcpMouseCoordinates{}, "", fmt.Errorf("Mouse coordinates (%d, %d) are outside the current page", x, y)
	}
	line := lines[y]
	if x < 0 || x >= len(line) {
		return mcpMouseCoordinates{}, "", fmt.Errorf("Mouse coordinates (%d, %d) are outside the current page", x, y)
	}
	if unicode.IsSpace(line[x]) {
		return mcpMouseCoordinates{}, "", fmt.Errorf("Mouse coordinates (%d, %d) do not point to visible page text", x, y)
	}
	return mcpMouseCoordinates{X: x, Y: y}, textUnderLineCoordinate(line, x), nil
}

func performMCPMouseAction(action string, coordinates mcpMouseCoordinates, currentText string) (string, bool, error) {
	requestID := pseudoUUID()
	actionPayload := map[string]interface{}{
		"name":       "mouse",
		"request_id": requestID,
		"action":     action,
		"coordinates": map[string]int{
			"x": coordinates.X,
			"y": coordinates.Y,
		},
	}
	payload, err := json.Marshal(actionPayload)
	if err != nil {
		return "", false, err
	}
	sendMessageToWebExtension("/tab_command,/mcp_action," + string(payload))

	resultJSON, waitErr := waitForMCPActionResult(requestID, 5*time.Second)
	result := mcpMouseActionResult{}
	if waitErr == nil {
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			return "", false, err
		}
		if result.Error != "" {
			return "", false, fmt.Errorf("%s", result.Error)
		}
	}
	if action == "move" {
		if waitErr != nil {
			return "", false, waitErr
		}
		return result.CursorText, false, nil
	}

	// Wait for any click-triggered DOM updates or navigation before comparing page text.
	time.Sleep(2 * time.Second)
	updatedText, err := fetchCurrentTabRawText()
	if err != nil {
		if waitErr != nil {
			return "", false, waitErr
		}
		return "", false, fmt.Errorf("Error fetching current tab after mouse %s: %v", action, err)
	}

	pageChanged := isSignificantChange(rawTextBody(currentText), rawTextBody(updatedText))
	mcpCachedText = updatedText
	if waitErr != nil {
		if pageChanged {
			return "", true, nil
		}
		return "", false, waitErr
	}
	return result.CursorText, pageChanged, nil
}

func getCurrentMCPText() (string, bool, error) {
	if mcpCachedText != "" {
		return mcpCachedText, true, nil
	}
	text, err := fetchCurrentTabRawText()
	if err != nil {
		return "", false, err
	}
	mcpCachedText = text
	return text, false, nil
}

func refreshCurrentMCPText() (string, error) {
	text, err := fetchCurrentTabRawText()
	if err != nil {
		return "", err
	}
	mcpCachedText = text
	return text, nil
}

func waitForMCPActionResult(requestID string, timeout time.Duration) (string, error) {
	startTime := time.Now()
	for time.Since(startTime) < timeout {
		if result, ok := mcpActionRequests.load(requestID); ok {
			mcpActionRequests.remove(requestID)
			return result, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	mcpActionRequests.remove(requestID)
	return "", fmt.Errorf("timeout waiting for mouse action result")
}

func formatMCPMouseResponse(action string, usedTextSearch bool, coordinates mcpMouseCoordinates, textUnderCursor, cursorText string, pageChanged bool) string {
	parts := []string{}
	if usedTextSearch {
		parts = append(parts, fmt.Sprintf("Mouse %s target coordinates: x=%d, y=%d.", action, coordinates.X, coordinates.Y))
	} else {
		parts = append(parts, fmt.Sprintf("Text under cursor: %s", textUnderCursor))
	}
	if cursorText != "" {
		parts = append(parts, fmt.Sprintf("Focused text cursor: %s", cursorText))
	}
	if pageChanged {
		parts = append(parts, fmt.Sprintf("Mouse %s changed the page contents significantly. Call 'fetch_current_tab' to read the updated page text.", action))
	}
	return strings.Join(parts, "\n")
}

func normaliseMCPMouseAction(action string) string {
	if strings.TrimSpace(action) == "" {
		return "move"
	}
	return strings.ToLower(strings.TrimSpace(action))
}

func isSupportedMCPMouseAction(action string) bool {
	switch action {
	case "move", "click", "right_click":
		return true
	default:
		return false
	}
}

func rawTextBody(rawText string) string {
	bodyText := rawText
	if footerIndex := strings.LastIndex(bodyText, "\n\nThe above is a text render of "); footerIndex != -1 {
		bodyText = bodyText[:footerIndex]
	} else if footerIndex := strings.LastIndex(bodyText, "\n\nURL: "); footerIndex != -1 {
		bodyText = bodyText[:footerIndex]
	} else if footerIndex := strings.LastIndex(bodyText, "\n\nBuilt by Browsh"); footerIndex != -1 {
		bodyText = bodyText[:footerIndex]
	}
	return strings.TrimRight(bodyText, "\n")
}

func rawTextLines(rawText string) [][]rune {
	parts := strings.Split(rawTextBody(rawText), "\n")
	lines := make([][]rune, 0, len(parts))
	for _, part := range parts {
		lines = append(lines, []rune(part))
	}
	return lines
}

func runeIndexToLineColumn(text string, runeIndex int) (int, int) {
	line := 0
	column := 0
	for index, character := range []rune(text) {
		if index >= runeIndex {
			break
		}
		if character == '\n' {
			line++
			column = 0
			continue
		}
		column++
	}
	return line, column
}

func excerptAroundMatch(text string, matchRuneIndex int, matchRuneLength int) string {
	textRunes := []rune(text)
	start := maxInt(0, matchRuneIndex-matchRuneLength)
	end := minInt(len(textRunes), matchRuneIndex+matchRuneLength+matchRuneLength)
	start = extendSnippetStart(textRunes, start)
	end = extendSnippetEnd(textRunes, end)
	snippet := strings.Join(strings.Fields(string(textRunes[start:end])), " ")
	if start > 0 {
		snippet = "... " + snippet
	}
	if end < len(textRunes) {
		snippet += " ..."
	}
	return snippet
}

func excerptAtLineCoordinate(line []rune, coordinate int) string {
	start := maxInt(0, coordinate-20)
	end := minInt(len(line), coordinate+21)
	start = extendSnippetStart(line, start)
	end = extendSnippetEnd(line, end)
	snippetRunes := make([]rune, 0, end-start+1)
	snippetRunes = append(snippetRunes, line[start:coordinate]...)
	snippetRunes = append(snippetRunes, '|')
	snippetRunes = append(snippetRunes, line[coordinate:end]...)
	snippet := strings.Join(strings.Fields(string(snippetRunes)), " ")
	if start > 0 {
		snippet = "... " + snippet
	}
	if end < len(line) {
		snippet += " ..."
	}
	return snippet
}

func textUnderLineCoordinate(line []rune, coordinate int) string {
	start := coordinate
	for start > 0 && !unicode.IsSpace(line[start-1]) {
		start--
	}
	end := coordinate
	for end < len(line) && !unicode.IsSpace(line[end]) {
		end++
	}
	if start == end {
		start = maxInt(0, coordinate-20)
		end = minInt(len(line), coordinate+21)
		start = extendSnippetStart(line, start)
		end = extendSnippetEnd(line, end)
	}
	return strings.TrimSpace(string(line[start:end]))
}

func extendSnippetStart(text []rune, start int) int {
	for start > 0 && !unicode.IsSpace(text[start-1]) {
		start--
	}
	return start
}

func extendSnippetEnd(text []rune, end int) int {
	for end < len(text) && !unicode.IsSpace(text[end]) {
		end++
	}
	return end
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func isSupportedMCPKeySpec(keySpec string) bool {
	parts := strings.Split(keySpec, "+")
	baseKeyCount := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return false
		}
		switch strings.ToLower(part) {
		case "shift", "ctrl", "alt":
			continue
		}
		baseKeyCount++
		if baseKeyCount > 1 {
			return false
		}
		lowerPart := strings.ToLower(part)
		switch lowerPart {
		case "enter", "escape", "space", "tab", "backspace", "delete", "insert", "up", "down", "left", "right", "home", "end", "pgup", "pgdn":
			continue
		}
		if strings.HasPrefix(lowerPart, "f") {
			functionNumber, err := strconv.Atoi(lowerPart[1:])
			if err == nil && functionNumber >= 1 && functionNumber <= 24 {
				continue
			}
		}
		if utf8.RuneCountInString(part) == 1 {
			continue
		}
		return false
	}
	return baseKeyCount == 1
}

func isSignificantChange(oldText, newText string) bool {
	if oldText == newText {
		return false
	}
	diff := len(newText) - len(oldText)
	if diff < 0 {
		diff = -diff
	}
	// Require at least 50 characters of difference to consider it a significant change,
	// ignoring minor dynamic updates like relative timestamps.
	if diff < 50 {
		return false
	}
	return true
}
