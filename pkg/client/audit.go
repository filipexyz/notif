package client

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	ID        int64           `json:"id"`
	Timestamp string          `json:"timestamp"`
	Actor     string          `json:"actor"`
	Action    string          `json:"action"`
	OrgID     string          `json:"org_id,omitempty"`
	Target    string          `json:"target,omitempty"`
	Detail    json.RawMessage `json:"detail,omitempty"`
	IPAddress string          `json:"ip_address,omitempty"`
}

// AuditListResponse is the response from listing audit entries.
type AuditListResponse struct {
	Entries []AuditEntry `json:"entries"`
	Count   int          `json:"count"`
}

// AuditQueryOptions configures audit log queries.
type AuditQueryOptions struct {
	Org    string
	Action string
	Since  string // duration string like "1h", "30m"
	Limit  int
}

// AuditList queries the audit log.
func (c *Client) AuditList(opts AuditQueryOptions) (*AuditListResponse, error) {
	u, _ := url.Parse(c.server + "/api/v1/audit")
	q := u.Query()

	if opts.Org != "" {
		q.Set("org", opts.Org)
	}
	if opts.Action != "" {
		q.Set("action", opts.Action)
	}
	if opts.Since != "" {
		q.Set("since", opts.Since)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &AuthError{Message: "invalid or missing API key"}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to query audit log"}
	}

	var result AuditListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
