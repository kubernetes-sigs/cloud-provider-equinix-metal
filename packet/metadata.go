package packet

import (
	"github.com/packethost/packngo/metadata"
)

// GetAndParseMetadata retrieve metadata from a specific URL or Packet's standard
func GetAndParseMetadata(u string) (*metadata.CurrentDevice, error) {
	if u == "" {
		return metadata.GetMetadata()
	}
	return metadata.GetMetadataFromURL(u)
}
