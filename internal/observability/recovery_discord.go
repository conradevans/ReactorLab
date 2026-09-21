package observability

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/conradevans/ReactorLab/internal/system"
)

const (
	RecoveryDiscordTitle             = "ReactorLab recovered from an unexpected shutdown"
	defaultDiscordWebhookFile        = "/etc/reactorlab/discord-webhook"
	defaultRecoveryWorkerPoll        = 30 * time.Second
	recoveryDiscordStateTimeout      = 10 * time.Second
	discordWebhookOperationTimeout   = 20 * time.Second
	maxRecoveryDiscordLastError      = 512
	maxDiscordWebhookBytes           = 4096
	maxDiscordContentBytes           = 2000
	maxDiscordUsernameBytes          = 80
	maxDiscordRetryAfter             = 24 * time.Hour
	recoveryDiscordStatePending      = "pending"
	recoveryDiscordStateRetry        = "retry"
	recoveryDiscordStateSent         = "sent"
	discordErrorDeliveryFailed       = "delivery_failed"
	discordErrorNetwork              = "network_error"
	discordErrorRateLimited          = "rate_limited"
	discordErrorServer               = "server_error"
	discordErrorWebhookNotFound      = "webhook_not_found"
	discordErrorForbidden            = "forbidden"
	discordErrorInvalidConfiguration = "invalid_configuration"
)

var ErrRecoveryDiscordDeliveryUnavailable = errors.New("recovery Discord delivery unavailable")

type RecoveryDiscordConfig struct {
	Enabled          bool
	WebhookFile      string
	Username         string
	Timezone         string
	Location         *time.Location
	TimezoneFallback bool
}

func RecoveryDiscordConfigFromEnvironment() (RecoveryDiscordConfig, error) {
	return recoveryDiscordConfigFromLookup(os.LookupEnv)
}

func recoveryDiscordConfigFromLookup(
	lookup func(string) (string, bool),
) (RecoveryDiscordConfig, error) {
	value := func(key string) string {
		raw, _ := lookup(key)
		return strings.TrimSpace(raw)
	}
	config := RecoveryDiscordConfig{
		WebhookFile: value("REACTORLAB_DISCORD_WEBHOOK_FILE"),
		Username:    value("REACTORLAB_DISCORD_USERNAME"),
		Timezone:    value("REACTORLAB_RECOVERY_NOTIFICATION_TIMEZONE"),
		Location:    time.UTC,
	}
	if config.WebhookFile == "" {
		config.WebhookFile = defaultDiscordWebhookFile
	}
	if config.Timezone == "" {
		config.Timezone = "UTC"
	} else if location, err := time.LoadLocation(config.Timezone); err == nil {
		config.Location = location
	} else {
		config.Timezone = "UTC"
		config.TimezoneFallback = true
	}

	enabledValue := value("REACTORLAB_RECOVERY_DISCORD_ENABLED")
	if enabledValue != "" {
		enabled, err := strconv.ParseBool(enabledValue)
		if err != nil {
			return config, errors.New("invalid recovery Discord enabled flag")
		}
		config.Enabled = enabled
	}
	if !config.Enabled {
		return config, nil
	}
	if err := validateRecoveryDiscordDeliveryConfig(config); err != nil {
		return config, err
	}
	return config, nil
}

func validateRecoveryDiscordDeliveryConfig(config RecoveryDiscordConfig) error {
	if config.WebhookFile == "" || !filepath.IsAbs(config.WebhookFile) {
		return errors.New("absolute Discord webhook file path is required")
	}
	if len(config.Username) > maxDiscordUsernameBytes ||
		strings.IndexFunc(config.Username, unicode.IsControl) >= 0 {
		return errors.New("invalid Discord webhook username")
	}
	return nil
}

type RecoveryDiscordMessage struct {
	Content string
}

