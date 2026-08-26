package agenttelemetry

import (
	"context"
	"sort"
	"time"
)

// UsageSample is one provider usage observation. Cumulative samples produce
// deltas only while the provider total remains monotonic.
type UsageSample struct {
	Provider, ID         string
	Total                int64
	Cumulative, Measured bool
	Segment              string
	Reset                bool
}
type UsageDelta struct {
	Total             int64
	Measured, Unknown bool
}
type UsageAccumulator struct {
	seen   map[string]bool
	totals map[string]int64
}

func (a *UsageAccumulator) Add(s UsageSample) UsageDelta {
	if a.seen == nil {
		a.seen, a.totals = map[string]bool{}, map[string]int64{}
	}
	key := s.Provider + "\x00" + s.ID
	if s.ID == "" || a.seen[key] {
		return UsageDelta{}
	}
	a.seen[key] = true
	if !s.Cumulative {
		return UsageDelta{Total: s.Total, Measured: s.Measured}
	}
	if s.Reset {
		a.totals[s.Provider] = 0
	}
	previous := a.totals[s.Provider]
	if s.Total < previous {
		return UsageDelta{Unknown: true}
	}
	a.totals[s.Provider] = s.Total
	return UsageDelta{Total: s.Total - previous, Measured: s.Measured}
}

// ActivityInterval is a measured span or an inferred observation window.
type ActivityInterval struct {
	Start, End time.Time
	Measured   bool
}
type ActivitySummary struct{ Wall, Measured, Inferred, Unknown time.Duration }

// SummarizeActivity unions measured spans first. Inference contributes only
// uncovered intervals up to idleCap, leaving the rest explicitly unknown.
func SummarizeActivity(start, end time.Time, intervals []ActivityInterval, idleCap time.Duration) ActivitySummary {
	out := ActivitySummary{Wall: end.Sub(start)}
	if !end.After(start) {
		return out
	}
	clip := func(i ActivityInterval) (ActivityInterval, bool) {
		if i.Start.Before(start) {
			i.Start = start
		}
		if i.End.After(end) {
			i.End = end
		}
		return i, i.End.After(i.Start)
	}
	var measured, inferred []ActivityInterval
	for _, i := range intervals {
		if i, ok := clip(i); ok {
			if i.Measured {
				measured = append(measured, i)
			} else {
				if i.End.Sub(i.Start) > idleCap {
					i.End = i.Start.Add(idleCap)
				}
				inferred = append(inferred, i)
			}
		}
	}
	union := func(xs []ActivityInterval) (time.Duration, []ActivityInterval) {
		xs = append([]ActivityInterval(nil), xs...)
		sort.Slice(xs, func(i, j int) bool { return xs[i].Start.Before(xs[j].Start) })
		var total time.Duration
		var merged []ActivityInterval
		for _, x := range xs {
			if len(merged) == 0 || x.Start.After(merged[len(merged)-1].End) {
				merged = append(merged, x)
				total += x.End.Sub(x.Start)
				continue
			}
			if x.End.After(merged[len(merged)-1].End) {
				total += x.End.Sub(merged[len(merged)-1].End)
				merged[len(merged)-1].End = x.End
			}
		}
		return total, merged
	}
	out.Measured, measured = union(measured)
	all, _ := union(append(measured, inferred...))
	out.Inferred = all - out.Measured
	out.Unknown = out.Wall - all
	return out
}

