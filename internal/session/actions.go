package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

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
			for _, targetID := range group.CascadesTo {
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
	_, records, err := s.ResolveTargets(ids)
	if err != nil {
		return ActionPlan{}, err
	}
	plan := ActionPlan{Type: ActionArchive, RequestedIDs: ids}
	for _, record := range records {
		target := ActionTarget{
			ID:       record.ID,
			Path:     record.Path,
			Status:   record.Status,
			ParentID: record.ParentID,
			IsChild:  record.ParentID != "",
		}
		if record.Status == StatusArchived {
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
		target.Path = dst
		plan.Targets = append(plan.Targets, target)
		plan.TargetIDs = append(plan.TargetIDs, record.ID)
	}
	return plan, nil
}

// Unarchive moves archived rollout files back into the dated active tree.
func (s *Store) Unarchive(ids []string) (ActionPlan, error) {
	_, records, err := s.ResolveTargets(ids)
	if err != nil {
		return ActionPlan{}, err
	}
	plan := ActionPlan{Type: ActionUnarchive, RequestedIDs: ids}
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
		target.Path = dst
		plan.Targets = append(plan.Targets, target)
		plan.TargetIDs = append(plan.TargetIDs, record.ID)
	}
	return plan, nil
}

// Delete removes archived rollout files and easy sidecar artifacts.
func (s *Store) Delete(ids []string) (ActionPlan, error) {
	_, records, err := s.ResolveTargets(ids)
	if err != nil {
		return ActionPlan{}, err
	}
	plan := ActionPlan{Type: ActionDelete, RequestedIDs: ids}

	deleteIDs := make(map[string]struct{})
	for _, record := range records {
		if record.Status != StatusArchived {
			plan.BlockedByActiveIDs = append(plan.BlockedByActiveIDs, record.ID)
			continue
		}
		deleteIDs[record.ID] = struct{}{}
	}
	if len(plan.BlockedByActiveIDs) > 0 {
		slices.Sort(plan.BlockedByActiveIDs)
		return plan, fmt.Errorf("delete blocked by active sessions: %s", strings.Join(plan.BlockedByActiveIDs, ", "))
	}

	for _, record := range records {
		if record.Status != StatusArchived {
			continue
		}
		if err := os.Remove(record.Path); err != nil && !os.IsNotExist(err) {
			return plan, fmt.Errorf("delete %s: %w", record.ID, err)
		}
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
	return plan, nil
}

// ResumeCmd returns the shell command used to resume an active session.
func (s *Store) ResumeCmd(record SessionRecord) (*exec.Cmd, error) {
	if record.Status != StatusActive {
		return nil, fmt.Errorf("session %s is archived; unarchive it before resuming", record.ID)
	}
	workdir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve current working directory: %w", err)
	}
	if record.CWD != "" {
		if info, err := os.Stat(record.CWD); err == nil && info.IsDir() {
			workdir = record.CWD
		}
	}
	cmd := exec.Command("codex", "resume", record.ID)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "CODEX_HOME="+s.cfg.CodexHome)
	return cmd, nil
}

// Resume executes codex resume for a single active session.
func (s *Store) Resume(ctx context.Context, id string) error {
	snapshot, err := s.LoadSnapshot()
	if err != nil {
		return err
	}
	record, ok := snapshot.RecordsByID[id]
	if !ok {
		return fmt.Errorf("unknown session id: %s", id)
	}
	cmd, err := s.ResumeCmd(record)
	if err != nil {
		return err
	}
	if ctx != nil {
		resumeCmd := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
		resumeCmd.Dir = cmd.Dir
		resumeCmd.Env = cmd.Env
		resumeCmd.Stdin = os.Stdin
		resumeCmd.Stdout = os.Stdout
		resumeCmd.Stderr = os.Stderr
		cmd = resumeCmd
	} else {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("resume session %s: %w", id, err)
	}
	return nil
}