func FormatRecoveryDiscordMessage(
	incident RecoveryIncident,
	protection system.RecoveryProtectionState,
	location *time.Location,
) (RecoveryDiscordMessage, error) {
	if incident.LastKnownAliveAt.IsZero() || incident.RecoveredAt.IsZero() ||
		incident.RecoveredAt.Before(incident.LastKnownAliveAt) ||
		incident.DowntimeSeconds < 0 || incident.Status != recoveryIncidentStatus {
		return RecoveryDiscordMessage{}, errors.New("invalid recovery incident")
	}
	if location == nil {
		location = time.UTC
	}
	hardware := recoveryProtectionLabel(protection.HardwareWatchdog.State)
	rtc := recoveryProtectionLabel(protection.RTC.State)
	aggregateState := protection.State
	if !system.ValidRecoveryProtectionState(aggregateState) {
		aggregateState = system.AggregateRecoveryProtectionState(
			normalizedRecoveryProtectionState(protection.HardwareWatchdog.State),
			normalizedRecoveryProtectionState(protection.RTC.State),
		)
	}
	content := fmt.Sprintf(
		"**%s**\n\n"+
			"Last known alive: %s\n"+
			"Recovered: %s\n"+
			"Downtime: %s\n"+
			"Status: Recovered\n\n"+
			"Automatic recovery: %s\n"+
			"Hardware watchdog: %s\n"+
			"RTC recovery: %s\n\n"+
			"ReactorLab is back online.",
		RecoveryDiscordTitle,
		formatRecoveryTimestamp(incident.LastKnownAliveAt, location),
		formatRecoveryTimestamp(incident.RecoveredAt, location),
		formatRecoveryDuration(incident.DowntimeSeconds),
		recoveryProtectionLabel(aggregateState),
		hardware,
		rtc,
	)
	if len(content) > maxDiscordContentBytes {
		return RecoveryDiscordMessage{}, errors.New("recovery Discord message exceeds safe size")
	}
	return RecoveryDiscordMessage{Content: content}, nil
}

func formatRecoveryTimestamp(value time.Time, location *time.Location) string {
	if location == nil {
		location = time.UTC
	}
	return value.In(location).Format("Jan 2, 2006 3:04 PM MST")
}

func formatRecoveryDuration(seconds int64) string {
	if seconds < 0 {
		return "Unavailable"
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remainingSeconds := seconds % 60
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	case minutes > 0 && remainingSeconds > 0:
		return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", remainingSeconds)
	}
}

func normalizedRecoveryProtectionState(value string) string {
	if system.ValidRecoveryProtectionState(value) {
		return value
	}
	return system.RecoveryProtectionUnavailable
}

func recoveryProtectionLabel(value string) string {
	switch normalizedRecoveryProtectionState(value) {
	case system.RecoveryProtectionArmed:
		return "Armed"
	case system.RecoveryProtectionNotArmed:
		return "Not armed"
	default:
		return "Unavailable"
	}
}

type RecoveryDiscordSender interface {
	SendRecoveryDiscord(context.Context, RecoveryDiscordMessage) error
}

type DiscordWebhookSender struct {
	webhookFile string
	username    string
	client      *http.Client
}

func NewDiscordWebhookSender(config RecoveryDiscordConfig) (*DiscordWebhookSender, error) {
	if err := validateRecoveryDiscordDeliveryConfig(config); err != nil {
		return nil, ErrRecoveryDiscordDeliveryUnavailable
	}
	return &DiscordWebhookSender{
		webhookFile: config.WebhookFile,
		username:    config.Username,
		client:      &http.Client{Timeout: discordWebhookOperationTimeout},
	}, nil
}

type discordDeliveryError struct {
	category   string
	retryAfter time.Duration
}

func (e discordDeliveryError) Error() string { return e.category }

func (e discordDeliveryError) Unwrap() error {
	return ErrRecoveryDiscordDeliveryUnavailable
}

func newDiscordDeliveryError(category string, retryAfter time.Duration) error {
	return discordDeliveryError{
		category:   sanitizeRecoveryDiscordLastError(category),
		retryAfter: boundedDiscordRetryAfter(retryAfter),
	}
}

