package packet

import (
	"github.com/packethost/packngo"
)

// isNotFound check if an error is a 404 not found
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if perr, ok := err.(*packngo.ErrorResponse); ok {
		if perr.Response == nil {
			return false
		}
		return perr.Response.StatusCode == 404
	}
	return false
}
