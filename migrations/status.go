package migrations

import "fmt"

type MigrationDescriptor struct {
	Version uint   `json:"version"`
	Name    string `json:"name"`
}

type StatusReport struct {
	Available      []MigrationDescriptor `json:"available"`
	CurrentVersion *uint                 `json:"current_version,omitempty"`
	LatestVersion  uint                  `json:"latest_version"`
	Pending        []MigrationDescriptor `json:"pending"`
	AppliedCount   int                   `json:"applied_count"`
	PendingCount   int                   `json:"pending_count"`
	Dirty          bool                  `json:"dirty"`
	UnknownCurrent bool                  `json:"unknown_current"`
}

func buildStatusReport(available []availableMigration, currentVersion uint, dirty bool, hasVersion bool) StatusReport {
	descriptors := make([]MigrationDescriptor, 0, len(available))
	versionSet := make(map[uint]struct{}, len(available))
	pending := make([]MigrationDescriptor, 0)

	latest := uint(0)
	for _, m := range available {
		d := MigrationDescriptor{Version: m.Version, Name: m.Name}
		descriptors = append(descriptors, d)
		versionSet[m.Version] = struct{}{}
		if m.Version > latest {
			latest = m.Version
		}
	}

	report := StatusReport{
		Available:     descriptors,
		LatestVersion: latest,
		Dirty:         dirty,
	}

	if !hasVersion {
		report.Pending = append(report.Pending, descriptors...)
		report.AppliedCount = 0
		report.PendingCount = len(report.Pending)
		return report
	}

	v := currentVersion
	report.CurrentVersion = &v

	if _, ok := versionSet[currentVersion]; !ok {
		report.UnknownCurrent = true
	}

	appliedCount := 0
	for _, d := range descriptors {
		if d.Version <= currentVersion {
			appliedCount++
			continue
		}
		pending = append(pending, d)
	}

	report.AppliedCount = appliedCount
	report.Pending = pending
	report.PendingCount = len(pending)
	return report
}

func (s StatusReport) SummaryString() string {
	current := "nil"
	if s.CurrentVersion != nil {
		current = fmt.Sprintf("%d", *s.CurrentVersion)
	}
	return fmt.Sprintf(
		"available=%d applied=%d pending=%d current=%s latest=%d dirty=%t unknown_current=%t",
		len(s.Available),
		s.AppliedCount,
		s.PendingCount,
		current,
		s.LatestVersion,
		s.Dirty,
		s.UnknownCurrent,
	)
}
