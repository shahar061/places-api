package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"places_api/internal/types"
	"strings"
)

// GraphQLClient handles GraphQL operations with Supabase
type GraphQLClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewGraphQLClient creates a new GraphQL client instance
func NewGraphQLClient(baseURL, apiKey string, httpClient *http.Client) *GraphQLClient {
	return &GraphQLClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

// GetGraphQLEndpoint returns the GraphQL endpoint URL
func (c *GraphQLClient) GetGraphQLEndpoint() string {
	// Replace /rest/v1 with /graphql/v1
	return strings.Replace(c.baseURL, "/rest/v1", "/graphql/v1", 1)
}

// Execute executes a GraphQL query against Supabase
func (c *GraphQLClient) Execute(query string, variables map[string]interface{}) (*types.GraphQLResponse, error) {
	gqlReq := types.GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	bodyBytes, err := json.Marshal(gqlReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	req, err := http.NewRequest("POST", c.GetGraphQLEndpoint(), strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GraphQL request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var gqlResp types.GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return &gqlResp, nil
}

