package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func gwsHelperWorkspace(t *testing.T, autoApprove bool, capture *[]string) *workspace {
	t.Helper()
	return &workspace{
		root:        t.TempDir(),
		control:     &fakeControl{},
		autoApprove: autoApprove,
		gwsExecFn: func(_ context.Context, _ string, argv []string, _ []string, _ time.Duration) (string, string, int, error) {
			if capture != nil {
				*capture = argv
			}
			return "{}", "", 0, nil
		},
	}
}

func TestGwsGmailInboxUsesTriage(t *testing.T) {
	var argv []string
	ws := gwsHelperWorkspace(t, true, &argv)
	ws.gwsExecFn = func(_ context.Context, _ string, a []string, _ []string, _ time.Duration) (string, string, int, error) {
		argv = a
		return `[{"subject":"Hi","from":"a@b.com"}]`, "", 0, nil
	}
	result, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.inbox", Args: `{"max":5,"query":"from:boss","labels":true}`})
	if err != nil {
		t.Fatalf("inbox error = %v", err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"gmail +triage", "--format json", "--max 5", "--query from:boss", "--labels"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv missing %q: %v", want, argv)
		}
	}
	if !strings.Contains(result.Result, "notice") || !strings.Contains(result.Result, "messages") {
		t.Fatalf("inbox should wrap with notice+messages: %s", result.Result)
	}
}

func TestGwsGmailInboxCachesAuthFailure(t *testing.T) {
	calls := 0
	ws := gwsHelperWorkspace(t, true, nil)
	ws.gwsExecFn = func(_ context.Context, _ string, _ []string, _ []string, _ time.Duration) (string, string, int, error) {
		calls++
		return "", "error[auth]: Gmail auth failed: No credentials found.", 2, nil
	}

	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.inbox", Args: `{}`}); err == nil ||
		!strings.Contains(err.Error(), "No credentials found") {
		t.Fatalf("first inbox error = %v", err)
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.inbox", Args: `{}`}); err == nil ||
		!strings.Contains(err.Error(), "already-open browser/Gmail tab") {
		t.Fatalf("cached inbox error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("gws calls = %d, want 1", calls)
	}
}

func TestGwsGmailSendBuildsFlagsAndConfirms(t *testing.T) {
	var argv []string
	ws := gwsHelperWorkspace(t, true, &argv)
	_, err := ws.awDispatch(nil, awArgs{
		Action: "gws.gmail.send",
		Args:   `{"to":["a@b.com","c@d.com"],"subject":"Hi","body":"Hello","cc":"e@f.com","html":true,"attach":["/tmp/x.pdf"]}`,
	})
	if err != nil {
		t.Fatalf("send error = %v", err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"gmail +send", "--to a@b.com,c@d.com", "--subject Hi", "--body Hello", "--cc e@f.com", "--html", "--attach /tmp/x.pdf"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("send argv missing %q: %v", want, argv)
		}
	}
}

func TestGwsGmailSendDeniedWithoutApproval(t *testing.T) {
	ws := &workspace{
		root:    t.TempDir(),
		control: &fakeControl{},
		confirm: func(context.Context, ConfirmRequest) (bool, error) { return false, nil },
		gwsExecFn: func(_ context.Context, _ string, _ []string, _ []string, _ time.Duration) (string, string, int, error) {
			t.Fatal("gws must not run when send is denied")
			return "", "", 0, nil
		},
	}
	_, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.send", Args: `{"to":"a@b.com","subject":"x","body":"y"}`})
	if err != ErrConfirmationDenied {
		t.Fatalf("expected denial, got %v", err)
	}
}

func TestGwsGmailSendDraftSkipsConfirmation(t *testing.T) {
	ran := false
	ws := &workspace{
		root:    t.TempDir(),
		control: &fakeControl{},
		confirm: func(context.Context, ConfirmRequest) (bool, error) {
			t.Fatal("draft should not require confirmation")
			return false, nil
		},
		gwsExecFn: func(_ context.Context, _ string, argv []string, _ []string, _ time.Duration) (string, string, int, error) {
			ran = true
			if !strings.Contains(strings.Join(argv, " "), "--draft") {
				t.Fatalf("expected --draft flag, got %v", argv)
			}
			return "{}", "", 0, nil
		},
	}
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.send", Args: `{"to":"a@b.com","subject":"x","body":"y","draft":true}`}); err != nil {
		t.Fatalf("draft send error = %v", err)
	}
	if !ran {
		t.Fatal("draft send should execute gws")
	}
}

func TestGwsGmailReplyAllUsesRightHelper(t *testing.T) {
	var argv []string
	ws := gwsHelperWorkspace(t, true, &argv)
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.reply", Args: `{"id":"m1","body":"ok","replyAll":true,"remove":"bob@x.com"}`}); err != nil {
		t.Fatalf("reply error = %v", err)
	}
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "gmail +reply-all") || !strings.Contains(joined, "--message-id m1") || !strings.Contains(joined, "--remove bob@x.com") {
		t.Fatalf("reply-all argv = %v", argv)
	}
}

