package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCompression(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		acceptEncoding    string
		contentType       string
		responseSize      int
		expectCompression bool
	}{
		{
			name:              "Should compress JSON response",
			acceptEncoding:    "gzip",
			contentType:       "application/json",
			responseSize:      2048,
			expectCompression: true,
		},
		{
			name:              "Should not compress small response",
			acceptEncoding:    "gzip",
			contentType:       "application/json",
			responseSize:      512,
			expectCompression: false,
		},
		{
			name:              "Should not compress when client doesn't accept gzip",
			acceptEncoding:    "",
			contentType:       "application/json",
			responseSize:      2048,
			expectCompression: false,
		},
		{
			name:              "Should not compress image",
			acceptEncoding:    "gzip",
			contentType:       "image/jpeg",
			responseSize:      2048,
			expectCompression: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(Compression(DefaultCompressionConfig()))

			// Create test response data
			data := strings.Repeat("a", tt.responseSize)

			r.GET("/test", func(c *gin.Context) {
				c.Header("Content-Type", tt.contentType)
				c.String(http.StatusOK, data)
			})

			// Create test request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}

			// Perform request
			r.ServeHTTP(w, req)

			// Check response
			assert.Equal(t, http.StatusOK, w.Code)

			if tt.expectCompression {
				assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))

				// Decompress response
				reader, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
				assert.NoError(t, err)
				defer reader.Close()

				decompressed, err := io.ReadAll(reader)
				assert.NoError(t, err)
				assert.Equal(t, data, string(decompressed))
			} else {
				assert.NotEqual(t, "gzip", w.Header().Get("Content-Encoding"))
				assert.Equal(t, data, w.Body.String())
			}
		})
	}
}

func TestCompressedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(Compression(DefaultCompressionConfig()))

	// Handler that echoes request body
	r.POST("/test", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		assert.NoError(t, err)
		c.String(http.StatusOK, string(body))
	})

	// Create test data and compress it
	data := "test data"
	var compressedBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&compressedBuf)
	_, err := gzWriter.Write([]byte(data))
	assert.NoError(t, err)
	err = gzWriter.Close()
	assert.NoError(t, err)

	// Create test request with compressed body
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader(compressedBuf.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	// Perform request
	r.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	// Decompress response if it's compressed
	var responseBody string
	if w.Header().Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
		assert.NoError(t, err)
		defer reader.Close()

		decompressed, err := io.ReadAll(reader)
		assert.NoError(t, err)
		responseBody = string(decompressed)
	} else {
		responseBody = w.Body.String()
	}

	assert.Equal(t, data, responseBody)
}

func TestInvalidCompressedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(Compression(DefaultCompressionConfig()))

	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach here")
	})

	// Create invalid compressed data
	invalidData := []byte("not gzipped data")

	// Create test request with invalid compressed body
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", bytes.NewReader(invalidData))
	req.Header.Set("Content-Encoding", "gzip")

	// Perform request
	r.ServeHTTP(w, req)

	// Check response
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
