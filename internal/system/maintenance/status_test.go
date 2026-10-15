package maintenance

import (
	"encoding/json"
	"testing"
)

func TestStoredStatusAgreesWithDesignFixtures(t *testing.T) {
	for _, fixture := range designFixtures(t) {
		if fixture.Kind != "status" && fixture.Kind != "transition" && fixture.Kind != "observation" {
			continue
		}
		t.Run(fixture.ID, func(t *testing.T) {
			var err error
			switch fixture.Kind {
			case "status":
				var data struct {
					Status  json.RawMessage
					Request BuildRequest
					Profile ProfileContext
				}
				if json.Unmarshal(fixture.Data, &data) != nil {
					t.Fatal("invalid fixture")
				}
				var status JobStatus
				if Decode(data.Status, &status) != nil {
					err = reject("invalid_status")
				} else {
					err = ValidateStatus(status, data.Request, data.Profile)
				}
			case "transition":
				var data struct{ Before, After string }
				if json.Unmarshal(fixture.Data, &data) != nil {
					t.Fatal("invalid fixture")
				}
				err = validateTransition(data.Before, data.After)
			case "observation":
				var data struct{ Before, After JobStatus }
				if json.Unmarshal(fixture.Data, &data) != nil {
					t.Fatal("invalid fixture")
				}
				err = validateJournalUpdate(data.Before, data.After)
			}
			actual := "accept"
			if err != nil {
				actual = ErrorResult(err).Code
			}
			if actual != fixture.Expected {
				t.Fatalf("want %s, got %s", fixture.Expected, actual)
			}
		})
	}
}
