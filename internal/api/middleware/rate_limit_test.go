package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"wattwatch/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Define RateLimit struct type to match config
	type RateLimit struct {
		Requests int `envconfig:"RATE_LIMIT_REQUESTS" default:"1000"`
		Window   int `envconfig:"RATE_LIMIT_WINDOW" default:"60"`
		Burst    int `envconfig:"RATE_LIMIT_BURST" default:"50"`
	}

	tests := []struct {
		name          string
		config        config.Config
		requests      int
		expectedCodes []int
		timeBetween   time.Duration
		clientIP      string
		description   string
	}{
		{
			name: "Normal usage - under limit",
			config: config.Config{
				RateLimit: RateLimit{
					Requests: 10,
					Window:   1,
					Burst:    10,
				},
			},
			requests:      3,
			expectedCodes: []int{200, 200, 200},
			timeBetween:   50 * time.Millisecond,
			clientIP:      "192.168.1.1",
			description:   "Should allow requests under the rate limit",
		},
		{
			name: "At rate limit",
			config: config.Config{
				RateLimit: RateLimit{
					Requests: 2,
					Window:   1,
					Burst:    2,
				},
			},
			requests:      2,
			expectedCodes: []int{200, 200},
			timeBetween:   10 * time.Millisecond,
			clientIP:      "192.168.1.2",
			description:   "Should allow requests up to the limit",
		},
		{
			name: "Exceeds rate limit",
			config: config.Config{
				RateLimit: RateLimit{
					Requests: 2,
					Window:   1,
					Burst:    2,
				},
			},
			requests:      3,
			expectedCodes: []int{200, 200, 429},
			timeBetween:   10 * time.Millisecond,
			clientIP:      "192.168.1.3",
			description:   "Should block requests that exceed the rate limit",
		},
		{
			name: "Different IPs - separate limits",
			config: config.Config{
				RateLimit: RateLimit{
					Requests: 1,
					Window:   1,
					Burst:    1,
				},
			},
			requests:      1, // We'll make one request per IP
			expectedCodes: []int{200},
			timeBetween:   10 * time.Millisecond,
			clientIP:      "192.168.1.4",
			description:   "Should track rate limits separately for different IPs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new rate limiter with test configuration
			limiter := NewRateLimiter(&tt.config)

			// Create a test router
			router := gin.New()
			router.Use(limiter.Middleware())

			// Add a test endpoint
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			// For the "Different IPs" test, make requests from different IPs
			if tt.name == "Different IPs - separate limits" {
				// First request from IP1
				req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
				req1.Header.Set("X-Forwarded-For", "192.168.1.4")
				w1 := httptest.NewRecorder()
				router.ServeHTTP(w1, req1)
				assert.Equal(t, 200, w1.Code, "First IP request should succeed")

				time.Sleep(tt.timeBetween)

				// Second request from IP2
				req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
				req2.Header.Set("X-Forwarded-For", "192.168.1.5")
				w2 := httptest.NewRecorder()
				router.ServeHTTP(w2, req2)
				assert.Equal(t, 200, w2.Code, "Second IP request should succeed")
				return
			}

			// Normal test case handling
			for i := 0; i < tt.requests; i++ {
				// Create a new request
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				req.Header.Set("X-Forwarded-For", tt.clientIP) // Set the client IP

				// Create a response recorder
				w := httptest.NewRecorder()

				// Process the request
				router.ServeHTTP(w, req)

				// Check if the response code matches expected
				assert.Equal(t, tt.expectedCodes[i], w.Code,
					"Request %d: expected status %d but got %d",
					i+1, tt.expectedCodes[i], w.Code)

				// Wait between requests
				time.Sleep(tt.timeBetween)
			}
		})
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	type RateLimit struct {
		Requests int `envconfig:"RATE_LIMIT_REQUESTS" default:"1000"`
		Window   int `envconfig:"RATE_LIMIT_WINDOW" default:"60"`
		Burst    int `envconfig:"RATE_LIMIT_BURST" default:"50"`
	}

	cfg := &config.Config{
		RateLimit: RateLimit{
			Requests: 10,
			Window:   1,
			Burst:    10,
		},
	}

	limiter := NewRateLimiter(cfg)

	// Override cleanup duration for testing
	limiter.cleanup = 100 * time.Millisecond

	// Create some test limiters
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}
	for _, ip := range ips {
		limiter.getLimiter(ip)
	}

	// Verify limiters were created
	assert.Equal(t, len(ips), len(limiter.limiters), "Expected limiters to be created")

	// Wait for cleanup
	time.Sleep(150 * time.Millisecond)

	// Verify cleanup occurred
	assert.Equal(t, 0, len(limiter.limiters), "Expected limiters to be cleaned up")
}
