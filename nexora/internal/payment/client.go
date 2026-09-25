// Package payment talks to the external payment gateway.
package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client charges customers through the gateway's HTTP API.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient returns a client for the gateway at baseURL.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: timeout}}
}

type chargeRequest struct {
	OrderRef    string `json:"order_ref"`
	AmountCents int    `json:"amount_cents"`
}

// Charge asks the gateway to charge amountCents for orderRef.
func (c *Client) Charge(ctx context.Context, orderRef string, amountCents int) error {
	body, err := json.Marshal(chargeRequest{OrderRef: orderRef, AmountCents: amountCents})
	if err != nil {
		return fmt.Errorf("payment: encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/charge", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("payment: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("payment: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("payment: gateway returned %s", resp.Status)
	}
	return nil
}
