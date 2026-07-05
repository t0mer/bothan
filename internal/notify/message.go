package notify

import (
	"fmt"
	"strings"

	"github.com/t0mer/bothan/internal/model"
)

// formatMessage builds the notification text for a matched rule.
func formatMessage(host *model.Host, scan, prev *model.Scan, rule model.Rule) string {
	var b strings.Builder

	switch scan.Status {
	case model.ScanStatusError:
		fmt.Fprintf(&b, "❌ Bothan: scan FAILED for %s\n", host.Hostname)
	default:
		fmt.Fprintf(&b, "🔔 Bothan: %s — grade %s\n", host.Hostname, gradeOrNA(scan.OverallGrade))
	}

	fmt.Fprintf(&b, "Rule: %s (%s)\n", rule.Name, rule.ConditionType)

	if scan.Status == model.ScanStatusReady {
		if prev != nil && prev.OverallGrade != scan.OverallGrade {
			fmt.Fprintf(&b, "Grade: %s → %s\n", gradeOrNA(prev.OverallGrade), gradeOrNA(scan.OverallGrade))
		} else {
			fmt.Fprintf(&b, "Grade: %s\n", gradeOrNA(scan.OverallGrade))
		}
		for _, ep := range scan.Endpoints {
			line := fmt.Sprintf("  • %s: %s", ep.IPAddress, gradeOrNA(ep.Grade))
			if ep.CertNotAfter != nil {
				line += fmt.Sprintf(" (cert expires %s)", ep.CertNotAfter.Format("2006-01-02"))
			}
			b.WriteString(line + "\n")
		}
	} else if scan.ErrorMessage != "" {
		fmt.Fprintf(&b, "Error: %s\n", scan.ErrorMessage)
	}

	if scan.Trigger != "" {
		fmt.Fprintf(&b, "Trigger: %s\n", scan.Trigger)
	}
	fmt.Fprintf(&b, "Report: https://www.ssllabs.com/ssltest/analyze.html?d=%s\n", host.Hostname)
	return b.String()
}

func gradeOrNA(g string) string {
	if g == "" {
		return "n/a"
	}
	return g
}
