package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var (
	ErrSessionNotFound     = errors.New("session not found")
	ErrResumeIneligible    = errors.New("resume ineligible")
	ErrDeleteBlockedActive = errors.New("delete blocked by active sessions")
)

type DeleteBlockedActiveError struct {
	ActiveIDs []string
}

func (e *DeleteBlockedActiveError) Error() string {
	if len(e.ActiveIDs) == 0 {
		return ErrDeleteBlockedActive.Error()
	}
	if len(e.ActiveIDs) == 1 {
		return fmt.Sprintf("%s: %s", ErrDeleteBlockedActive.Error(), e.ActiveIDs[0])
	}
	return fmt.Sprintf("%s: %s", ErrDeleteBlockedActive.Error(), strings.Join(e.ActiveIDs, ", "))
}

func (e *DeleteBlockedActiveError) Is(target error) bool {
	return target == ErrDeleteBlockedActive
}

func (s *Store) resolveActionTargets(ids []string) (Snapshot, []SessionRecord, []ActionSkip, error) {
	snapshot, records, err := s.ResolveTargets(ids)
	if err != nil {
		return Snapshot{}, nil, nil, err
	}

	groupByID := make(map[string]struct{}, len(snapshot.Groups))
	for _, group := range snapshot.Groups {
		groupByID[group.Parent.ID] = struct{}{}
	}

	skipped := make([]ActionSkip, 0)
	seenMissing := make(map[string]struct{})
	for _, id := range ids {
		if _, ok := groupByID[id]; ok {
			continue
		}
		if _, ok := snapshot.RecordsByID[id]; ok {
			continue
		}
		if _, ok := seenMissing[id]; ok {
			continue
		}
		seenMissing[id] = struct{}{}
		skipped = append(skipped, ActionSkip{ID: id, Reason: "not found"})
	}

	return snapshot, records, skipped, nil
}

