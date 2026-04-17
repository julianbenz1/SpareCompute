package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/julianbenz1/SpareCompute/internal/common"
)

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Start(ctx context.Context, controlURL string, req common.RuntimeStartRequest) (common.RuntimeStartResponse, error) {
	var out common.RuntimeStartResponse
	err := c.post(ctx, controlURL, "/api/runtime/start", req, &out)
	return out, err
}

func (c *Client) Stop(ctx context.Context, controlURL string, req common.RuntimeStopRequest) error {
	return c.post(ctx, controlURL, "/api/runtime/stop", req, nil)
}

func (c *Client) Checkpoint(ctx context.Context, controlURL string, req common.RuntimeCheckpointRequest) error {
	return c.post(ctx, controlURL, "/api/runtime/checkpoint", req, nil)
}

func (c *Client) Restore(ctx context.Context, controlURL string, req common.RuntimeRestoreRequest) (common.RuntimeStartResponse, error) {
	var out common.RuntimeStartResponse
	err := c.post(ctx, controlURL, "/api/runtime/restore", req, &out)
	return out, err
}

func (c *Client) post(ctx context.Context, baseURL, path string, payload any, out any) error {
	url := strings.TrimRight(baseURL, "/") + path
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("runtime request %s failed with status %s", path, res.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}
