package rp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// ItemScope provides operations on test items within a project.
type ItemScope struct {
	project *ProjectScope
}

// ListItemsOption configures filter and pagination for item listing.
type ListItemsOption func(params url.Values)

// List returns test items matching the given filters.
// Uses the /api/v1/{project}/item endpoint.
func (s *ItemScope) List(ctx context.Context, opts ...ListItemsOption) (*PagedItems, error) {
	params := url.Values{}
	for _, opt := range opts {
		opt(params)
	}

	u := fmt.Sprintf("%s/api/v1/%s/item?%s",
		s.project.client.baseURL, s.project.projectName, params.Encode())

	var paged PagedItems
	if err := s.project.client.doJSON(ctx, "GET", u, "list items", nil, &paged); err != nil {
		return nil, err
	}
	return &paged, nil
}

// ListAll returns all test items matching the filters, auto-paginating.
func (s *ItemScope) ListAll(ctx context.Context, opts ...ListItemsOption) ([]TestItemResource, error) {
	var all []TestItemResource
	page := 1
	pageSize := 200

	for {
		pageOpts := append(opts,
			WithItemPageSize(pageSize),
			WithItemPageNumber(page),
		)
		paged, err := s.List(ctx, pageOpts...)
		if err != nil {
			return nil, err
		}
		all = append(all, paged.Content...)
		if len(paged.Content) < pageSize {
			break
		}
		page++
	}
	return all, nil
}

// Get returns a single test item by its numeric ID.
func (s *ItemScope) Get(ctx context.Context, id int) (*TestItemResource, error) {
	u := fmt.Sprintf("%s/api/v1/%s/item/%d",
		s.project.client.baseURL, s.project.projectName, id)

	var item TestItemResource
	if err := s.project.client.doJSON(ctx, "GET", u, "get item", nil, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateDefect updates the defect type for a single test item.
func (s *ItemScope) UpdateDefect(ctx context.Context, itemID int, defectType string) error {
	u := fmt.Sprintf("%s/api/v1/%s/item/%d/update",
		s.project.client.baseURL, s.project.projectName, itemID)

	body := map[string]any{
		"issues": []map[string]any{
			{"issueType": defectType},
		},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("update defect: marshal: %w", err)
	}

	return s.project.client.doJSON(ctx, "PUT", u, "update defect", bytes.NewReader(payload), nil)
}

// UpdateDefectBulk updates defect types for multiple test items in one call.
func (s *ItemScope) UpdateDefectBulk(ctx context.Context, definitions []IssueDefinition) error {
	u := fmt.Sprintf("%s/api/v1/%s/item",
		s.project.client.baseURL, s.project.projectName)

	body := map[string]any{
		"issues": definitions,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("update defect bulk: marshal: %w", err)
	}

	return s.project.client.doJSON(ctx, "PUT", u, "update defect bulk", bytes.NewReader(payload), nil)
}

// --- Item listing options ---

// WithLaunchID filters items by launch ID.
func WithLaunchID(id int) ListItemsOption {
	return func(p url.Values) { p.Set("filter.eq.launchId", strconv.Itoa(id)) }
}

// WithStatus filters items by status (e.g. "FAILED").
func WithStatus(status string) ListItemsOption {
	return func(p url.Values) { p.Set("filter.eq.status", status) }
}

// WithItemType filters items by type (e.g. "TEST", "STEP", "SUITE").
func WithItemType(itemType string) ListItemsOption {
	return func(p url.Values) { p.Set("filter.eq.type", itemType) }
}

// WithItemPageSize sets the page size for item listing.
func WithItemPageSize(size int) ListItemsOption {
	return func(p url.Values) { p.Set("page.size", strconv.Itoa(size)) }
}

// WithItemPageNumber sets the page number for item listing.
func WithItemPageNumber(n int) ListItemsOption {
	return func(p url.Values) { p.Set("page.page", strconv.Itoa(n)) }
}
