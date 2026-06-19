package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type BeszelClient struct {
	url    string
	email  string
	password string
	http *http.Client

	mu sync.Mutex
	token string
}

func NewBeszelClient(url, email, password string) *BeszelClient {
	return &BeszelClient{
		url:      url,
		email:    email,
		password: password,
		http:     &http.Client{},
	}
}


// authenticate via PocketBase and cache JWT
func (c *BeszelClient) authenticate(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"identity": c.email,
		"password": c.password,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, 
	c.url+"/api/collections/users/auth-with-password", 
	bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: %s", resp.Status)
	}
	
	var out struct {
		Token string `json:"token"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	
	c.mu.Lock()
	c.token = out.Token
	c.mu.Unlock()
	return nil
}

func (c *BeszelClient) currentToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *BeszelClient) get(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	if c.currentToken() == "" {
		if err := c.authenticate(ctx); err != nil {
			return nil, err
		}
	}
	
	do := func() (*http.Response, error) {
		u := c.url + path
		if len(query) > 0 {
			u += "?" + query.Encode()
		}
		
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", c.currentToken())
		return c.http.Do(req)
	}

	resp, err := do()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if err := c.authenticate(ctx); err != nil {
			return nil, err
		}
		if resp, err = do(); err != nil {
			return nil, err
		}
	}
	
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Get %s: status %d", path, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

type listResponse struct {
	Items []map[string]any `json:"items"`
}

// MCP took input/output types
type ListSystemsInput struct {}

type ListSystemsOutput struct {
	Systems []map[string]any `json:"systems" jsonschema:"monitored systems visible to this user"`
}

type GetSystemStatusInput struct {
	SystemID string `json:"system_id" jsonschema:"the Beszel system record ID, as returned by list_systems"`
}

type GetSystemStatusOutput struct {
	Stats map[string]any `json:"stats" jsonschema:"current system metrics and status"`
}

// main

func main() {
	client := NewBeszelClient(
		mustEnv("BESZEL_URL"),
		mustEnv("BESZEL_EMAIL"),
		mustEnv("BESZEL_PASSWORD"),
	)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "beszel-mcp",
		Version: "0.1.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_systems",
		Description: "List all systems visible to the authenticated user",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListSystemsInput) (*mcp.CallToolResult, ListSystemsOutput, error) {
		q := url.Values{}
		q.Set("perPage", "100")

		raw, err := client.get(ctx, "/api/collections/systems/records", q)

		if err != nil {
			return nil, ListSystemsOutput{}, err
		}
		
		var lr listResponse
		if err := json.Unmarshal(raw, &lr); err != nil {
			return nil, ListSystemsOutput{}, err
		}
		return nil, ListSystemsOutput{Systems: lr.Items}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_system_status",
		Description: "Get current status and metrics for a specific system",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input GetSystemStatusInput) (*mcp.CallToolResult, GetSystemStatusOutput, error) {
		q := url.Values{}

		q.Set("filter", fmt.Sprintf("id = '%s'", input.SystemID))
		q.Set("sort", "-created")
		q.Set("perPage", "1")

		raw, err := client.get(ctx, "/api/collections/systems/records", q)
		if err != nil {
			return nil, GetSystemStatusOutput{}, err
		}

		var lr listResponse
		if err := json.Unmarshal(raw, &lr); err != nil {
			return nil, GetSystemStatusOutput{}, err
		}

		if len(lr.Items) == 0 {
			return nil, GetSystemStatusOutput{}, fmt.Errorf("no stats found for system %s", input.SystemID)
		}

		return nil, GetSystemStatusOutput{Stats: lr.Items[0]}, nil
	})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable: %s", key)
	}
	return v
}