func TestGwsGmailForwardRequiresRecipient(t *testing.T) {
	ws := gwsHelperWorkspace(t, true, nil)
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.gmail.forward", Args: `{"id":"m1"}`}); err == nil {
		t.Fatal("forward without to should error")
	}
}

func TestGwsCalendarAgendaReadOnly(t *testing.T) {
	var argv []string
	ws := gwsHelperWorkspace(t, false, &argv) // no auto-approve: agenda must not need it
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.calendar.agenda", Args: `{"week":true,"timezone":"America/Sao_Paulo"}`}); err != nil {
		t.Fatalf("agenda error = %v", err)
	}
	joined := strings.Join(argv, " ")
	if !strings.Contains(joined, "calendar +agenda") || !strings.Contains(joined, "--week") || !strings.Contains(joined, "--timezone America/Sao_Paulo") {
		t.Fatalf("agenda argv = %v", argv)
	}
}

func TestGwsCalendarInsertConfirms(t *testing.T) {
	var argv []string
	ws := gwsHelperWorkspace(t, true, &argv)
	if _, err := ws.awDispatch(nil, awArgs{
		Action: "gws.calendar.insert",
		Args:   `{"summary":"Standup","start":"2026-06-17T09:00:00-03:00","end":"2026-06-17T09:30:00-03:00","attendees":["a@b.com"],"meet":true}`,
	}); err != nil {
		t.Fatalf("insert error = %v", err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"calendar +insert", "--summary Standup", "--start 2026-06-17T09:00:00-03:00", "--attendee a@b.com", "--meet"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("insert argv missing %q: %v", want, argv)
		}
	}
}

func TestGwsDriveUploadConfirms(t *testing.T) {
	var argv []string
	ws := gwsHelperWorkspace(t, true, &argv)
	if _, err := ws.awDispatch(nil, awArgs{Action: "gws.drive.upload", Args: `{"file":"/tmp/report.pdf","parent":"FOLDER1","name":"Report.pdf"}`}); err != nil {
		t.Fatalf("upload error = %v", err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"drive +upload /tmp/report.pdf", "--parent FOLDER1", "--name Report.pdf"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("upload argv missing %q: %v", want, argv)
		}
	}
}

func TestGwsCallSupportsUploadAndPaging(t *testing.T) {
	var argv []string
	ws := gwsHelperWorkspace(t, true, &argv)
	if _, err := ws.awDispatch(nil, awArgs{
		Action: "gws.call",
		Args:   `{"service":"drive","resource":"files","method":"list","pageAll":true,"output":"/tmp/out.bin","upload":"/tmp/in.pdf"}`,
	}); err != nil {
		t.Fatalf("gws.call error = %v", err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--upload /tmp/in.pdf", "--output /tmp/out.bin", "--page-all"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("gws.call argv missing %q: %v", want, argv)
		}
	}
}

func TestGwsHelperActionsRegistered(t *testing.T) {
	ws := gwsHelperWorkspace(t, true, nil)
	listed, err := ws.awDispatch(nil, awArgs{Action: "aw.actions"})
	if err != nil {
		t.Fatalf("aw.actions error = %v", err)
	}
	for _, want := range []string{"gws.gmail.inbox", "gws.gmail.send", "gws.gmail.reply", "gws.gmail.forward", "gws.calendar.agenda", "gws.calendar.insert", "gws.drive.upload"} {
		if !strings.Contains(listed.Result, want) {
			t.Fatalf("aw.actions missing %q", want)
		}
	}
}
