package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/KOFI-GYIMAH/github-monitor/pkg/logger"
)

type ErrorLevel int

const (
	LevelFatal ErrorLevel = iota + 1
	LevelError
	LevelWarning
	LevelInfo
)

func (l ErrorLevel) String() string {
	return [...]string{"", "Fatal", "Error", "Warning", "Info"}[l]
}

type ApplicationError struct {
	Reference   string
	Title       string
	Detail      string
	RootCause   error
	Level       ErrorLevel
	OccurredAt  time.Time
	CallerTrace []string
}

func (e *ApplicationError) Error() string {
	var b strings.Builder

	fmt.Fprintf(&b, "[%s][%s] %s", e.OccurredAt.Format(time.RFC3339), e.Reference, e.Title)

	if e.Detail != "" {
		fmt.Fprintf(&b, " - %s", e.Detail)
	}

	if e.RootCause != nil {
		fmt.Fprintf(&b, " (caused by: %v)", e.RootCause)
	}

	return b.String()
}

func (e *ApplicationError) Unwrap() error {
	return e.RootCause
}

func New(ref, title, detail string, cause error, level ErrorLevel) *ApplicationError {
	return &ApplicationError{
		Reference:   ref,
		Title:       title,
		Detail:      detail,
		RootCause:   cause,
		Level:       level,
		OccurredAt:  time.Now().UTC(),
		CallerTrace: captureCallerInfo(3),
	}
}

func Wrap(ref, title, detail string, cause error, level ErrorLevel) *ApplicationError {
	return New(ref, title, detail, cause, level)
}

func captureCallerInfo(skip int) []string {
	pc := make([]uintptr, 10)
	n := runtime.Callers(skip, pc)
	if n == 0 {
		return nil
	}

	pc = pc[:n]
	frames := runtime.CallersFrames(pc)

	var trace []string
	for {
		frame, more := frames.Next()
		trace = append(trace, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		if !more {
			break
		}
	}

	return trace
}

type HTTPErrorResponse struct {
	Status     int       `json:"status"`
	ErrorRef   string    `json:"error_reference,omitempty"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail,omitempty"`
	Resolution string    `json:"resolution,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

func WriteHTTPError(w http.ResponseWriter, err error) {
	var appErr *ApplicationError

	resp := HTTPErrorResponse{
		Status:    http.StatusInternalServerError,
		Title:     "An unexpected error occurred",
		Timestamp: time.Now().UTC(),
	}

	if errors.As(err, &appErr) {
		resp.ErrorRef = appErr.Reference
		resp.Title = appErr.Title
		resp.Detail = appErr.Detail

		switch appErr.Level {
		case LevelFatal:
			resp.Status = http.StatusInternalServerError
			resp.Resolution = "Please contact support with the error reference"
		case LevelError:
			resp.Status = http.StatusBadRequest
		case LevelWarning:
			resp.Status = http.StatusConflict
			resp.Resolution = "Please review your request and try again"
		case LevelInfo:
			resp.Status = http.StatusOK
		}
	} else {
		resp.Detail = err.Error()
	}

	logger.Error("%v", err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	json.NewEncoder(w).Encode(resp)
}
