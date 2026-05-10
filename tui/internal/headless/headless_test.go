package headless

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWriteJSON_WritesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"status": "ok"}
	WriteJSON(&buf, data)
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nbody: %s", err, buf.String())
	}
	if parsed["status"] != "ok" {
		t.Errorf("status = %q, want %q", parsed["status"], "ok")
	}
}

func TestWriteJSON_TrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	WriteJSON(&buf, map[string]string{"a": "b"})
	s := buf.String()
	if s[len(s)-1] != '\n' {
		t.Error("output should end with newline")
	}
}

func TestWriteError_WritesErrorAndCode(t *testing.T) {
	var buf bytes.Buffer
	WriteError(&buf, "something broke", "test_error")
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if parsed["error"] != "something broke" {
		t.Errorf("error = %q, want %q", parsed["error"], "something broke")
	}
	if parsed["code"] != "test_error" {
		t.Errorf("code = %q, want %q", parsed["code"], "test_error")
	}
}
