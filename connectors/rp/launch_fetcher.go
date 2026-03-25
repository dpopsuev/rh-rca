package rp

import (
	"context"
	"time"

	"github.com/dpopsuev/rh-rca/rcatype"
)

var _ rcatype.RunDiscoverer = (*RPRunDiscoverer)(nil)

// RPRunDiscoverer implements rcatype.RunDiscoverer for ReportPortal.
type RPRunDiscoverer struct {
	client  *Client
	project string
}

// NewRunDiscoverer creates a RunDiscoverer backed by a ReportPortal API client.
func NewRunDiscoverer(baseURL, apiKeyPath, project string) (rcatype.RunDiscoverer, error) {
	key, err := ReadAPIKey(apiKeyPath)
	if err != nil {
		return nil, err
	}
	client, err := New(baseURL, key, WithTimeout(30*time.Second))
	if err != nil {
		return nil, err
	}
	return &RPRunDiscoverer{client: client, project: project}, nil
}

func (f *RPRunDiscoverer) DiscoverRuns(project string, since time.Time) ([]rcatype.RunInfo, error) {
	ctx := context.Background()
	paged, err := f.client.Project(project).Launches().List(ctx,
		WithPageSize(100),
		WithSort("startTime,desc"),
	)
	if err != nil {
		return nil, err
	}

	var runs []rcatype.RunInfo
	for _, l := range paged.Content {
		var startTime time.Time
		if l.StartTime != nil {
			startTime = l.StartTime.Time()
		}
		if !since.IsZero() && startTime.Before(since) {
			continue
		}
		failed := 0
		if l.Statistics != nil {
			if execs, ok := l.Statistics.Executions["failed"]; ok {
				failed = execs
			}
		}
		runs = append(runs, rcatype.RunInfo{
			ID:          l.ID,
			UUID:        l.UUID,
			Name:        l.Name,
			Number:      l.Number,
			Status:      l.Status,
			StartTime:   startTime,
			FailedCount: failed,
		})
	}
	return runs, nil
}

func (f *RPRunDiscoverer) FetchFailures(runID int) ([]rcatype.FailureInfo, error) {
	ctx := context.Background()
	items, err := f.client.Project(f.project).Items().ListAll(ctx,
		WithLaunchID(runID),
		WithStatus("FAILED"),
	)
	if err != nil {
		return nil, err
	}

	var failures []rcatype.FailureInfo
	for _, item := range items {
		fi := rcatype.FailureInfo{
			RunID:    runID,
			ItemID:   item.ID,
			ItemUUID: item.UUID,
			TestName: item.Name,
			Status:   item.Status,
		}
		if item.Issue != nil {
			fi.IssueType = item.Issue.IssueType
			fi.AutoAnalyzed = item.Issue.AutoAnalyzed
		}
		failures = append(failures, fi)
	}
	return failures, nil
}
