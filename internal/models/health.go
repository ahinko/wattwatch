package models

import "time"

// HealthResponse represents the response from the health check endpoint
type HealthResponse struct {
	Status string    `json:"status" example:"healthy"`
	Time   time.Time `json:"time" example:"2024-03-20T13:00:00Z"`
}