// ResolveTargets expands requested IDs into concrete rollout files.
func (s *Store) ResolveTargets(ids []string) (Snapshot, []SessionRecord, error) {
	snapshot, err := s.LoadSnapshot()
	if err != nil {
		return Snapshot{}, nil, err
	}
	groupByID := make(map[string]SessionGroup, len(snapshot.Groups))
	for _, group := range snapshot.Groups {
		groupByID[group.Parent.ID] = group
	}

	var resolved []SessionRecord
	seen := make(map[string]struct{})
	for _, id := range ids {
		if group, ok := groupByID[id]; ok {
			for _, targetID := range group.AllIDs() {
				record, ok := snapshot.RecordsByID[targetID]
				if !ok {
					continue
				}
				if _, exists := seen[targetID]; exists {
					continue
				}
				seen[targetID] = struct{}{}
				resolved = append(resolved, record)
			}
			continue
		}
		record, ok := snapshot.RecordsByID[id]
		if !ok {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		resolved = append(resolved, record)
	}
	return snapshot, resolved, nil
}

// Archive moves active rollout files to the archived root.
func (s *Store) Archive(ids []string) (ActionPlan, error) {
	slog.Info("archive operation started", "ids", ids, "action", "archive")
	_, records, skipped, err := s.resolveActionTargets(ids)
	if err != nil {
		return ActionPlan{}, err
	}
	plan := ActionPlan{Type: ActionArchive, RequestedIDs: ids, Skipped: skipped}
	for _, record := range records {
		target := ActionTarget{
			ID:       record.ID,
			Path:     record.Path,
			Status:   record.Status,
			ParentID: record.ParentID,
			IsChild:  record.ParentID != "",
		}
		if record.IsArchived() {
			slog.Debug("archive skipped", "id", record.ID, "reason", "already archived")
			plan.Skipped = append(plan.Skipped, ActionSkip{ID: record.ID, Path: record.Path, Reason: "already archived"})
			continue
		}
		dst := filepath.Join(s.cfg.ArchivedDir, filepath.Base(record.Path))
		if err := os.MkdirAll(s.cfg.ArchivedDir, 0o755); err != nil {
			return plan, fmt.Errorf("create archived sessions directory: %w", err)
		}
		if err := os.Rename(record.Path, dst); err != nil {
			return plan, fmt.Errorf("archive %s: %w", record.ID, err)
		}
		slog.Info("archive completed", "id", record.ID, "src", record.Path, "dst", dst, "action", "archive")
		target.Path = dst
		plan.Targets = append(plan.Targets, target)
		plan.TargetIDs = append(plan.TargetIDs, record.ID)
	}
	slog.Info("archive operation finished", "requested", len(ids), "processed", len(plan.Targets), "skipped", len(plan.Skipped), "action", "archive")
	s.InvalidateCache()
	return plan, nil
}

// Unarchive moves archived rollout files back into the dated active tree.
func (s *Store) Unarchive(ids []string) (ActionPlan, error) {
	slog.Info("unarchive operation started", "ids", ids, "action", "unarchive")
	_, records, skipped, err := s.resolveActionTargets(ids)
	if err != nil {
		return ActionPlan{}, err
	}
	plan := ActionPlan{Type: ActionUnarchive, RequestedIDs: ids, Skipped: skipped}
	now := time.Now()
	for _, record := range records {
		target := ActionTarget{
			ID:       record.ID,
			Path:     record.Path,
			Status:   record.Status,
			ParentID: record.ParentID,
			IsChild:  record.ParentID != "",
		}
		if record.Status == StatusActive {
			slog.Debug("unarchive skipped", "id", record.ID, "reason", "already active")
			plan.Skipped = append(plan.Skipped, ActionSkip{ID: record.ID, Path: record.Path, Reason: "already active"})
			continue
		}
		dstDir := filepath.Join(
			s.cfg.SessionsDir,
			record.CreatedAt.UTC().Format("2006"),
			record.CreatedAt.UTC().Format("01"),
			record.CreatedAt.UTC().Format("02"),
		)
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return plan, fmt.Errorf("create active sessions directory: %w", err)
		}
		dst := filepath.Join(dstDir, filepath.Base(record.Path))
		if err := os.Rename(record.Path, dst); err != nil {
			return plan, fmt.Errorf("unarchive %s: %w", record.ID, err)
		}
		if err := os.Chtimes(dst, now, now); err != nil {
			return plan, fmt.Errorf("bump unarchived mtime for %s: %w", record.ID, err)
		}
		slog.Info("unarchive completed", "id", record.ID, "src", record.Path, "dst", dst, "action", "unarchive")
		target.Path = dst
		plan.Targets = append(plan.Targets, target)
		plan.TargetIDs = append(plan.TargetIDs, record.ID)
	}
	slog.Info("unarchive operation finished", "requested", len(ids), "processed", len(plan.Targets), "skipped", len(plan.Skipped), "action", "unarchive")
	s.InvalidateCache()
	return plan, nil
}

