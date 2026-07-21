package application

import (
	"fmt"
	"testing"
)

type fakeVoiceCapture struct {
	started  bool
	stopped  bool
	canceled bool
	data     []byte
	err      error
}

func (f *fakeVoiceCapture) Start() error {
	f.started = true
	return f.err
}

func (f *fakeVoiceCapture) Stop() ([]byte, error) {
	f.stopped = true
	return f.data, f.err
}

func (f *fakeVoiceCapture) Cancel() error {
	f.canceled = true
	return f.err
}

func TestVoiceCaptureRequiresAdapter(t *testing.T) {
	if err := StartVoiceCapture(nil); err == nil {
		t.Fatal("StartVoiceCapture(nil) should fail")
	}
	if _, err := StopVoiceCapture(nil); err == nil {
		t.Fatal("StopVoiceCapture(nil) should fail")
	}
	if err := CancelVoiceCapture(nil); err != nil {
		t.Fatalf("CancelVoiceCapture(nil) = %v, want nil", err)
	}
}

func TestVoiceCaptureDelegatesToAdapter(t *testing.T) {
	capture := &fakeVoiceCapture{data: []byte("RIFFdata")}
	if err := StartVoiceCapture(capture); err != nil || !capture.started {
		t.Fatalf("start err=%v started=%v", err, capture.started)
	}
	data, err := StopVoiceCapture(capture)
	if err != nil || string(data) != "RIFFdata" || !capture.stopped {
		t.Fatalf("stop data=%q err=%v", data, err)
	}
	if err := CancelVoiceCapture(capture); err != nil || !capture.canceled {
		t.Fatalf("cancel err=%v canceled=%v", err, capture.canceled)
	}
}

func TestVoiceCapturePropagatesErrors(t *testing.T) {
	capture := &fakeVoiceCapture{err: fmt.Errorf("mic denied")}
	if err := StartVoiceCapture(capture); err == nil {
		t.Fatal("start should propagate adapter error")
	}
	if _, err := StopVoiceCapture(capture); err == nil {
		t.Fatal("stop should propagate adapter error")
	}
}
