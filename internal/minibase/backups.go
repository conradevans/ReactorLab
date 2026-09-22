package minibase

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	maxBackupResponseBytes = 2 << 20
	maxUpstreamBackups     = 500
)

var (
	databaseIDPattern = regexp.MustCompile(`^database_[0-9a-f]{32}$`)
	backupIDPattern   = regexp.MustCompile(`^backup_[0-9a-f]{32}$`)
)

type Backup struct {
	ID          string     `json:"id"`
	DatabaseID  string     `json:"databaseId"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	SizeBytes   int64      `json:"sizeBytes"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type backupWire struct {
	ID                  string     `json:"id"`
	DatabaseID          string     `json:"databaseId"`
	DatabaseDisplayName string     `json:"databaseDisplayName"`
	Kind                string     `json:"kind"`
	Status              string     `json:"status"`
	SizeBytes           int64      `json:"sizeBytes"`
	CreatedAt           time.Time  `json:"createdAt"`
	CompletedAt         *time.Time `json:"completedAt"`
}

func (c *Client) DatabaseBackups(
	ctx context.Context,
	databaseID string,
) ([]Backup, error) {
	if !ValidDatabaseID(databaseID) {
		return nil, fmt.Errorf("validate MiniBase backups: invalid database ID")
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/api/v1/databases/"+databaseID+"/backups",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("build MiniBase backups request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request MiniBase backups: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"MiniBase backups returned status %d",
			response.StatusCode,
		)
	}

	var raw []backupWire
	if err := decodeBoundedJSON(
		response.Body,
		maxBackupResponseBytes,
		&raw,
	); err != nil {
		return nil, fmt.Errorf("decode MiniBase backups: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("validate MiniBase backups: missing backup list")
	}
	if len(raw) > maxUpstreamBackups {
		return nil, fmt.Errorf("validate MiniBase backups: too many backups")
	}

	backups := make([]Backup, 0, len(raw))
	for _, item := range raw {
		if !backupIDPattern.MatchString(item.ID) ||
			item.DatabaseID != databaseID ||
			!ValidDatabaseID(item.DatabaseID) ||
			!validBackupKind(item.Kind) ||
			!validBackupStatus(item.Status) ||
			item.CreatedAt.IsZero() ||
			strings.TrimSpace(item.DatabaseDisplayName) == "" {

			return nil, fmt.Errorf("validate MiniBase backups: invalid backup")
		}
		if !validBackupLifecycle(
			item.Status,
			item.SizeBytes,
			item.CreatedAt,
			item.CompletedAt,
		) {
			return nil, fmt.Errorf(
				"validate MiniBase backups: invalid backup lifecycle",
			)
		}
		item.CreatedAt = item.CreatedAt.UTC()
		if item.CompletedAt != nil {
			completedAt := item.CompletedAt.UTC()
			item.CompletedAt = &completedAt
		}
		backups = append(backups, Backup{
			ID:          item.ID,
			DatabaseID:  item.DatabaseID,
			Kind:        item.Kind,
			Status:      item.Status,
			SizeBytes:   item.SizeBytes,
			CreatedAt:   item.CreatedAt,
			CompletedAt: item.CompletedAt,
		})
	}
	return backups, nil
}

func ValidDatabaseID(value string) bool {
	return databaseIDPattern.MatchString(value)
}

func validBackupKind(value string) bool {
	return value == "manual" || value == "automatic" ||
		value == "pre_restore"
}

func validBackupStatus(value string) bool {
	return value == "creating" || value == "ready" || value == "error"
}

func validBackupLifecycle(
	status string,
	sizeBytes int64,
	createdAt time.Time,
	completedAt *time.Time,
) bool {
	switch status {
	case "creating":
		return sizeBytes == 0 && completedAt == nil
	case "ready":
		return sizeBytes > 0 && completedAt != nil &&
			!completedAt.IsZero() && !completedAt.Before(createdAt)
	case "error":
		return sizeBytes == 0 && completedAt != nil &&
			!completedAt.IsZero() && !completedAt.Before(createdAt)
	default:
		return false
	}
}
