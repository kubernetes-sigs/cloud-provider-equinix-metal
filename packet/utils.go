package packet

import (
	"net/http"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"
)

func getFacilityID(client *packngo.Client) (string, error) {
	facility, err := deviceFacility()
	if err != nil {
		return "", errors.Wrap(err, "failed to get facility from device metadata")
	}

	return getFacilityIDFromFacilityName(client, facility)
}

func getFacilityIDFromFacilityName(client *packngo.Client, facilityName string) (string, error) {
	facilities, resp, err := client.Facilities.List()
	if err != nil {
		if resp.StatusCode == http.StatusForbidden {
			return "", errors.New("cannot get facilityID, access denied to search facilities")
		}

		return "", errors.Wrap(err, "cannot get facilityID")
	}

	for _, facility := range facilities {
		if facility.Code == facilityName {
			return facility.ID, nil
		}
	}

	return "", errors.New("FacilityID cannot be found")
}
