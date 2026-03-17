package rp

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// LaunchScope provides read operations on launches within a project.
type LaunchScope struct {
	project *ProjectScope
}

// Get returns a single launch by its numeric ID.
func (l *LaunchScope) Get(ctx context.Context, id int) (*LaunchResource, error) {
	u := fmt.Sprintf("%s/api/v1/%s/launch/%d",
		l.project.client.baseURL, l.project.projectName, id)

	var launch LaunchResource
	if err := l.project.client.doJSON(ctx, "GET", u, "get launch", nil, &launch); err != nil {
		return nil, err
	}
	return &launch, nil
}

// GetByUUID returns a single launch by its UUID string.
func (l *LaunchScope) GetByUUID(ctx context.Context, uuid string) (*LaunchResource, error) {
	u := fmt.Sprintf("%s/api/v1/%s/launch/uuid/%s",
		l.project.client.baseURL, l.project.projectName, uuid)

	var launch LaunchResource
	if err := l.project.client.doJSON(ctx, "GET", u, "get launch by uuid", nil, &launch); err != nil {
		return nil, err
	}
	return &launch, nil
}

// ListLaunchesOption configures filter and pagination for launch listing.
type ListLaunchesOption func(params url.Values)

// List returns launches matching the given filters.
func (l *LaunchScope) List(ctx context.Context, opts ...ListLaunchesOption) (*PagedLaunches, error) {
	params := url.Values{}
	for _, opt := range opts {
		opt(params)
	}

	u := fmt.Sprintf("%s/api/v1/%s/launch?%s",
		l.project.client.baseURL, l.project.projectName, params.Encode())

	var paged PagedLaunches
	if err := l.project.client.doJSON(ctx, "GET", u, "list launches", nil, &paged); err != nil {
		return nil, err
	}
	return &paged, nil
}

// WithLaunchName filters launches by exact name.
func WithLaunchName(name string) ListLaunchesOption {
	return func(p url.Values) { p.Set("filter.eq.name", name) }
}

// WithLaunchStatus filters launches by status.
func WithLaunchStatus(status string) ListLaunchesOption {
	return func(p url.Values) { p.Set("filter.eq.status", status) }
}

// WithPageSize sets the page size for listing.
func WithPageSize(size int) ListLaunchesOption {
	return func(p url.Values) { p.Set("page.size", strconv.Itoa(size)) }
}

// WithPageNumber sets the page number (1-based) for listing.
func WithPageNumber(n int) ListLaunchesOption {
	return func(p url.Values) { p.Set("page.page", strconv.Itoa(n)) }
}

// WithSort sets the sort order (e.g. "startTime,desc").
func WithSort(sort string) ListLaunchesOption {
	return func(p url.Values) { p.Set("page.sort", sort) }
}
