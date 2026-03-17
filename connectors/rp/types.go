package rp

import (
	"encoding/json"
	"fmt"
	"time"
)

// maxMillisTimestamp is the upper bound for a value to be interpreted as
// milliseconds (approximately year 2286). Values at or above this threshold
// are treated as microseconds.
const maxMillisTimestamp int64 = 1e13

// EpochMillis represents a point in time serialized as an integer epoch
// timestamp. On deserialization it auto-detects whether the value is
// milliseconds or microseconds based on its magnitude. Serialization always
// produces milliseconds.
type EpochMillis time.Time

// Time returns the underlying time.Time value.
func (e EpochMillis) Time() time.Time { return time.Time(e) }

// MarshalJSON serializes EpochMillis as Unix milliseconds.
func (e EpochMillis) MarshalJSON() ([]byte, error) {
	ms := time.Time(e).UnixMilli()
	return json.Marshal(ms)
}

// UnmarshalJSON deserializes an integer timestamp, auto-detecting ms or us.
func (e *EpochMillis) UnmarshalJSON(data []byte) error {
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("unmarshal epoch millis: %w", err)
	}
	if value >= maxMillisTimestamp {
		*e = EpochMillis(time.UnixMicro(value))
	} else {
		*e = EpochMillis(time.UnixMilli(value))
	}
	return nil
}

// --- RP Response Types (hand-written, aligned with RP 5.11 OpenAPI spec) ---

// LaunchResource represents a Report Portal launch.
type LaunchResource struct {
	ID          int                   `json:"id"`
	UUID        string                `json:"uuid,omitempty"`
	Name        string                `json:"name,omitempty"`
	Number      int                   `json:"number,omitempty"`
	Status      string                `json:"status,omitempty"`
	StartTime   *EpochMillis          `json:"startTime,omitempty"`
	EndTime     *EpochMillis          `json:"endTime,omitempty"`
	Description string                `json:"description,omitempty"`
	Owner       string                `json:"owner,omitempty"`
	Attributes  []ItemAttributeResource `json:"attributes,omitempty"`
	Statistics  *StatisticsResource   `json:"statistics,omitempty"`
}

// TestItemResource represents a Report Portal test item (step/test/suite).
type TestItemResource struct {
	ID           int                    `json:"id"`
	UUID         string                 `json:"uuid,omitempty"`
	Name         string                 `json:"name,omitempty"`
	Type         string                 `json:"type,omitempty"`
	Status       string                 `json:"status,omitempty"`
	LaunchID     int                    `json:"launchId,omitempty"`
	CodeRef      string                 `json:"codeRef,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Parent       int                    `json:"parent,omitempty"`
	Path         string                 `json:"path,omitempty"`
	PathNames    *PathNameResource      `json:"pathNames,omitempty"`
	StartTime    *EpochMillis           `json:"startTime,omitempty"`
	EndTime      *EpochMillis           `json:"endTime,omitempty"`
	Issue        *Issue                 `json:"issue,omitempty"`
	Attributes   []ItemAttributeResource `json:"attributes,omitempty"`
	Statistics   *StatisticsResource    `json:"statistics,omitempty"`
	HasChildren  bool                   `json:"hasChildren,omitempty"`
	HasStats     bool                   `json:"hasStats,omitempty"`
	TestCaseHash int32                  `json:"testCaseHash,omitempty"`
	TestCaseID   string                 `json:"testCaseId,omitempty"`
	UniqueID     string                 `json:"uniqueId,omitempty"`
}

// Issue represents the defect/issue information attached to a test item.
type Issue struct {
	IssueType            string                 `json:"issueType,omitempty"`
	Comment              string                 `json:"comment,omitempty"`
	AutoAnalyzed         bool                   `json:"autoAnalyzed,omitempty"`
	ExternalSystemIssues []ExternalSystemIssue  `json:"externalSystemIssues,omitempty"`
}

// ExternalSystemIssue links a test item to an external bug tracker.
type ExternalSystemIssue struct {
	TicketID string `json:"ticketId,omitempty"`
	URL      string `json:"url,omitempty"`
	BtsURL   string `json:"btsUrl,omitempty"`
}

// IssueDefinition is used for bulk defect updates.
type IssueDefinition struct {
	Issue      Issue `json:"issue"`
	TestItemID int   `json:"testItemId"`
}

// ItemAttributeResource represents a key-value attribute on a launch/item.
type ItemAttributeResource struct {
	Key    string `json:"key,omitempty"`
	Value  string `json:"value,omitempty"`
	System bool   `json:"system,omitempty"`
}

// StatisticsResource holds execution and defect statistics.
type StatisticsResource struct {
	Defects    map[string]map[string]int `json:"defects,omitempty"`
	Executions map[string]int            `json:"executions,omitempty"`
}

// PathNameResource holds the hierarchical path names for an item.
type PathNameResource struct {
	ItemPaths []PathSegment `json:"itemPaths,omitempty"`
}

// PathSegment is one element in the path hierarchy.
type PathSegment struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// --- Paginated response wrappers ---

// PagedLaunches is the paginated response for launch listing.
type PagedLaunches struct {
	Content []LaunchResource `json:"content"`
	Page    PageInfo         `json:"page"`
}

// PagedItems is the paginated response for item listing.
type PagedItems struct {
	Content []TestItemResource `json:"content"`
	Page    PageInfo           `json:"page"`
}

// PageInfo holds pagination metadata.
type PageInfo struct {
	Number        int `json:"number"`
	Size          int `json:"size"`
	TotalElements int `json:"totalElements"`
	TotalPages    int `json:"totalPages"`
}

// UserResource is the response from GET /users (current authenticated user).
type UserResource struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	UserRole string `json:"userRole"`
}

// ErrorRS is the standard RP error response shape.
type ErrorRS struct {
	ErrorCode int    `json:"errorCode"`
	Message   string `json:"message"`
}