func (s *DiscordWebhookSender) SendRecoveryDiscord(
	ctx context.Context,
	message RecoveryDiscordMessage,
) error {
	if s == nil || s.client == nil || message.Content == "" ||
		len(message.Content) > maxDiscordContentBytes {
		return newDiscordDeliveryError(discordErrorInvalidConfiguration, 0)
	}
	webhookURL, err := readDiscordWebhookURL(s.webhookFile)
	if err != nil {
		return newDiscordDeliveryError(discordErrorInvalidConfiguration, 0)
	}
	payload := struct {
		Content  string `json:"content"`
		Username string `json:"username,omitempty"`
	}{
		Content:  message.Content,
		Username: s.username,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return newDiscordDeliveryError(discordErrorInvalidConfiguration, 0)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		webhookURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return newDiscordDeliveryError(discordErrorInvalidConfiguration, 0)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "ReactorLab recovery notification")
	response, err := s.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return newDiscordDeliveryError(discordErrorNetwork, 0)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		return newDiscordDeliveryError(
			discordErrorRateLimited,
			parseDiscordRetryAfter(response.Header.Get("Retry-After"), time.Now()),
		)
	case response.StatusCode >= 500:
		return newDiscordDeliveryError(discordErrorServer, 0)
	case response.StatusCode == http.StatusForbidden:
		return newDiscordDeliveryError(discordErrorForbidden, 0)
	case response.StatusCode == http.StatusNotFound:
		return newDiscordDeliveryError(discordErrorWebhookNotFound, 0)
	default:
		return newDiscordDeliveryError(discordErrorInvalidConfiguration, 0)
	}
}

func readDiscordWebhookURL(path string) (string, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil || !pathInfo.Mode().IsRegular() {
		return "", ErrRecoveryDiscordDeliveryUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return "", ErrRecoveryDiscordDeliveryUnavailable
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(pathInfo, info) {
		return "", ErrRecoveryDiscordDeliveryUnavailable
	}
	permissions := info.Mode().Perm()
	if permissions&0o007 != 0 || permissions&0o020 != 0 || permissions&0o111 != 0 || permissions&0o444 == 0 {
		return "", ErrRecoveryDiscordDeliveryUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(file, maxDiscordWebhookBytes+1))
	if err != nil || len(data) > maxDiscordWebhookBytes {
		return "", ErrRecoveryDiscordDeliveryUnavailable
	}
	webhookURL := strings.TrimRight(string(data), "\r\n")
	if webhookURL == "" || strings.TrimSpace(webhookURL) != webhookURL ||
		strings.IndexFunc(webhookURL, unicode.IsControl) >= 0 ||
		validateDiscordWebhookURL(webhookURL) != nil {
		return "", ErrRecoveryDiscordDeliveryUnavailable
	}
	return webhookURL, nil
}

func validateDiscordWebhookURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return ErrRecoveryDiscordDeliveryUnavailable
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	webhookPath := false
	for index := 0; index+3 < len(parts); index++ {
		if parts[index] == "api" && parts[index+1] == "webhooks" &&
			parts[index+2] != "" && parts[index+3] != "" {
			webhookPath = true
			break
		}
	}
	if !webhookPath {
		return ErrRecoveryDiscordDeliveryUnavailable
	}
	return nil
}

func parseDiscordRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds > 0 {
		return boundedDiscordRetryAfter(time.Duration(seconds * float64(time.Second)))
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return boundedDiscordRetryAfter(at.Sub(now))
	}
	return 0
}

func boundedDiscordRetryAfter(value time.Duration) time.Duration {
	if value <= 0 {
		return 0
	}
	if value > maxDiscordRetryAfter {
		return maxDiscordRetryAfter
	}
	return value
}

type recoveryDiscordWork struct {
	Incident     RecoveryIncident
	AttemptCount int
}

type recoveryDiscordOutbox interface {
	NextRecoveryDiscord(context.Context, time.Time) (*recoveryDiscordWork, error)
	MarkRecoveryDiscordSent(context.Context, string, time.Time) error
	MarkRecoveryDiscordFailed(context.Context, string, time.Time, string) error
}

