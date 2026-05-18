package acp

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

func TestBuildSeedSystemPrompt_AppendsGuard(t *testing.T) {
	seed := buildSeedSystemPrompt("role setup")
	if seed == "role setup" {
		t.Fatalf("expected guard to be appended")
	}
	if want := "protocol and role setup"; !strings.Contains(seed, want) {
		t.Fatalf("expected seed prompt to contain %q, got %q", want, seed)
	}
	if want := "Do NOT execute any commands"; !strings.Contains(seed, want) {
		t.Fatalf("expected seed prompt to contain %q, got %q", want, seed)
	}
	if want := "Wait for the next message"; !strings.Contains(seed, want) {
		t.Fatalf("expected seed prompt to contain %q, got %q", want, seed)
	}
}

// newManagerForSessionTest builds a Manager with the sessions map
// initialized, bypassing Create() (which needs a real ACP agent
// process). Lets unit tests exercise the session bookkeeping logic
// (NewSession map insert, EndSession map delete, Sessions snapshot,
// error paths) without spinning up an agent.
//
// The returned Manager has nil conn; methods that go through ACP
// (PromptSession etc.) return "agent not started".
func newManagerForSessionTest(t *testing.T) *Manager {
	t.Helper()
	return &Manager{
		logger:   slog.Default(),
		sessions: make(map[acp.SessionId]*sessionState),
	}
}

func TestSessions_EmptyWhenNoneRegistered(t *testing.T) {
	m := newManagerForSessionTest(t)
	if got := m.Sessions(); len(got) != 0 {
		t.Fatalf("expected empty sessions, got %v", got)
	}
}

func TestSessions_SnapshotIncludesRegistered(t *testing.T) {
	m := newManagerForSessionTest(t)
	m.sessions["sess-a"] = &sessionState{id: "sess-a", cwd: "/a"}
	m.sessions["sess-b"] = &sessionState{id: "sess-b", cwd: "/b"}
	m.sessionID = "sess-a"

	got := m.Sessions()
	if len(got) != 2 {
		t.Fatalf("expected 2 sessions, got %v", got)
	}
	strs := make([]string, len(got))
	for i, s := range got {
		strs[i] = string(s)
	}
	sort.Strings(strs)
	want := []string{"sess-a", "sess-b"}
	for i := range want {
		if strs[i] != want[i] {
			t.Fatalf("session %d: want %q got %q", i, want[i], strs[i])
		}
	}
}

func TestEndSession_RemovesFromMap(t *testing.T) {
	m := newManagerForSessionTest(t)
	m.sessionID = "initial"
	m.sessions["initial"] = &sessionState{id: "initial"}
	m.sessions["extra"] = &sessionState{id: "extra", cwd: "/x"}

	if err := m.EndSession("extra"); err != nil {
		t.Fatalf("end extra: %v", err)
	}
	if _, ok := m.sessions["extra"]; ok {
		t.Fatalf("extra session not removed from map")
	}
	if _, ok := m.sessions["initial"]; !ok {
		t.Fatalf("initial session unexpectedly removed")
	}
}

func TestEndSession_RefusesInitialSession(t *testing.T) {
	m := newManagerForSessionTest(t)
	m.sessionID = "initial"
	m.sessions["initial"] = &sessionState{id: "initial"}

	err := m.EndSession("initial")
	if err == nil {
		t.Fatalf("expected EndSession to refuse the initial session")
	}
	if !strings.Contains(err.Error(), "initial session") {
		t.Fatalf("expected error to mention initial session, got: %v", err)
	}
	if _, ok := m.sessions["initial"]; !ok {
		t.Fatalf("initial session removed despite error")
	}
}

func TestEndSession_UnknownSessionErrors(t *testing.T) {
	m := newManagerForSessionTest(t)
	m.sessionID = "initial"
	m.sessions["initial"] = &sessionState{id: "initial"}

	err := m.EndSession("never-existed")
	if err == nil {
		t.Fatalf("expected EndSession to error on unknown session")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected error to mention 'not found', got: %v", err)
	}
}

func TestPromptSession_RejectsBeforeAgentStarted(t *testing.T) {
	// Manager with no conn (Create never ran). PromptSession should
	// return a clear error rather than panic on nil conn — protects
	// callers that race or skip Create().
	m := newManagerForSessionTest(t)
	m.sessions["known"] = &sessionState{id: "known"}

	_, err := m.PromptSession(context.Background(), "known",
		[]acp.ContentBlock{acp.TextBlock("hi")})
	if err == nil {
		t.Fatalf("expected error from PromptSession with nil conn")
	}
	if !strings.Contains(err.Error(), "agent not started") {
		t.Fatalf("expected 'agent not started' error, got: %v", err)
	}
}

func TestCancelSession_RejectsBeforeAgentStarted(t *testing.T) {
	m := newManagerForSessionTest(t)
	m.sessions["known"] = &sessionState{id: "known"}

	err := m.CancelSession(context.Background(), "known")
	if err == nil {
		t.Fatalf("expected error from CancelSession with nil conn")
	}
	if !strings.Contains(err.Error(), "agent not started") {
		t.Fatalf("expected 'agent not started' error, got: %v", err)
	}
}
