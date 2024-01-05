package metal

import (
	"net/http"
	"strings"
)

// isNotFound check if an error is a 404 not found
func isNotFound(resp *http.Response, err error) bool {
	if err == nil {
		return false
	}
	if resp.StatusCode == http.StatusNotFound || strings.Contains(err.Error(), "Not Found") {
		return true
	}
	return false
}
