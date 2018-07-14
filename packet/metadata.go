package packet

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
)

const (
	packetMetadataURL = "https://metadata.packet.net/2009-04-04/meta-data"
	facilityMetadata  = "facility"
)

// deviceFacility returns the facility of the currently device
func deviceFacility() (string, error) {
	return httpGet(fmt.Sprintf("%s/%s", packetMetadataURL, facilityMetadata))
}

// httpGet is for getting different metadata about the machine
func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("device metadata returned non 200 status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
