package middleware

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	// Skip compression for these content types
	excludedContentTypes = []string{
		"image/",
		"video/",
		"audio/",
	}
)

// CompressionConfig holds configuration for the compression middleware
type CompressionConfig struct {
	// Minimum content length to trigger compression (default: 1KB)
	MinLength int
	// Gzip compression level (1-9, higher = better compression but slower)
	Level int
}

// DefaultCompressionConfig returns the default compression configuration
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		MinLength: 1024, // 1KB
		Level:     gzip.DefaultCompression,
	}
}

// shouldCompress checks if the response should be compressed based on content type
func shouldCompress(contentType string) bool {
	// Skip compression for excluded content types
	for _, excluded := range excludedContentTypes {
		if strings.HasPrefix(contentType, excluded) {
			return false
		}
	}
	return true
}

// Compression returns a middleware that compresses HTTP responses using gzip compression
func Compression(cfg CompressionConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request is compressed
		if c.Request.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(c.Request.Body)
			if err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(reader)
			if err != nil {
				reader.Close()
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			reader.Close()
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}

		// Check if client accepts gzip for response
		if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// Replace writer with gzip writer
		gzipWriter := &gzipResponseWriter{
			ResponseWriter: c.Writer,
			minLength:      cfg.MinLength,
			level:          cfg.Level,
			contentBuf:     new(bytes.Buffer),
		}
		c.Writer = gzipWriter

		// Add Vary header to prevent caching issues
		c.Header("Vary", "Accept-Encoding")

		c.Next()

		// Ensure everything is written
		gzipWriter.finishWriting()
	}
}

type gzipResponseWriter struct {
	gin.ResponseWriter
	writer     *gzip.Writer
	minLength  int
	level      int
	contentBuf *bytes.Buffer
}

func (g *gzipResponseWriter) Write(data []byte) (int, error) {
	// Write to buffer first
	return g.contentBuf.Write(data)
}

func (g *gzipResponseWriter) finishWriting() error {
	contentType := g.Header().Get("Content-Type")
	content := g.contentBuf.Bytes()
	shouldGzip := shouldCompress(contentType) && len(content) >= g.minLength

	if shouldGzip {
		gz, err := gzip.NewWriterLevel(g.ResponseWriter, g.level)
		if err != nil {
			return err
		}
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Del("Content-Length")

		_, err = gz.Write(content)
		if err != nil {
			gz.Close()
			return err
		}

		return gz.Close()
	}

	_, err := g.ResponseWriter.Write(content)
	return err
}

func (g *gzipResponseWriter) WriteString(s string) (int, error) {
	return g.Write([]byte(s))
}

// Implement other required interfaces
func (g *gzipResponseWriter) CloseNotify() <-chan bool {
	return g.ResponseWriter.CloseNotify()
}

func (g *gzipResponseWriter) Flush() {
	if g.writer != nil {
		g.writer.Flush()
	}
	g.ResponseWriter.Flush()
}

func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return g.ResponseWriter.Hijack()
}

func (g *gzipResponseWriter) Size() int {
	return g.ResponseWriter.Size()
}

func (g *gzipResponseWriter) Written() bool {
	return g.ResponseWriter.Written()
}

func (g *gzipResponseWriter) WriteHeaderNow() {
	g.ResponseWriter.WriteHeaderNow()
}

func (g *gzipResponseWriter) Status() int {
	return g.ResponseWriter.Status()
}
