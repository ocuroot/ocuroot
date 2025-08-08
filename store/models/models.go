package models

import (
	"net/url"
	"time"

	"github.com/ocuroot/ocuroot/refs"
	"github.com/ocuroot/ocuroot/sdk"
)

// Status represents the status of various entities
type Status string

// Common status constants
const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusComplete  Status = "complete"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Log represents a log entry for a function
type Log struct {
	ID          LogID    `json:"id"`
	FunctionRef refs.Ref `json:"function_ref"`
	sdk.Log
}

type Link struct {
	Text string  `json:"text"`
	URL  url.URL `json:"url"`
}

type Secret string

func NewMarker() Marker {
	return Marker{
		Time: time.Now(),
	}
}

// A generic marker for a ref without any additional content
type Marker struct {
	Time time.Time `json:"time"`
}