// Delete removes rollout files and easy sidecar artifacts.
func (s *Store) Delete(ids []string) (ActionPlan, error) {
	slog.Info("delete operation started", "ids", ids, "action", "delete")
	_, records, skipped, err := s.resolveActionTargets(ids)
	if err != nil {
		return ActionPlan{}, err
	}
	plan := ActionPlan{Type: ActionDelete, RequestedIDs: ids, Skipped: skipped}

	blockedActiveIDs := make([]string, 0)
	deleteIDs := make(map[string]struct{})
	for _, record := range records {
		if record.Status == StatusActive {
			blockedActiveIDs = append(blockedActiveIDs, record.ID)
			continue
		}
		deleteIDs[record.ID] = struct{}{}
	}
	if len(blockedActiveIDs) > 0 {
		slices.Sort(blockedActiveIDs)
		plan.BlockedByActiveIDs = blockedActiveIDs
		slog.Warn("delete blocked by active sessions", "ids", blockedActiveIDs, "action", "delete")
		return plan, &DeleteBlockedActiveError{ActiveIDs: slices.Clone(blockedActiveIDs)}
	}

	for _, record := range records {
		if err := os.Remove(record.Path); err != nil && !os.IsNotExist(err) {
			return plan, fmt.Errorf("delete %s: %w", record.ID, err)
		}
		slog.Info("delete completed", "id", record.ID, "path", record.Path, "action", "delete")
		plan.Targets = append(plan.Targets, ActionTarget{
			ID:       record.ID,
			Path:     record.Path,
			Status:   record.Status,
			ParentID: record.ParentID,
			IsChild:  record.ParentID != "",
		})
		plan.TargetIDs = append(plan.TargetIDs, record.ID)

		snapshot := filepath.Join(s.cfg.ShellSnapshots, record.ID+".sh")
		if err := os.Remove(snapshot); err == nil {
			slog.Debug("snapshot removed", "id", record.ID, "path", snapshot, "action", "delete")
			plan.RemovedSnapshots = append(plan.RemovedSnapshots, snapshot)
		} else if err != nil && !os.IsNotExist(err) {
			return plan, fmt.Errorf("delete snapshot for %s: %w", record.ID, err)
		}
	}

	removed, err := rewriteIndex(s.cfg.SessionIndexPath, deleteIDs)
	if err != nil {
		return plan, err
	}
	plan.RemovedIndexRows = removed
	slog.Info("delete operation finished", "requested", len(ids), "processed", len(plan.Targets), "index_rows_removed", removed, "action", "delete")
	s.InvalidateCache()
	return plan, nil
}

// ResumeIntent resolves the machine-readable resume plan for a session.
func (s *Store) ResumeIntent(id string) (ResumeIntent, error) {
	snapshot, err := s.LoadSnapshot()
	if err != nil {
		return ResumeIntent{}, err
	}
	record, ok := snapshot.RecordsByID[id]
	if !ok {
		return ResumeIntent{
			RequestedID: id,
			Eligible:    false,
		}, fmt.Errorf("%w: unknown session id: %s", ErrSessionNotFound, id)
	}

	workdir, err := os.Getwd()
	if err != nil {
		return ResumeIntent{}, fmt.Errorf("resolve current working directory: %w", err)
	}
	if record.CWD != "" {
		if info, err := os.Stat(record.CWD); err == nil && info.IsDir() {
			workdir = record.CWD
		}
	}

	intent := ResumeIntent{
		RequestedID:      id,
		SessionID:        record.ID,
		Status:           record.Status,
		Eligible:         record.Status == StatusActive,
		WorkingDirectory: workdir,
		Executable:       "codex",
		Args:             []string{"resume", record.ID},
		EnvOverrides: map[string]string{
			EnvCodexHome: s.cfg.CodexHome,
		},
	}
	if !intent.Eligible {
		return intent, fmt.Errorf("%w: session %s is archived; unarchive it before resuming", ErrResumeIneligible, record.ID)
	}

	return intent, nil
}

func (s *Store) resumeCmd(intent ResumeIntent, ctx context.Context) *exec.Cmd {
	if ctx != nil {
		return exec.CommandContext(ctx, intent.Executable, intent.Args...)
	}
	return exec.Command(intent.Executable, intent.Args...)
}

func resumeEnvAssignments(overrides map[string]string) []string {
	assignments := make([]string, 0, len(overrides))
	for key, value := range overrides {
		assignments = append(assignments, key+"="+value)
	}
	slices.Sort(assignments)
	return assignments
}

// Resume executes codex resume for a single active session.
func (s *Store) Resume(ctx context.Context, id string) error {
	intent, err := s.ResumeIntent(id)
	if err != nil {
		return err
	}
	cmd := s.resumeCmd(intent, ctx)
	cmd.Dir = intent.WorkingDirectory
	cmd.Env = append(os.Environ(), resumeEnvAssignments(intent.EnvOverrides)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("resume session %s: %w", id, err)
	}
	return nil
}