// ProjectEvents materializes one idempotent projection row per accepted event.
// The checkpoint advances in the same transaction as the uniqueness-protected rows.
func (s *Store) ProjectEvents(ctx context.Context, version string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var from int64
	if err := tx.QueryRowContext(ctx, `SELECT last_rowid FROM projector_checkpoints WHERE name='events' AND version=?`, version).Scan(&from); err != nil && err.Error() != "sql: no rows in result set" {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.rowid, e.id, e.run_id, e.event_type, e.payload, COALESCE(m.provider,''), COALESCE(m.interaction_id,'') FROM events e LEFT JOIN evidence_metadata m ON m.event_id=e.id WHERE e.rowid > ? ORDER BY e.rowid`, from)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rowid int64
		var e Event
		var payload string
		if err := rows.Scan(&rowid, &e.EventID, &e.RunID, &e.EventType, &payload, &e.Provider, &e.InteractionID); err != nil {
			return err
		}
		e.Payload = []byte(payload)
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO projected_events(source_event_id,projector_version) VALUES (?,?)`, e.EventID, version); err != nil {
			return err
		}
		if p := DecodeProjectionPayload(e); p.Usage != nil {
			total := p.Usage.Total
			if p.Usage.Cumulative {
				var previous int64
				err := tx.QueryRowContext(ctx, `SELECT total FROM usage_cumulative_states WHERE run_id=? AND provider=? AND interaction_id=? AND segment_id=? AND projector_version=?`, e.RunID, p.Usage.Provider, e.InteractionID, p.Usage.Segment, version).Scan(&previous)
				if err != nil && err.Error() != "sql: no rows in result set" {
					return err
				}
				if p.Usage.Total < previous {
					from = rowid
					continue
				}
				total -= previous
				if _, err := tx.ExecContext(ctx, `INSERT INTO usage_cumulative_states VALUES (?,?,?,?,?,?) ON CONFLICT(run_id,provider,interaction_id,segment_id,projector_version) DO UPDATE SET total=excluded.total`, e.RunID, p.Usage.Provider, e.InteractionID, p.Usage.Segment, version, p.Usage.Total); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO usage_projection_samples VALUES (?,?,?,?,?)`, e.RunID, p.Usage.Provider, version+"\x00"+p.Usage.ID, total, p.Usage.Measured); err != nil {
				return err
			}
		}
		if p := DecodeProjectionPayload(e); p.Activity != nil && !p.Activity.Interval.Start.IsZero() {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO activity_projection_samples VALUES (?,?,?,?)`, e.RunID, p.Activity.Interval.Start, p.Activity.Interval.End, p.Activity.Interval.Measured); err != nil {
				return err
			}
		}
		if p := DecodeProjectionPayload(e); p.Activity != nil && !p.Activity.Heartbeat.IsZero() {
			var previous time.Time
			err := tx.QueryRowContext(ctx, `SELECT observed_at FROM activity_heartbeat_samples WHERE run_id=? AND clock_id=? AND projector_version=? ORDER BY observed_at DESC LIMIT 1`, e.RunID, p.Activity.ClockID, version).Scan(&previous)
			if err == nil && p.Activity.Heartbeat.After(previous) {
				end := p.Activity.Heartbeat
				if end.Sub(previous) > 5*time.Minute {
					end = previous.Add(5 * time.Minute)
				}
				if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO activity_projection_samples VALUES (?,?,?,?)`, e.RunID, previous, end, false); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO activity_heartbeat_samples VALUES (?,?,?,?)`, e.RunID, p.Activity.ClockID, p.Activity.Heartbeat, version); err != nil {
				return err
			}
		}
		if p := DecodeProjectionPayload(e); p.Git != nil {
			g := p.Git
			if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO git_lifecycle_projection_samples VALUES (?,?,?,?,?,?,?,?,?,?,?)`, e.RunID, lifecyclePhase(e.EventType), version, g.Root, g.Head, g.Branch, g.Detached, g.Staged, g.Unstaged, g.Untracked, g.CapturedAt); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO git_projection_samples VALUES (?,?,?,?,?,?,?,?,?)`, e.RunID, g.Root, g.Head, g.Branch, g.Detached, g.Staged, g.Unstaged, g.Untracked, g.CapturedAt); err != nil {
				return err
			}
		}
		if s.projectEventsAfterRow != nil {
			if err := s.projectEventsAfterRow(rowid); err != nil {
				return err
			}
		}
		from = rowid
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO projector_checkpoints(name,version,last_rowid) VALUES ('events',?,?) ON CONFLICT(name,version) DO UPDATE SET last_rowid=excluded.last_rowid`, version, from)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// SaveTypedProjectionSamples persists narrow, replay-safe evidence projections.
func (s *Store) SaveTypedProjectionSamples(ctx context.Context, runID string, usage []UsageSample, activity []ActivityInterval, git GitSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, u := range usage {
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO usage_projection_samples VALUES (?,?,?,?,?)`, runID, u.Provider, u.ID, u.Total, u.Measured); err != nil {
			return err
		}
	}
	for _, a := range activity {
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO activity_projection_samples VALUES (?,?,?,?)`, runID, a.Start.UTC(), a.End.UTC(), a.Measured); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO git_projection_samples VALUES (?,?,?,?,?,?,?,?,?)`, runID, git.Root, git.Head, git.Branch, git.Detached, git.Staged, git.Unstaged, git.Untracked, git.CapturedAt.UTC())
	if err != nil {
		return err
	}
	return tx.Commit()
}
