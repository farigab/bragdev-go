// Package integration contains HTTP clients integrating with external services.
package integration

import (
	"io"
	"log"
)

// closeBody closes an io.Closer and logs any error.
func closeBody(c io.Closer) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		log.Printf("integration: failed to close body: %v", err)
	}
}
