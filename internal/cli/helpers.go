package cli

import (
	"encoding/json"
	"io"

	"github.com/grikomsn/codex-chat-manager/internal/session"
)

func resolveStore(codexHome string) (*session.Store, error) {
	cfg, err := session.ResolveConfig(codexHome)
	if err != nil {
		return nil, err
	}
	return session.NewStore(cfg), nil
}

func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
