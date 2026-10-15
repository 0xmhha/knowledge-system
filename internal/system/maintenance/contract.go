// Package maintenance implements explicitly configured, scoped dataset jobs.
// Legacy maintenance and already-open reader pins are not changed by this API.
package maintenance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var commitPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{15,127}$`)

type Error struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
	Phase     string `json:"phase"`
}

func (e *Error) Error() string { return e.Code }
func reject(code string) error {
	return &Error{Code: code, Retryable: code == "writer_busy", Phase: "accepted"}
}

func ErrorResult(err error) Error {
	var value *Error
	if errors.As(err, &value) {
		return *value
	}
	return Error{Code: "operation_failed", Phase: "accepted"}
}

type SourcePin struct {
	Mode       string `json:"mode"`
	SnapshotID string `json:"snapshot_id"`
	BaseCommit string `json:"base_commit"`
}

type Base struct {
	State       string                  `json:"state"`
	Version     string                  `json:"version,omitempty"`
	Coordinates *contract.V2Coordinates `json:"coordinates,omitempty"`
}

type BuildRequest struct {
	SchemaVersion   int       `json:"schema_version"`
	ProjectID       string    `json:"project_id"`
	ProfileID       string    `json:"profile_id"`
	ProfileDigest   string    `json:"profile_digest"`
	SourcePin       SourcePin `json:"source_pin"`
	InputDigest     string    `json:"input_digest"`
	EmbeddingDigest string    `json:"embedding_digest"`
	Base            Base      `json:"base"`
	NewVersion      string    `json:"new_version"`
	Activation      string    `json:"activation"`
	IdempotencyKey  string    `json:"idempotency_key"`
	DeadlineSeconds int       `json:"deadline_seconds"`
}

type ProfileRequest struct {
	SchemaVersion int    `json:"schema_version"`
	ProjectID     string `json:"project_id"`
	ProfileID     string `json:"profile_id"`
}

type JobReference struct {
	SchemaVersion int    `json:"schema_version"`
	ProjectID     string `json:"project_id"`
	ProfileID     string `json:"profile_id"`
	JobID         string `json:"job_id"`
}

type ProfileContext struct {
	ProjectID         string    `json:"project_id"`
	ProfileID         string    `json:"profile_id"`
	ProfileDigest     string    `json:"profile_digest"`
	SourcePin         SourcePin `json:"source_pin"`
	InputDigest       string    `json:"input_digest"`
	EmbeddingDigest   string    `json:"embedding_digest"`
	ObservedBase      Base      `json:"observed_base"`
	ExistingVersions  []string  `json:"existing_versions"`
	AllowedActivation []string  `json:"allowed_activation"`
	RequiredGates     []string  `json:"required_gates"`
	TimeoutMaxSeconds int       `json:"timeout_max_seconds"`
}

type Capabilities struct {
	VersionedBuild   bool   `json:"versioned_build"`
	Cancel           bool   `json:"cancel"`
	JobPersistence   string `json:"job_persistence"`
	ServingAdoption  string `json:"serving_adoption"`
	InputPreparation string `json:"input_preparation"`
}

type Inspection struct {
	Profile      ProfileContext `json:"profile"`
	Capabilities Capabilities   `json:"capabilities"`
}

type Candidate struct {
	Version         string                 `json:"version"`
	Coordinates     contract.V2Coordinates `json:"coordinates"`
	InputDigest     string                 `json:"input_digest"`
	EmbeddingDigest string                 `json:"embedding_digest"`
}

type Publication struct {
	Base          Base                   `json:"base"`
	Version       string                 `json:"version"`
	Coordinates   contract.V2Coordinates `json:"coordinates"`
	PublishedAt   string                 `json:"published_at"`
	ReceiptDigest string                 `json:"receipt_digest"`
}

type JobStatus struct {
	SchemaVersion   int               `json:"schema_version"`
	JobID           string            `json:"job_id"`
	ProjectID       string            `json:"project_id"`
	ProfileID       string            `json:"profile_id"`
	RequestDigest   string            `json:"request_digest"`
	TargetVersion   string            `json:"target_version"`
	Revision        int               `json:"revision"`
	State           string            `json:"state"`
	Phase           string            `json:"phase"`
	Terminal        bool              `json:"terminal"`
	BuildComplete   bool              `json:"build_complete"`
	Activated       bool              `json:"activated"`
	Candidate       *Candidate        `json:"candidate"`
	Publication     *Publication      `json:"publication"`
	Gates           map[string]string `json:"gates"`
	Error           *Error            `json:"error"`
	ServingAdoption string            `json:"serving_adoption"`
}

type CancelResult struct {
	Decision string    `json:"decision"`
	Status   JobStatus `json:"status"`
}

func validVersion(value string) bool {
	return identifier.MatchString(value) && !strings.EqualFold(value, "current")
}

func validCoordinates(c contract.V2Coordinates) bool {
	return identifier.MatchString(c.ProjectID) && hashPattern.MatchString(c.DatasetID) &&
		hashPattern.MatchString(c.SnapshotID) && validPin(SourcePin{c.SourceMode, c.SnapshotID, c.BaseCommit})
}

func validPin(p SourcePin) bool {
	if !hashPattern.MatchString(p.SnapshotID) {
		return false
	}
	switch p.Mode {
	case "committed", "working-tree":
		return commitPattern.MatchString(p.BaseCommit)
	case "snapshot-only":
		return p.BaseCommit == ""
	default:
		return false
	}
}

func (r BuildRequest) Validate() error {
	if r.SchemaVersion != 1 || !identifier.MatchString(r.ProjectID) || !identifier.MatchString(r.ProfileID) ||
		!hashPattern.MatchString(r.ProfileDigest) || !hashPattern.MatchString(r.InputDigest) || !hashPattern.MatchString(r.EmbeddingDigest) ||
		!validPin(r.SourcePin) || !validVersion(r.NewVersion) || !keyPattern.MatchString(r.IdempotencyKey) ||
		r.DeadlineSeconds < 10 || r.DeadlineSeconds > 86400 ||
		(r.Activation != "hold_for_review" && r.Activation != "activate_after_gate") {
		return reject("invalid_request")
	}
	if r.Base.State == "absent" {
		if r.Base.Version != "" || r.Base.Coordinates != nil {
			return reject("invalid_request")
		}
	} else if r.Base.State != "pinned" || !validVersion(r.Base.Version) || r.Base.Coordinates == nil || !validCoordinates(*r.Base.Coordinates) {
		return reject("invalid_request")
	}
	return nil
}

// Decode checks duplicate keys before typed decoding, including nested objects.
func Decode(raw []byte, target any) error {
	if len(raw) > 64<<10 {
		return reject("invalid_request")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 64 {
			return reject("invalid_request")
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		if number, ok := token.(json.Number); ok && strings.ContainsAny(number.String(), ".eE") {
			return reject("invalid_request")
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					key, err := d.Token()
					if err != nil {
						return err
					}
					name, ok := key.(string)
					if !ok || seen[name] {
						return reject("invalid_request")
					}
					seen[name] = true
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
			default:
				return reject("invalid_request")
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := walk(0); err != nil {
		return reject("invalid_request")
	}
	if _, err := d.Token(); err != io.EOF {
		return reject("invalid_request")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return reject("invalid_request")
	}
	if err := requiredFields(raw, reflect.TypeOf(target)); err != nil {
		return err
	}
	if r, ok := target.(*BuildRequest); ok && r.Base.State == "absent" {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return reject("invalid_request")
		}
		var base map[string]json.RawMessage
		if err := json.Unmarshal(fields["base"], &base); err != nil || len(base) != 1 {
			return reject("invalid_request")
		}
	}
	return nil
}

func requiredFields(raw json.RawMessage, t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return reject("invalid_request")
	}
	allowed := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		tag := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		allowed[tag] = true
	}
	for name := range fields {
		if !allowed[name] {
			return reject("invalid_request")
		}
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		value, exists := fields[name]
		if !exists {
			if !strings.Contains(tag, ",omitempty") {
				return reject("invalid_request")
			}
			continue
		}
		if err := requiredFields(value, f.Type); err != nil {
			return err
		}
	}
	return nil
}

// Digest sorts object keys recursively to agree with the design contract.
func Digest(value any, domain string) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var object any
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", err
	}
	raw, err = json.Marshal(object)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(domain+"\x00"), raw...))
	return hex.EncodeToString(sum[:]), nil
}