func (s *Store) NextRecoveryDiscord(
	ctx context.Context,
	now time.Time,
) (*recoveryDiscordWork, error) {
	row := s.db.QueryRowContext(ctx, `SELECT o.event_id, o.attempt_count, e.details_json
		FROM recovery_discord_outbox o
		JOIN events e ON e.event_id = o.event_id
		WHERE o.state IN (?, ?) AND o.next_attempt_at_ms <= ?
			AND e.event_type = ? AND e.source = ? AND e.resource_type = ?
		ORDER BY o.next_attempt_at_ms, o.created_at_ms, o.event_id
		LIMIT 1`,
		recoveryDiscordStatePending,
		recoveryDiscordStateRetry,
		now.UTC().UnixMilli(),
		recoveryEventType,
		"reactorlab",
		"host",
	)
	var eventID, detailsJSON string
	var attemptCount int
	if err := row.Scan(&eventID, &attemptCount, &detailsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("read recovery Discord outbox: %w", err)
	}
	incident, err := recoveryIncidentFromJSON(eventID, detailsJSON)
	if err != nil {
		return nil, err
	}
	return &recoveryDiscordWork{
		Incident:     *incident,
		AttemptCount: attemptCount,
	}, nil
}

func (s *Store) MarkRecoveryDiscordSent(
	ctx context.Context,
	eventID string,
	sentAt time.Time,
) error {
	result, err := s.db.ExecContext(ctx, `UPDATE recovery_discord_outbox
		SET state = ?, attempt_count = attempt_count + 1,
			next_attempt_at_ms = ?, sent_at_ms = ?, last_error = ''
		WHERE event_id = ? AND state IN (?, ?)`,
		recoveryDiscordStateSent,
		sentAt.UTC().UnixMilli(),
		sentAt.UTC().UnixMilli(),
		eventID,
		recoveryDiscordStatePending,
		recoveryDiscordStateRetry,
	)
	if err != nil {
		return fmt.Errorf("mark recovery Discord notification sent: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read recovery Discord sent result: %w", err)
	}
	if affected != 1 {
		return errors.New("recovery Discord outbox item is no longer sendable")
	}
	return nil
}

func (s *Store) MarkRecoveryDiscordFailed(
	ctx context.Context,
	eventID string,
	nextAttemptAt time.Time,
	lastError string,
) error {
	lastError = sanitizeRecoveryDiscordLastError(lastError)
	result, err := s.db.ExecContext(ctx, `UPDATE recovery_discord_outbox
		SET state = ?, attempt_count = attempt_count + 1,
			next_attempt_at_ms = ?, sent_at_ms = NULL, last_error = ?
		WHERE event_id = ? AND state IN (?, ?)`,
		recoveryDiscordStateRetry,
		nextAttemptAt.UTC().UnixMilli(),
		lastError,
		eventID,
		recoveryDiscordStatePending,
		recoveryDiscordStateRetry,
	)
	if err != nil {
		return fmt.Errorf("schedule recovery Discord retry: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read recovery Discord retry result: %w", err)
	}
	if affected != 1 {
		return errors.New("recovery Discord outbox item is no longer retryable")
	}
	return nil
}

func sanitizeRecoveryDiscordLastError(value string) string {
	switch strings.TrimSpace(value) {
	case discordErrorNetwork,
		discordErrorRateLimited,
		discordErrorServer,
		discordErrorWebhookNotFound,
		discordErrorForbidden,
		discordErrorInvalidConfiguration,
		discordErrorDeliveryFailed:
		return strings.TrimSpace(value)
	default:
		return discordErrorDeliveryFailed
	}
}

type RecoveryDiscordWorker struct {
	store             recoveryDiscordOutbox
	sender            RecoveryDiscordSender
	inspectProtection func(context.Context) system.RecoveryProtectionState
	location          *time.Location
	now               func() time.Time
	pollInterval      time.Duration
	errors            *errorReporter
}

func NewRecoveryDiscordWorker(
	store *Store,
	sender RecoveryDiscordSender,
	location *time.Location,
) *RecoveryDiscordWorker {
	if location == nil {
		location = time.UTC
	}
	return &RecoveryDiscordWorker{
		store:             store,
		sender:            sender,
		inspectProtection: inspectCurrentRecoveryProtection,
		location:          location,
		now:               time.Now,
		pollInterval:      defaultRecoveryWorkerPoll,
		errors:            newErrorReporter(time.Minute),
	}
}

