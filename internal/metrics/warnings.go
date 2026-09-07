package metrics

import (
	"context"
	"fmt"

	"github.com/conradevans/ReactorLab/internal/history"
)

const (
	memoryWarningPercent      = 90.0
	diskWarningPercent        = 85.0
	temperatureWarningCelsius = 85.0
)

type warningCondition struct {
	Active          bool
	Source          string
	Subject         string
	Message         string
	RecoveryMessage string
	Fingerprint     string
}

func (s *Sampler) evaluateWarnings(
	ctx context.Context,
	batch history.Batch,
	deploymentsAvailable bool,
	databasesAvailable bool,
) error {
	for _, condition := range warningConditions(
		batch,
		deploymentsAvailable,
		databasesAvailable,
	) {
		if err := s.reconcileWarning(ctx, condition); err != nil {
			return err
		}
	}
	return nil
}

func warningConditions(
	batch history.Batch,
	deploymentsAvailable bool,
	databasesAvailable bool,
) []warningCondition {
	conditions := []warningCondition{
		{
			Active:          batch.System.MemoryUsagePercent >= memoryWarningPercent,
			Source:          "system",
			Subject:         "Memory usage",
			Message:         fmt.Sprintf("Memory usage is %.1f%%.", batch.System.MemoryUsagePercent),
			RecoveryMessage: "Memory usage returned below the warning threshold.",
			Fingerprint:     "system:memory",
		},
		{
			Active:          batch.System.DiskUsagePercent >= diskWarningPercent,
			Source:          "system",
			Subject:         "Disk usage",
			Message:         fmt.Sprintf("Disk usage is %.1f%%.", batch.System.DiskUsagePercent),
			RecoveryMessage: "Disk usage returned below the warning threshold.",
			Fingerprint:     "system:disk",
		},
		{
			Active:          batch.System.TemperatureCelsius >= temperatureWarningCelsius,
			Source:          "system",
			Subject:         "CPU temperature",
			Message:         fmt.Sprintf("CPU temperature is %.1f°C.", batch.System.TemperatureCelsius),
			RecoveryMessage: "CPU temperature returned below the warning threshold.",
			Fingerprint:     "system:temperature",
		},
		{
			Active:          !deploymentsAvailable,
			Source:          "minideploy",
			Subject:         "MiniDeploy metrics",
			Message:         "MiniDeploy monitoring data is unavailable.",
			RecoveryMessage: "MiniDeploy monitoring data is available again.",
			Fingerprint:     "source:minideploy",
		},
		{
			Active:          !databasesAvailable,
			Source:          "minibase",
			Subject:         "MiniBase metrics",
			Message:         "MiniBase monitoring data is unavailable.",
			RecoveryMessage: "MiniBase monitoring data is available again.",
			Fingerprint:     "source:minibase",
		},
	}

	for _, deployment := range batch.Deployments {
		conditions = append(conditions, warningCondition{
			Active:          deployment.Status != "healthy",
			Source:          "minideploy",
			Subject:         deployment.App,
			Message:         fmt.Sprintf("Deployment status is %s.", deployment.Status),
			RecoveryMessage: "Deployment returned to healthy.",
			Fingerprint:     "deployment:" + deployment.App + ":status",
		})

		if databasesAvailable {
			active := deployment.DatabaseState == "conflict" ||
				deployment.DatabaseState == "unresolved"
			conditions = append(conditions, warningCondition{
				Active:          active,
				Source:          "relationship",
				Subject:         deployment.App,
				Message:         fmt.Sprintf("Database relationship is %s.", deployment.DatabaseState),
				RecoveryMessage: "Database relationship resolved.",
				Fingerprint:     "deployment:" + deployment.App + ":database-relationship",
			})
		}
	}

	for _, database := range batch.Databases {
		conditions = append(conditions, warningCondition{
			Active:          database.Status != "ready",
			Source:          "minibase",
			Subject:         database.DisplayName,
			Message:         fmt.Sprintf("Database status is %s.", database.Status),
			RecoveryMessage: "Database returned to ready.",
			Fingerprint:     "database:" + database.ID + ":status",
		})

		if deploymentsAvailable {
			active := database.DeploymentState == "conflict" ||
				database.DeploymentState == "unresolved"
			conditions = append(conditions, warningCondition{
				Active:          active,
				Source:          "relationship",
				Subject:         database.DisplayName,
				Message:         fmt.Sprintf("Deployment relationship is %s.", database.DeploymentState),
				RecoveryMessage: "Deployment relationship resolved.",
				Fingerprint:     "database:" + database.ID + ":deployment-relationship",
			})
		}
	}

	return conditions
}

func (s *Sampler) reconcileWarning(
	ctx context.Context,
	condition warningCondition,
) error {
	previous, found, err := s.store.LatestActivity(
		ctx,
		condition.Fingerprint,
	)
	if err != nil {
		return err
	}

	if condition.Active {
		if found && previous.Kind == "warning" {
			return nil
		}
		return s.store.AppendActivity(ctx, history.ActivityEvent{
			OccurredAt:  s.now().UTC(),
			Source:      condition.Source,
			Kind:        "warning",
			Severity:    "warning",
			Subject:     condition.Subject,
			Message:     condition.Message,
			Fingerprint: condition.Fingerprint,
		})
	}

	if !found || previous.Kind != "warning" {
		return nil
	}

	return s.store.AppendActivity(ctx, history.ActivityEvent{
		OccurredAt:  s.now().UTC(),
		Source:      condition.Source,
		Kind:        "recovery",
		Severity:    "info",
		Subject:     condition.Subject,
		Message:     condition.RecoveryMessage,
		Fingerprint: condition.Fingerprint,
	})
}
