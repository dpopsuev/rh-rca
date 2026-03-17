package rp

import (
	"context"
	"fmt"
	"strconv"
)

// FetchEnvelope fetches a launch and its failed test items, mapping them
// into an Envelope.
func (p *ProjectScope) FetchEnvelope(ctx context.Context, launchID int) (*Envelope, error) {
	launch, err := p.Launches().Get(ctx, launchID)
	if err != nil {
		return nil, fmt.Errorf("fetch envelope: get launch: %w", err)
	}

	items, err := p.Items().ListAll(ctx,
		WithLaunchID(launchID),
		WithStatus("FAILED"),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch envelope: list items: %w", err)
	}

	env := &Envelope{
		RunID:       strconv.Itoa(launch.ID),
		LaunchUUID:  launch.UUID,
		Name:        launch.Name,
		FailureList: make([]FailureItem, 0, len(items)),
	}

	for _, attr := range launch.Attributes {
		env.LaunchAttributes = append(env.LaunchAttributes, Attribute{
			Key:    attr.Key,
			Value:  attr.Value,
			System: attr.System,
		})
	}

	for _, it := range items {
		path := it.Path
		if path == "" {
			path = strconv.Itoa(it.ID)
		}

		fi := FailureItem{
			ID:     it.ID,
			UUID:   it.UUID,
			Name:   it.Name,
			Type:   it.Type,
			Status: it.Status,
			Path:   path,
			CodeRef:     it.CodeRef,
			Description: it.Description,
			ParentID:    it.Parent,
		}

		if it.Issue != nil {
			fi.IssueType = it.Issue.IssueType
			fi.IssueComment = it.Issue.Comment
			fi.AutoAnalyzed = it.Issue.AutoAnalyzed
			for _, ext := range it.Issue.ExternalSystemIssues {
				fi.ExternalIssues = append(fi.ExternalIssues, ExternalIssue{
					TicketID: ext.TicketID,
					URL:      ext.URL,
				})
			}
		}

		env.FailureList = append(env.FailureList, fi)
	}

	return env, nil
}
