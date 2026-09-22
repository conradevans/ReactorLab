package minideploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	maxMetadataResponseBytes = 2 << 20
	maxMetadataDeployments   = 500
	maxHistoryVersions       = 100
)

var (
	ErrDeploymentNotFound = errors.New("deployment not found")

	applicationIDPattern = regexp.MustCompile(
		`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`,
	)
	gitObjectIDPattern = regexp.MustCompile(
		`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`,
	)
	imageIDPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	databaseIDPattern = regexp.MustCompile(
		`^database_[0-9a-f]{32}$`,
	)
	sourceProviderPattern   = regexp.MustCompile(`^[a-z0-9.-]{1,64}$`)
	sourceRepositoryPattern = regexp.MustCompile(
		`^[A-Za-z0-9_.-]{1,100}/[A-Za-z0-9_.-]{1,100}$`,
	)
)

type Source struct {
	Provider     string `json:"provider,omitempty"`
	Repository   string `json:"repository,omitempty"`
	Branch       string `json:"branch,omitempty"`
	RequestedRef string `json:"requestedRef,omitempty"`
	CommitSHA    string `json:"commitSha"`
}

type ServiceMetadata struct {
	Name     string `json:"name"`
	Strategy string `json:"strategy"`
	Status   string `json:"status"`
	ImageID  string `json:"imageId,omitempty"`
}

type DatabaseAttachment struct {
	DatabaseID  string `json:"databaseId"`
	DisplayName string `json:"displayName"`
	BindingName string `json:"bindingName"`
}

type DeploymentMetadata struct {
	App                 string               `json:"app"`
	Strategy            string               `json:"strategy"`
	Status              string               `json:"status"`
	ImageID             string               `json:"imageId,omitempty"`
	Source              *Source              `json:"source,omitempty"`
	ActivatedAt         *time.Time           `json:"activatedAt,omitempty"`
	Services            []ServiceMetadata    `json:"services"`
	DatabaseAttachments []DatabaseAttachment `json:"databaseAttachments"`
}

type DeploymentVersion struct {
	App         string            `json:"app"`
	Strategy    string            `json:"strategy"`
	ImageID     string            `json:"imageId,omitempty"`
	Source      *Source           `json:"source,omitempty"`
	ActivatedAt *time.Time        `json:"activatedAt,omitempty"`
	ArchivedAt  time.Time         `json:"archivedAt"`
	Services    []ServiceMetadata `json:"services"`
}

type DeploymentHistory struct {
	App      string              `json:"app"`
	Versions []DeploymentVersion `json:"versions"`
}

type deploymentHistoryEnvelope struct {
	App      string                         `json:"app"`
	Versions []deploymentHistoryVersionWire `json:"versions"`
}

type deploymentHistoryVersionWire struct {
	App         string            `json:"app"`
	Strategy    string            `json:"strategy"`
	ImageID     string            `json:"imageId"`
	Source      *Source           `json:"source"`
	ActivatedAt *time.Time        `json:"activatedAt"`
	DeployedAt  time.Time         `json:"deployedAt"`
	Services    []ServiceMetadata `json:"services"`
}

func (c *Client) DeploymentMetadata(
	ctx context.Context,
) ([]DeploymentMetadata, error) {
	var deployments []DeploymentMetadata
	if err := c.getManagementJSON(ctx, "/deployments", &deployments); err != nil {
		return nil, err
	}
	if deployments == nil {
		return nil, fmt.Errorf("validate MiniDeploy metadata: missing deployments")
	}
	if len(deployments) > maxMetadataDeployments {
		return nil, fmt.Errorf("validate MiniDeploy metadata: too many deployments")
	}
	for index := range deployments {
		if err := validateDeploymentMetadata(&deployments[index]); err != nil {
			return nil, fmt.Errorf("validate MiniDeploy metadata: %w", err)
		}
	}
	return deployments, nil
}

func (c *Client) DeploymentHistory(
	ctx context.Context,
	app string,
) (DeploymentHistory, error) {
	if !ValidApplicationID(app) {
		return DeploymentHistory{}, fmt.Errorf("validate MiniDeploy history: invalid app")
	}
	var envelope deploymentHistoryEnvelope
	err := c.getManagementJSON(
		ctx,
		"/deployments/"+app+"/history",
		&envelope,
	)
	if errors.Is(err, ErrDeploymentNotFound) {
		return DeploymentHistory{}, err
	}
	if err != nil {
		return DeploymentHistory{}, err
	}
	if envelope.App != app || envelope.Versions == nil {
		return DeploymentHistory{}, fmt.Errorf(
			"validate MiniDeploy history: invalid envelope",
		)
	}
	if len(envelope.Versions) > maxHistoryVersions {
		return DeploymentHistory{}, fmt.Errorf(
			"validate MiniDeploy history: too many versions",
		)
	}

	versions := make([]DeploymentVersion, 0, len(envelope.Versions))
	for _, raw := range envelope.Versions {
		version := DeploymentVersion{
			App:         raw.App,
			Strategy:    raw.Strategy,
			ImageID:     raw.ImageID,
			Source:      raw.Source,
			ActivatedAt: raw.ActivatedAt,
			ArchivedAt:  raw.DeployedAt,
			Services:    raw.Services,
		}
		if err := validateDeploymentVersion(app, &version); err != nil {
			return DeploymentHistory{}, fmt.Errorf(
				"validate MiniDeploy history: %w",
				err,
			)
		}
		versions = append(versions, version)
	}
	return DeploymentHistory{App: app, Versions: versions}, nil
}

