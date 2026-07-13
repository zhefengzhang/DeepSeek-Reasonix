package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"reasonix/internal/tool"
)

func init() { tool.RegisterBuiltin(headroomRetrieve{}) }

// headroomRetrieve implements the headroom_retrieve CCR tool. When headroom
// compresses a tool output, it stores the original in a local SQLite cache and
// emits a marker like "[N items compressed... hash=abc123]". This tool calls the
// headroom proxy's /v1/retrieve endpoint to fetch the uncompressed original by
// hash key.
type headroomRetrieve struct{}

func (headroomRetrieve) Name() string { return "headroom_retrieve" }

func (headroomRetrieve) Description() string {
	return "Retrieve original uncompressed content that was compressed to save tokens. Use this when you need more data than what's shown in compressed tool results. The hash is provided in compression markers like [N items compressed... hash=abc123]."
}

func (headroomRetrieve) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "hash":{"type":"string","description":"Hash key from the compression marker (e.g., 'abc123' from hash=abc123)"}
},
"required":["hash"]
}`)
}

func (headroomRetrieve) ReadOnly() bool { return true }

func (headroomRetrieve) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(args, &params); err != nil || params.Hash == "" {
		return "", fmt.Errorf("headroom_retrieve requires a non-empty \"hash\" parameter")
	}

	port := 8787
	if s := os.Getenv("HEADROOM_PORT"); s != "" {
		if p, err := strconv.Atoi(s); err == nil && p > 0 && p < 65536 {
			port = p
		}
	}

	body := fmt.Sprintf(`{"hash":%q}`, params.Hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/v1/retrieve", port),
		bytes.NewReader([]byte(body)))
	if err != nil {
		return "", fmt.Errorf("headroom_retrieve: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf(
			"headroom proxy not reachable on port %d (error: %v). "+
				"Enable headroom compression in the provider settings first, "+
				"or read the original file directly with read_file.",
			port, err,
		)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("headroom proxy returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		OriginalContent string `json:"original_content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return string(respBody), nil // return raw body on parse failure
	}

	if result.OriginalContent == "" {
		return "", fmt.Errorf("content not found or expired for hash %q", params.Hash)
	}
	return result.OriginalContent, nil
}
