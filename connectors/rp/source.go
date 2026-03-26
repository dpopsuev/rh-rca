package rp

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/dpopsuev/rh-rca/rcatype"
)

var _ rcatype.SourceReader = (*SourceReaderRP)(nil)

// SourceReaderRP implements rcatype.SourceReader for ReportPortal.
type SourceReaderRP struct {
	client  *Client
	project string
}

// NewSourceReader creates a SourceReader connected to a ReportPortal instance.
func NewSourceReader(baseURL, apiKeyPath, project string) (rcatype.SourceReader, error) {
	key, err := ReadAPIKey(apiKeyPath)
	if err != nil {
		return nil, err
	}
	client, err := New(baseURL, key, WithTimeout(30*time.Second))
	if err != nil {
		return nil, err
	}
	return &SourceReaderRP{client: client, project: project}, nil
}

func (a *SourceReaderRP) FetchEnvelope(runID string) (*rcatype.Envelope, error) {
	launchID, err := strconv.Atoi(runID)
	if err != nil {
		return nil, fmt.Errorf("invalid runID %q: %w", runID, err)
	}
	f := NewFetcher(a.client, a.project)
	rpEnv, err := f.Fetch(launchID)
	if err != nil {
		return nil, err
	}
	return envelopeToRCAType(rpEnv), nil
}

func (a *SourceReaderRP) EnvelopeFetcher() rcatype.EnvelopeFetcher {
	return &envelopeFetcherBridge{client: a.client, project: a.project}
}

func (a *SourceReaderRP) CurrentUser() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if u, err := a.client.GetCurrentUser(ctx); err == nil && u.UserID != "" {
		return u.UserID
	}
	return ""
}

type envelopeFetcherBridge struct {
	client  *Client
	project string
}

func (b *envelopeFetcherBridge) Fetch(runID string) (*rcatype.Envelope, error) {
	launchID, err := strconv.Atoi(runID)
	if err != nil {
		return nil, fmt.Errorf("invalid runID %q: %w", runID, err)
	}
	f := NewFetcher(b.client, b.project)
	rpEnv, err := f.Fetch(launchID)
	if err != nil {
		return nil, err
	}
	return envelopeToRCAType(rpEnv), nil
}

// DefectWriterRP implements rcatype.DefectWriter for ReportPortal.
type DefectWriterRP struct {
	pusher *Pusher
}

var _ rcatype.DefectWriter = (*DefectWriterRP)(nil)

func NewDefectWriter(baseURL, apiKeyPath, project, submittedBy string) (rcatype.DefectWriter, error) {
	key, err := ReadAPIKey(apiKeyPath)
	if err != nil {
		return nil, err
	}
	client, err := New(baseURL, key, WithTimeout(30*time.Second))
	if err != nil {
		return nil, err
	}
	return &DefectWriterRP{pusher: NewPusher(client, project, submittedBy, "")}, nil
}

func (p *DefectWriterRP) Push(verdict rcatype.RCAVerdict) (*rcatype.PushedRecord, error) {
	st := NewMemPushStore()
	if err := p.pusher.PushVerdict(verdict, st); err != nil {
		return nil, err
	}
	rec := st.LastPushed()
	if rec == nil {
		return nil, nil
	}
	return &rcatype.PushedRecord{RunID: rec.RunID, DefectType: rec.DefectType}, nil
}

// envelopeToRCAType converts RP types to rcatype, storing RP-specific values in Tags.
func envelopeToRCAType(e *Envelope) *rcatype.Envelope {
	if e == nil {
		return nil
	}
	tags := map[string]string{}
	if e.LaunchUUID != "" {
		tags["rp.launch_uuid"] = e.LaunchUUID
	}
	env := &rcatype.Envelope{
		RunID: e.RunID,
		Name:  e.Name,
	}
	if len(tags) > 0 {
		env.Tags = tags
	}
	for _, f := range e.FailureList {
		ftags := map[string]string{}
		if f.UUID != "" {
			ftags["rp.uuid"] = f.UUID
		}
		if f.Type != "" {
			ftags["rp.type"] = f.Type
		}
		if f.Path != "" {
			ftags["rp.path"] = f.Path
		}
		if f.CodeRef != "" {
			ftags["rp.code_ref"] = f.CodeRef
		}
		if f.ParentID != 0 {
			ftags["rp.parent_id"] = strconv.Itoa(f.ParentID)
		}
		if f.IssueType != "" {
			ftags["rp.issue_type"] = f.IssueType
		}
		if f.AutoAnalyzed {
			ftags["rp.auto_analyzed"] = "true"
		}
		item := rcatype.FailureItem{
			ID:             strconv.Itoa(f.ID),
			Name:           f.Name,
			Status:         f.Status,
			Description:    f.Description,
			ErrorMessage:   f.Description,
			LogSnippet:     f.IssueComment,
			ExternalIssues: nil,
		}
		if len(ftags) > 0 {
			item.Tags = ftags
		}
		for _, ei := range f.ExternalIssues {
			item.ExternalIssues = append(item.ExternalIssues, rcatype.ExternalIssue{
				TicketID: ei.TicketID, URL: ei.URL,
			})
		}
		env.FailureList = append(env.FailureList, item)
	}
	for _, a := range e.LaunchAttributes {
		env.LaunchAttributes = append(env.LaunchAttributes, rcatype.Attribute{
			Key: a.Key, Value: a.Value, System: a.System,
		})
	}
	return env
}