func (c *Client) getManagementJSON(
	ctx context.Context,
	path string,
	target any,
) error {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+path,
		nil,
	)
	if err != nil {
		return fmt.Errorf("build MiniDeploy metadata request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request MiniDeploy metadata: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return ErrDeploymentNotFound
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"MiniDeploy metadata returned status %d",
			response.StatusCode,
		)
	}

	payload, err := io.ReadAll(io.LimitReader(
		response.Body,
		maxMetadataResponseBytes+1,
	))
	if err != nil {
		return fmt.Errorf("read MiniDeploy metadata: %w", err)
	}
	if len(payload) > maxMetadataResponseBytes {
		return fmt.Errorf("decode MiniDeploy metadata: response too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode MiniDeploy metadata: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return fmt.Errorf("decode MiniDeploy metadata: %w", err)
	}
	return nil
}

func ValidApplicationID(value string) bool {
	return applicationIDPattern.MatchString(value)
}

func validateDeploymentMetadata(deployment *DeploymentMetadata) error {
	if !ValidApplicationID(deployment.App) ||
		!validBoundedText(deployment.Strategy, 128, false) ||
		!validBoundedText(deployment.Status, 128, false) {

		return fmt.Errorf("invalid deployment")
	}
	if err := validateImageID(deployment.ImageID); err != nil {
		return err
	}
	if err := validateSource(deployment.Source); err != nil {
		return err
	}
	if err := normalizeOptionalTime(deployment.ActivatedAt); err != nil {
		return err
	}
	if deployment.Services == nil {
		deployment.Services = []ServiceMetadata{}
	}
	if len(deployment.Services) > 20 {
		return fmt.Errorf("too many deployment services")
	}
	for index := range deployment.Services {
		if err := validateService(&deployment.Services[index]); err != nil {
			return err
		}
	}
	if deployment.DatabaseAttachments == nil {
		deployment.DatabaseAttachments = []DatabaseAttachment{}
	}
	if len(deployment.DatabaseAttachments) > 20 {
		return fmt.Errorf("too many database attachments")
	}
	for _, attachment := range deployment.DatabaseAttachments {
		if !databaseIDPattern.MatchString(attachment.DatabaseID) ||
			!validBoundedText(attachment.DisplayName, 256, true) ||
			!validBoundedText(attachment.BindingName, 64, false) {

			return fmt.Errorf("invalid database attachment")
		}
	}
	return nil
}

func validateDeploymentVersion(
	app string,
	version *DeploymentVersion,
) error {
	if version.App != app || !ValidApplicationID(version.App) ||
		!validBoundedText(version.Strategy, 128, true) {

		return fmt.Errorf("invalid deployment version")
	}
	if version.ArchivedAt.IsZero() {
		return fmt.Errorf("invalid deployment archive time")
	}
	version.ArchivedAt = version.ArchivedAt.UTC()
	if err := validateImageID(version.ImageID); err != nil {
		return err
	}
	if err := validateSource(version.Source); err != nil {
		return err
	}
	if err := normalizeOptionalTime(version.ActivatedAt); err != nil {
		return err
	}
	if version.Services == nil {
		version.Services = []ServiceMetadata{}
	}
	if len(version.Services) > 20 {
		return fmt.Errorf("too many deployment services")
	}
	for index := range version.Services {
		if err := validateService(&version.Services[index]); err != nil {
			return err
		}
	}
	return nil
}

func validateService(service *ServiceMetadata) error {
	if !validBoundedText(service.Name, 128, false) ||
		!validBoundedText(service.Strategy, 128, true) ||
		!validBoundedText(service.Status, 128, true) {

		return fmt.Errorf("invalid deployment service")
	}
	return validateImageID(service.ImageID)
}

func validateSource(source *Source) error {
	if source == nil {
		return nil
	}
	if !gitObjectIDPattern.MatchString(source.CommitSHA) {
		return fmt.Errorf("invalid deployment source commit")
	}
	if !validBoundedText(source.Branch, 1024, true) ||
		!validBoundedText(source.RequestedRef, 1024, true) {

		return fmt.Errorf("invalid deployment source ref")
	}
	if source.Provider == "" && source.Repository == "" {
		return nil
	}
	if !sourceProviderPattern.MatchString(source.Provider) ||
		!sourceRepositoryPattern.MatchString(source.Repository) {

		return fmt.Errorf("invalid deployment source repository")
	}
	return nil
}

func validateImageID(imageID string) error {
	if imageID != "" && !imageIDPattern.MatchString(imageID) {
		return fmt.Errorf("invalid deployment image identity")
	}
	return nil
}

func normalizeOptionalTime(value *time.Time) error {
	if value == nil {
		return nil
	}
	if value.IsZero() {
		return fmt.Errorf("invalid deployment timestamp")
	}
	normalized := value.UTC()
	*value = normalized
	return nil
}

func validBoundedText(value string, maximum int, allowEmpty bool) bool {
	if (!allowEmpty && strings.TrimSpace(value) == "") ||
		len(value) > maximum {

		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
