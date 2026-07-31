package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DNSProvider updates a DNS A record to point to a given IP address.
type DNSProvider interface {
	// SetRecord updates the target A record to the given IP.
	// Implementations must be idempotent.
	SetRecord(ctx context.Context, ip string) error
}

// ---------------------------------------------------------------------------
// Cloudflare implementation
// ---------------------------------------------------------------------------

// CloudflareProvider updates a DNS A record via the Cloudflare v4 API.
// It requires an API token with Zone.DNS edit permissions.
type CloudflareProvider struct {
	apiToken   string
	zoneID     string
	recordName string
	client     *http.Client
	recordID   string // cached after first lookup
}

// NewCloudflareProvider creates a Cloudflare DNS provider.
func NewCloudflareProvider(apiToken, zoneID, recordName string) *CloudflareProvider {
	return &CloudflareProvider{
		apiToken:   apiToken,
		zoneID:     zoneID,
		recordName: recordName,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

const cloudflareAPIBase = "https://api.cloudflare.com/client/v4"

// SetRecord updates the A record to point to the given IP.
func (p *CloudflareProvider) SetRecord(ctx context.Context, ip string) error {
	recordID, err := p.findRecordID(ctx)
	if err != nil {
		return fmt.Errorf("find DNS record: %w", err)
	}
	return p.updateRecord(ctx, recordID, ip)
}

// findRecordID looks up the DNS record ID by name (cached after first call).
func (p *CloudflareProvider) findRecordID(ctx context.Context) (string, error) {
	if p.recordID != "" {
		return p.recordID, nil
	}

	url := fmt.Sprintf("%s/zones/%s/dns_records?type=A&name=%s", cloudflareAPIBase, p.zoneID, p.recordName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	p.setHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var result struct {
		Success bool `json:"success"`
		Result  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"result"`
		Errors []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if !result.Success {
		msg := "unknown error"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return "", fmt.Errorf("cloudflare API error: %s", msg)
	}
	if len(result.Result) == 0 {
		return "", fmt.Errorf("no A record found for %s", p.recordName)
	}

	p.recordID = result.Result[0].ID
	return p.recordID, nil
}

// updateRecord patches the DNS record with a new IP.
func (p *CloudflareProvider) updateRecord(ctx context.Context, recordID, ip string) error {
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", cloudflareAPIBase, p.zoneID, recordID)
	payload := map[string]any{
		"type":    "A",
		"name":    p.recordName,
		"content": ip,
		"ttl":     30, // minimum TTL for fast propagation
		"proxied": false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	p.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	var result struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !result.Success {
		msg := "unknown error"
		if len(result.Errors) > 0 {
			msg = result.Errors[0].Message
		}
		return fmt.Errorf("cloudflare DNS update failed: %s", msg)
	}
	return nil
}

func (p *CloudflareProvider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
}
