package main

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"
)

type MockSender struct {
	Calls []Notification
}

func (m *MockSender) Send(n Notification) error {
	m.Calls = append(m.Calls, n)
	return nil
}
func TestConsoleSender(t *testing.T) {
	var buf bytes.Buffer
	sender := NewConsoleSender(&buf)

	n := Notification{
		Recipient: "userTitle@example.com",
		Subject:   "Deploy Done",
		Channel:   "console",
	}
	err := sender.Send(n)
	if err != nil {
		t.Fatalf("Send() error: %s", err)
	}

	got := buf.String()
	want := "[console] to userTitle@example.com | Deploy Done\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

}
func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		n       Notification
		wantErr bool
	}{
		{
			name:    "valid notification",
			n:       Notification{Recipient: "userTESTFILE@example.com", Channel: "email"},
			wantErr: false,
		},
		{
			name:    "empty recipient",
			n:       Notification{Recipient: "", Channel: "email"},
			wantErr: true,
		},
		{
			name:    "empty channel",
			n:       Notification{Recipient: "user@example.com", Channel: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.n.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}
func TestHealthHandler (t *testing.T) {

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	senders := map [string]Sender["console": &MockSender{}]

	s, err := NewServer(logger, senders)
	if err != nil{
		t.Fatalf("NewServer() error: %s", err)
	} 
@allpeopleon
1 месяц назад


	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, r)

	if w.Code != 200 {
		t.Errorf("status= %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok"){
		t.Errorf("body = %q, want contains %q", w.Body.String(), "ok")
	}
}
func TestCreateNotification (t *testing.T){
test := [] struct{
	name string 
	n Notification
	wantCalls int  
}{
	name: "valid requests",
	body: '{"to":"user@example.com","subject": "Deploy done", "channel":"email"}',
	wantStatus: 201,
	wantCalls: 1, 	
}
} 