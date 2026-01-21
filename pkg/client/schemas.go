package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Schema represents a schema definition.
type Schema struct {
	ID            string         `json:"id"`
	OrgID         string         `json:"org_id"`
	ProjectID     string         `json:"project_id"`
	Name          string         `json:"name"`
	TopicPattern  string         `json:"topic_pattern"`
	Description   string         `json:"description,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	LatestVersion *SchemaVersion `json:"latest_version,omitempty"`
}

// SchemaVersion represents a version of a schema.
type SchemaVersion struct {
	ID             string          `json:"id"`
	SchemaID       string          `json:"schema_id"`
	Version        string          `json:"version"`
	Schema         json.RawMessage `json:"schema"`
	ValidationMode string          `json:"validation_mode"`
	OnInvalid      string          `json:"on_invalid"`
	Compatibility  string          `json:"compatibility"`
	Examples       json.RawMessage `json:"examples,omitempty"`
	Fingerprint    string          `json:"fingerprint"`
	IsLatest       bool            `json:"is_latest"`
	CreatedAt      time.Time       `json:"created_at"`
	CreatedBy      string          `json:"created_by,omitempty"`
}

// SchemaListResponse is the response from listing schemas.
type SchemaListResponse struct {
	Schemas []*Schema `json:"schemas"`
	Count   int       `json:"count"`
}

// SchemaVersionListResponse is the response from listing schema versions.
type SchemaVersionListResponse struct {
	Versions []*SchemaVersion `json:"versions"`
	Count    int              `json:"count"`
}

// ValidationError represents a single validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
}

// ValidationResult is the result of validating data against a schema.
type ValidationResult struct {
	Valid   bool              `json:"valid"`
	Errors  []ValidationError `json:"errors,omitempty"`
	Schema  string            `json:"schema,omitempty"`
	Version string            `json:"version,omitempty"`
}

// CreateSchemaRequest is the request to create a schema.
type CreateSchemaRequest struct {
	Name         string   `json:"name"`
	TopicPattern string   `json:"topic_pattern"`
	Description  string   `json:"description,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// CreateSchemaVersionRequest is the request to create a schema version.
type CreateSchemaVersionRequest struct {
	Version        string          `json:"version"`
	Schema         json.RawMessage `json:"schema"`
	ValidationMode string          `json:"validation_mode,omitempty"`
	OnInvalid      string          `json:"on_invalid,omitempty"`
	Compatibility  string          `json:"compatibility,omitempty"`
	Examples       json.RawMessage `json:"examples,omitempty"`
}

// UpdateSchemaRequest is the request to update a schema.
type UpdateSchemaRequest struct {
	TopicPattern string   `json:"topic_pattern,omitempty"`
	Description  string   `json:"description,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// ValidateDataRequest is the request to validate data against a schema.
type ValidateDataRequest struct {
	Data json.RawMessage `json:"data"`
}

// SchemaCreate creates a new schema.
func (c *Client) SchemaCreate(req CreateSchemaRequest) (*Schema, error) {
	reqBody, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", c.server+"/api/v1/schemas", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &AuthError{Message: "invalid or missing API key"}
	}

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var schema Schema
	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		return nil, err
	}

	return &schema, nil
}

// SchemaList lists all schemas.
func (c *Client) SchemaList() (*SchemaListResponse, error) {
	req, err := http.NewRequest("GET", c.server+"/api/v1/schemas", nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to list schemas"}
	}

	var result SchemaListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SchemaGet retrieves a schema by name.
func (c *Client) SchemaGet(name string) (*Schema, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/schemas/%s", c.server, name), nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "schema not found"}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to get schema"}
	}

	var schema Schema
	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		return nil, err
	}

	return &schema, nil
}

// SchemaUpdate updates a schema.
func (c *Client) SchemaUpdate(name string, req UpdateSchemaRequest) (*Schema, error) {
	reqBody, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("PUT", fmt.Sprintf("%s/api/v1/schemas/%s", c.server, name), bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var schema Schema
	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		return nil, err
	}

	return &schema, nil
}

// SchemaDelete deletes a schema.
func (c *Client) SchemaDelete(name string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/schemas/%s", c.server, name), nil)
	if err != nil {
		return err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	return nil
}

// SchemaVersionCreate creates a new version of a schema.
func (c *Client) SchemaVersionCreate(schemaName string, req CreateSchemaVersionRequest) (*SchemaVersion, error) {
	reqBody, _ := json.Marshal(req)

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/schemas/%s/versions", c.server, schemaName), bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var version SchemaVersion
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return nil, err
	}

	return &version, nil
}

// SchemaVersionList lists all versions of a schema.
func (c *Client) SchemaVersionList(schemaName string) (*SchemaVersionListResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/schemas/%s/versions", c.server, schemaName), nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to list versions"}
	}

	var result SchemaVersionListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SchemaVersionGet retrieves a specific version of a schema.
func (c *Client) SchemaVersionGet(schemaName, version string) (*SchemaVersion, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/schemas/%s/versions/%s", c.server, schemaName, version), nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "version not found"}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to get version"}
	}

	var sv SchemaVersion
	if err := json.NewDecoder(resp.Body).Decode(&sv); err != nil {
		return nil, err
	}

	return &sv, nil
}

// SchemaValidate validates data against a schema.
func (c *Client) SchemaValidate(schemaName string, data json.RawMessage) (*ValidationResult, error) {
	reqBody, _ := json.Marshal(ValidateDataRequest{Data: data})

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/schemas/%s/validate", c.server, schemaName), bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "schema not found"}
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
	}

	var result ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// SchemaForTopic finds the schema that matches a topic.
func (c *Client) SchemaForTopic(topic string) (*Schema, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/schemas/for-topic/%s", c.server, topic), nil)
	if err != nil {
		return nil, err
	}
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // No schema for this topic
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: "failed to get schema for topic"}
	}

	var schema Schema
	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		return nil, err
	}

	return &schema, nil
}
