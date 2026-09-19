package observability

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT rule_id, scope, resource_id, metric,
		operator, threshold, duration_seconds, enabled, created_at_ms, updated_at_ms
		FROM alert_rules ORDER BY rule_id`)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()
	rules := make([]AlertRule, 0)
	for rows.Next() {
		var rule AlertRule
		var enabled int
		var created, updated int64
		if err := rows.Scan(&rule.ID, &rule.Scope, &rule.ResourceID, &rule.Metric,
			&rule.Operator, &rule.Threshold, &rule.DurationSeconds, &enabled,
			&created, &updated); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		rule.Enabled = enabled == 1
		rule.CreatedAt = time.UnixMilli(created).UTC()
		rule.UpdatedAt = time.UnixMilli(updated).UTC()
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *Store) ListAlertIncidents(ctx context.Context, state string) ([]AlertIncident, error) {
	query := `SELECT incident_id, rule_id, resource_type, resource_id, opened_at_ms,
		last_observed_at_ms, resolved_at_ms, state FROM alert_incidents`
	args := []any{}
	if state != "" {
		query += " WHERE state = ?"
		args = append(args, state)
	}
	query += " ORDER BY opened_at_ms DESC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alert incidents: %w", err)
	}
	defer rows.Close()
	incidents := make([]AlertIncident, 0)
	for rows.Next() {
		var incident AlertIncident
		var opened, observed int64
		var resolved sql.NullInt64
		if err := rows.Scan(&incident.ID, &incident.RuleID, &incident.ResourceType,
			&incident.ResourceID, &opened, &observed, &resolved, &incident.State); err != nil {
			return nil, fmt.Errorf("scan alert incident: %w", err)
		}
		incident.OpenedAt = time.UnixMilli(opened).UTC()
		incident.LastObservedAt = time.UnixMilli(observed).UTC()
		if resolved.Valid {
			value := time.UnixMilli(resolved.Int64).UTC()
			incident.ResolvedAt = &value
		}
		incidents = append(incidents, incident)
	}
	return incidents, rows.Err()
}
