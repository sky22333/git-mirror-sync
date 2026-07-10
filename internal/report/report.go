package report

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Status is the outcome of syncing one repository to one target.
type Status string

const (
	StatusOK      Status = "ok"
	StatusCreated Status = "created"
	StatusSkipped Status = "skipped"
	StatusFailed  Status = "failed"
)

// Item is one sync result line.
type Item struct {
	SourceRepo string
	Target     string
	Status     Status
	Message    string
	Duration   time.Duration
}

// Report aggregates sync results.
type Report struct {
	StartedAt time.Time
	Finished  time.Time
	Items     []Item
}

// Add appends a result item.
func (r *Report) Add(item Item) {
	r.Items = append(r.Items, item)
}

// Summary counts by status.
func (r *Report) Summary() (ok, created, skipped, failed int) {
	for _, it := range r.Items {
		switch it.Status {
		case StatusOK:
			ok++
		case StatusCreated:
			created++
		case StatusSkipped:
			skipped++
		case StatusFailed:
			failed++
		}
	}
	return
}

// Write prints a concise human-readable report.
func (r *Report) Write(w io.Writer) {
	ok, created, skipped, failed := r.Summary()
	elapsed := r.Finished.Sub(r.StartedAt)
	fmt.Fprintf(w, "\n======== Sync Report ========\n")
	fmt.Fprintf(w, "Duration : %s\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(w, "OK       : %d\n", ok)
	fmt.Fprintf(w, "Created  : %d\n", created)
	fmt.Fprintf(w, "Skipped  : %d\n", skipped)
	fmt.Fprintf(w, "Failed   : %d\n", failed)
	fmt.Fprintf(w, "-----------------------------\n")
	for _, it := range r.Items {
		msg := it.Message
		if msg != "" {
			msg = " — " + msg
		}
		fmt.Fprintf(w, "[%s] %s -> %s (%s)%s\n",
			strings.ToUpper(string(it.Status)),
			it.SourceRepo,
			it.Target,
			it.Duration.Round(time.Millisecond),
			msg,
		)
	}
	fmt.Fprintf(w, "=============================\n")
}

// Failed returns true if any item failed.
func (r *Report) Failed() bool {
	_, _, _, failed := r.Summary()
	return failed > 0
}
