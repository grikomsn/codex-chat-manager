package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type indexEntry struct {
	ID        string `json:"id"`
	Thread    string `json:"thread_name"`
	UpdatedAt string `json:"updated_at"`
}

type indexState struct {
	Titles map[string]string
	Lines  []indexEntry
}

func loadIndex(path string) (indexState, error) {
	state := indexState{Titles: make(map[string]string)}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, fmt.Errorf("open session index: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)
	for scanner.Scan() {
		var entry indexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.ID == "" {
			continue
		}
		state.Lines = append(state.Lines, entry)
		if entry.Thread != "" {
			state.Titles[entry.ID] = entry.Thread
		}
	}
	if err := scanner.Err(); err != nil {
		return state, fmt.Errorf("scan session index: %w", err)
	}
	return state, nil
}

func rewriteIndex(path string, ids map[string]struct{}) (int, error) {
	state, err := loadIndex(path)
	if err != nil {
		return 0, err
	}

	kept := make([]indexEntry, 0, len(state.Lines))
	removed := 0
	for _, line := range state.Lines {
		if _, ok := ids[line.ID]; ok {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return 0, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("rewrite session index: %w", err)
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, entry := range kept {
		data, err := json.Marshal(entry)
		if err != nil {
			return 0, fmt.Errorf("marshal session index entry: %w", err)
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			return 0, fmt.Errorf("write session index entry: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		return 0, fmt.Errorf("flush session index: %w", err)
	}
	return removed, nil
}
