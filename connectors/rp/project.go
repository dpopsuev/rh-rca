package rp

import "log/slog"

// ProjectScope provides access to resources within a specific Report Portal project.
type ProjectScope struct {
	client      *Client
	projectName string
}

// Project returns a ProjectScope for the named project.
func (c *Client) Project(name string) *ProjectScope {
	return &ProjectScope{client: c, projectName: name}
}

// Launches returns a LaunchScope for querying launches in this project.
func (p *ProjectScope) Launches() *LaunchScope {
	return &LaunchScope{project: p}
}

// Items returns an ItemScope for querying and updating test items in this project.
func (p *ProjectScope) Items() *ItemScope {
	return &ItemScope{project: p}
}

func (p *ProjectScope) logger() *slog.Logger {
	return p.client.logger.With("project", p.projectName)
}
