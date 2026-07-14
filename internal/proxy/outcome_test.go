package proxy

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseUsage(t *testing.T) {
	p, c := parseUsage([]byte(`{"usage":{"prompt_tokens":11,"completion_tokens":22}}`))
	if p != 11 || c != 22 {
		t.Fatalf("got %d/%d", p, c)
	}
	p, c = parseUsage([]byte(`{"id":"x"}`))
	if p != 0 || c != 0 {
		t.Fatalf("empty usage: %d/%d", p, c)
	}
}

func TestEnsureStreamIncludeUsage(t *testing.T) {
	out := ensureStreamIncludeUsage([]byte(`{"model":"m","stream":true}`))
	if !strings.Contains(string(out), `"include_usage":true`) {
		t.Fatalf("missing include_usage: %s", out)
	}
	// preserves existing opts
	out = ensureStreamIncludeUsage([]byte(`{"stream_options":{"include_usage":false,"foo":1}}`))
	s := string(out)
	if !strings.Contains(s, `"include_usage":true`) {
		t.Fatalf("should force true: %s", s)
	}
	if !strings.Contains(s, `"foo":1`) {
		t.Fatalf("should keep foo: %s", s)
	}
}

func TestUsageCaptureWriter(t *testing.T) {
	w := httptest.NewRecorder()
	u := &usageCaptureWriter{ResponseWriter: w}
	chunk := "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"
	chunk += "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":3}}\n\n"
	chunk += "data: [DONE]\n\n"
	if _, err := u.Write([]byte(chunk)); err != nil {
		t.Fatal(err)
	}
	if u.prompt != 5 || u.completion != 3 {
		t.Fatalf("tokens=%d/%d", u.prompt, u.completion)
	}
}

func TestResultStatusStreamError(t *testing.T) {
	r := forwardResult{status: 200, err: errDummy, wrote: true}
	if got := resultStatus(r); got != 502 {
		t.Fatalf("status=%d want 502", got)
	}
	r = forwardResult{status: 502, err: errDummy, wrote: true}
	if got := resultStatus(r); got != 502 {
		t.Fatalf("status=%d", got)
	}
}

func TestAppendAttemptStreamError(t *testing.T) {
	out := &Outcome{}
	r := forwardResult{status: 200, err: errDummy, wrote: true}
	appendAttempt(out, "oa", "gpt", "k1", r, time.Now())
	if out.Status != 502 {
		t.Fatalf("status=%d", out.Status)
	}
	if out.ErrorClass != "down" {
		t.Fatalf("class=%s", out.ErrorClass)
	}
	if len(out.Attempts) != 1 || out.Attempts[0].Status != 502 {
		t.Fatalf("attempt=%+v", out.Attempts)
	}
}

func TestFinalizeStreamOutcome(t *testing.T) {
	out := &Outcome{Status: 200, Attempts: []Attempt{{Status: 200}}}
	finalizeStreamOutcome(out, forwardResult{status: 200, err: errDummy, wrote: true})
	if out.Status != 502 || out.ErrorClass != "down" {
		t.Fatalf("out=%+v", out)
	}
	if out.Attempts[0].Status != 502 || out.Attempts[0].FailClass != "down" {
		t.Fatalf("attempt=%+v", out.Attempts[0])
	}
}

var errDummy = errString("stream cut")

type errString string

func (e errString) Error() string { return string(e) }
