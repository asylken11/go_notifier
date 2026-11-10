package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	appName    = "notifier"
	appVersion = "0.1.0"
)

type Sender interface {
	Send(Notification) error
}

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

type Server struct {
	notifications map[int]Notification
	nextID        int
	logger        *slog.Logger
	senders       map[string]Sender
}

type Notification struct {
	ID        int    `json:"id"`
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	IsUrgent  bool   `json:"urgent"`
}

type LoggingSender struct {
	Sender
	logger *slog.Logger
}

type ConsoleSender struct {
	w io.Writer
}
type EmailSender struct {
	w io.Writer
}
type TelegramSender struct{}

func (l LoggingSender) Send(n Notification) error {
	l.logger.Info("sending", "to", n.Recipient, "channel", n.Channel)

	err := l.Sender.Send(n)

	if err != nil {
		l.logger.Error("send failed", "to", n.Recipient, "error", err)
		return err
	}

	l.logger.Info("sent", "to", n.Recipient, "channel", n.Channel)
	return nil
}

func (c ConsoleSender) Send(n Notification) error {
	_, err := fmt.Fprintf(c.w, "[console] to %s | %s\n", n.Recipient, n.Subject)
	return err
}

func NewConsoleSender(w io.Writer) *ConsoleSender {
	if w == nil {
		w = os.Stdout
	}
	return &ConsoleSender{w: w}
}

func (e EmailSender) Send(n Notification) error {
	_, err := fmt.Fprintf(e.w, "[email] to %s | %s\n", n.Recipient, n.Subject)
	return err
}

func NewEmailSender(w io.Writer) *EmailSender {
	if w == nil {
		w = os.Stdout
	}
	return &EmailSender{w: w}
}

func (e TelegramSender) Send(n Notification) error {
	fmt.Printf("[telegram] to %s | %s\n", n.Recipient, n.Subject)
	return nil
}

func (n Notification) String() string {
	return fmt.Sprintf("Notification{to:%s, subject:%s, channel:%s}", n.Recipient, n.Subject, n.Channel)
}

func (n Notification) Format() string {
	return fmt.Sprintf("to: %s | subject: %s | body: %s | channel: %s | urgent: %v",
		n.Recipient, n.Subject, n.Body, n.Channel, n.IsUrgent)
}

func (n Notification) Send() (string, string) {
	message := n.Format()
	status := "queued"
	return message, status
}

func (n Notification) Validate() error {
	if n.Recipient == "" {
		return fmt.Errorf("recipient is required")
	}
	return nil
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status":"ok","version":"` + appVersion + `"}`))
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	var all []Notification
	for _, n := range s.notifications {
		all = append(all, n)
	}

	js, err := json.Marshal(all)
	if err != nil {
		s.logger.Error("marshal error", "error", err)
		return
	}

	w.Write(js)
}

func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
	var n Notification
	err := json.NewDecoder(r.Body).Decode(&n)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid JSON"}`))
		return
	}

	err = n.Validate()
	if err != nil {
		errResp := struct {
			Error string `json:"error"`
		}{
			Error: err.Error(),
		}

		js, _ := json.Marshal(errResp)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(js)
		return
	}

	sender, ok := s.senders[n.Channel]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"unknown channel"}`))
		return
	}

	s.nextID++
	n.ID = s.nextID
	s.notifications[n.ID] = n

	err = sender.Send(n)
	if err != nil {
		s.logger.Error("send failed", "error", err, "channel", n.Channel)
	}

	switch n.Channel {
	case "email":
		s.logger.Info("notification sent", "id", n.ID, "note", "check spam folder")
	case "telegram":
		s.logger.Info("notification sent", "id", n.ID, "note", "bot API used")
	default:
		s.logger.Info("notification sent", "id", n.ID)
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("marshal error", "error", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(js)
}

func (s *Server) getNotification(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid id"}`))
		return
	}

	n, ok := s.notifications[id]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"notification not found"}`))
		return
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("marshal error", "error", err)
		return
	}

	w.Write(js)
}

func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	})
}

func contentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func NewServer(logger *slog.Logger, senders map[string]Sender) (*Server, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("at least one sender is required")
	}

	return &Server{
		notifications: map[int]Notification{},
		logger:        logger,
		senders:       senders,
	}, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	senders := map[string]Sender{
		"console":  LoggingSender{Sender: NewConsoleSender(os.Stdout), logger: logger},
		"email":    LoggingSender{Sender: NewEmailSender(os.Stdout), logger: logger},
		"telegram": LoggingSender{Sender: TelegramSender{}, logger: logger},
	}

	s, err := NewServer(logger, senders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}

	s.nextID++
	s.notifications[s.nextID] = Notification{
		ID: s.nextID, Recipient: "user@example.com", Subject: "Deploy done",
		Body: "v1.2.3 deployed", Channel: "email",
	}
	s.nextID++
	s.notifications[s.nextID] = Notification{
		ID: s.nextID, Recipient: "ops-team@company.com", Subject: "Disk space warning",
		Body: "Server db-01: 92% used", Channel: "telegram", IsUrgent: true,
	}

	logger.Info("loaded notifications", "count", len(s.notifications), "first", s.notifications[1])

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.healthHandler)
	mux.HandleFunc("GET /api/notifications", s.listNotifications)
	mux.HandleFunc("POST /api/notifications", s.createNotification)
	mux.HandleFunc("GET /api/notifications/{id}", s.getNotification)

	logger.Info("starting server", "addr", ":8080")
	http.ListenAndServe(":8080", s.logRequest(contentType(mux)))
}