func (w *RecoveryDiscordWorker) Run(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}
	for {
		if ctx.Err() != nil {
			return
		}
		processed := w.processOne(ctx)
		if ctx.Err() != nil {
			return
		}
		if processed {
			continue
		}
		timer := time.NewTimer(w.pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (w *RecoveryDiscordWorker) processOne(ctx context.Context) bool {
	now := w.now().UTC()
	work, err := w.store.NextRecoveryDiscord(ctx, now)
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		w.errors.report(
			"recovery_discord_outbox",
			"recovery Discord outbox read failed",
			err,
		)
		return false
	}
	if work == nil {
		return false
	}

	protection := unavailableRecoveryProtection()
	if w.inspectProtection != nil {
		protection = w.inspectProtection(ctx)
	}
	message, err := FormatRecoveryDiscordMessage(work.Incident, protection, w.location)
	if err == nil {
		if w.sender == nil {
			err = newDiscordDeliveryError(discordErrorInvalidConfiguration, 0)
		} else {
			err = w.sender.SendRecoveryDiscord(ctx, message)
		}
	}
	if err == nil {
		markContext, cancelMark := context.WithTimeout(
			context.Background(),
			recoveryDiscordStateTimeout,
		)
		markErr := w.store.MarkRecoveryDiscordSent(
			markContext,
			work.Incident.EventID,
			w.now().UTC(),
		)
		cancelMark()
		if markErr != nil {
			w.errors.report(
				"recovery_discord_sent_mark",
				"recovery Discord notification accepted but sent state could not be recorded",
				markErr,
			)
			return false
		}
		log.Print("recovery Discord notification sent")
		return true
	}
	if ctx.Err() != nil {
		return true
	}

	attempt := work.AttemptCount + 1
	delay := recoveryDiscordRetryDelay(attempt)
	category, retryAfter := recoveryDiscordFailureDetails(err)
	if retryAfter > delay {
		delay = retryAfter
	}
	nextAttempt := now.Add(delay)
	if markErr := w.store.MarkRecoveryDiscordFailed(
		ctx,
		work.Incident.EventID,
		nextAttempt,
		category,
	); markErr != nil {
		w.errors.report(
			"recovery_discord_retry_mark",
			"recovery Discord failure could not be scheduled",
			markErr,
		)
		return false
	}
	w.errors.report(
		"recovery_discord_delivery",
		"recovery Discord delivery failed; retry scheduled",
		nil,
	)
	return true
}

func recoveryDiscordFailureDetails(err error) (string, time.Duration) {
	var deliveryErr discordDeliveryError
	if errors.As(err, &deliveryErr) {
		return sanitizeRecoveryDiscordLastError(deliveryErr.category),
			boundedDiscordRetryAfter(deliveryErr.retryAfter)
	}
	return discordErrorDeliveryFailed, 0
}

func recoveryDiscordRetryDelay(attempt int) time.Duration {
	switch attempt {
	case 1:
		return time.Minute
	case 2:
		return 5 * time.Minute
	case 3:
		return 15 * time.Minute
	case 4:
		return 30 * time.Minute
	default:
		return time.Hour
	}
}

func inspectCurrentRecoveryProtection(
	ctx context.Context,
) system.RecoveryProtectionState {
	hardware, err := system.InspectHardwareWatchdogProtection(ctx)
	if err != nil || !system.ValidRecoveryProtectionState(hardware.State) {
		hardware = system.HardwareWatchdogProtectionState{
			State: system.RecoveryProtectionUnavailable,
		}
	}
	rtc, err := system.InspectRTCRecoveryProtection(ctx)
	if err != nil || !system.ValidRecoveryProtectionState(rtc.State) {
		rtc = system.RTCRecoveryProtectionState{
			State: system.RecoveryProtectionUnavailable,
		}
	}
	return system.RecoveryProtectionState{
		State:            system.AggregateRecoveryProtectionState(hardware.State, rtc.State),
		HardwareWatchdog: hardware,
		RTC:              rtc,
	}
}

func unavailableRecoveryProtection() system.RecoveryProtectionState {
	return system.RecoveryProtectionState{
		State: system.RecoveryProtectionUnavailable,
		HardwareWatchdog: system.HardwareWatchdogProtectionState{
			State: system.RecoveryProtectionUnavailable,
		},
		RTC: system.RTCRecoveryProtectionState{
			State: system.RecoveryProtectionUnavailable,
		},
	}
}

var _ RecoveryDiscordSender = (*DiscordWebhookSender)(nil)
