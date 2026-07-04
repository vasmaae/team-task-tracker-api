package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/sony/gobreaker"
)

type EmailService struct {
	endpoint string
	client   *http.Client
	breaker  *gobreaker.CircuitBreaker
}

func NewEmailService(endpoint string) *EmailService {
	return &EmailService{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 2 * time.Second},
		breaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "email-service",
			MaxRequests: 3,
			Timeout:     30 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}),
	}
}

func (s *EmailService) SendInvite(ctx context.Context, email string, teamID int64) error {
	_, err := s.breaker.Execute(func() (any, error) {
		if strings.HasPrefix(s.endpoint, "mock://") {
			slog.Info("mock invite email sent", "email", email, "team_id", teamID)
			return nil, nil
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("email", email)
		q.Set("team_id", strconv.FormatInt(teamID, 10))
		req.URL.RawQuery = q.Encode()
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return nil, errors.New("email service unavailable")
		}
		return nil, nil
	})
	return err
}

func (s *EmailService) SendVerificationCode(ctx context.Context, email, code string) error {
	_, err := s.breaker.Execute(func() (any, error) {
		if strings.HasPrefix(s.endpoint, "mock://") {
			slog.Info("mock verification email sent", "email", email, "code", code)
			return nil, nil
		}
		if strings.HasPrefix(s.endpoint, "smtp://") {
			addr := strings.TrimPrefix(s.endpoint, "smtp://")
			var msg bytes.Buffer
			msg.WriteString("From: Task Desk <noreply@taskdesk.local>\r\n")
			msg.WriteString("To: " + email + "\r\n")
			msg.WriteString("Subject: Код регистрации Task Desk\r\n")
			msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
			msg.WriteString("Ваш код регистрации: " + code + "\r\n")
			return nil, smtp.SendMail(addr, nil, "noreply@taskdesk.local", []string{email}, msg.Bytes())
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, nil)
		if err != nil {
			return nil, err
		}
		q := req.URL.Query()
		q.Set("email", email)
		q.Set("code", code)
		req.URL.RawQuery = q.Encode()
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			return nil, errors.New("email service unavailable")
		}
		return nil, nil
	})
	return err
}
