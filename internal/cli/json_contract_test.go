package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintJSONEnvelopeSchema(t *testing.T) {
	var stdout bytes.Buffer

	payload := struct {
		Value string `json:"value"`
	}{
		Value: "ok",
	}

	if err := printJSON(&stdout, listCmd, payload); err != nil {
		t.Fatalf("printJSON() error = %v", err)
	}

	want := `{
  "schema_version": "1",
  "command": "sessions list",
  "ok": true,
  "data": {
    "value": "ok"
  }
}
`

	if stdout.String() != want {
		t.Fatalf("unexpected JSON envelope:\nwant:\n%s\n\ngot:\n%s", want, stdout.String())
	}
}

func TestPrintJSONErrorSchema(t *testing.T) {
	var stdout bytes.Buffer

	details := struct {
		RequiredFlag string `json:"required_flag"`
	}{
		RequiredFlag: "yes",
	}

	err := printJSONError(&stdout, deleteCmd, jsonErrorInvalidRequest, errString("delete requires --yes to confirm"), details)
	if err != nil {
		t.Fatalf("printJSONError() error = %v", err)
	}

	want := `{
  "schema_version": "1",
  "command": "sessions delete",
  "ok": false,
  "error": {
    "code": "invalid_request",
    "message": "delete requires --yes to confirm",
    "details": {
      "required_flag": "yes"
    }
  }
}
`

	if stdout.String() != want {
		t.Fatalf("unexpected JSON error envelope:\nwant:\n%s\n\ngot:\n%s", want, stdout.String())
	}
	if strings.Contains(stdout.String(), `"data"`) {
		t.Fatalf("unexpected top-level data in error envelope: %s", stdout.String())
	}
}

type errString string

func (e errString) Error() string {
	return string(e)
}
