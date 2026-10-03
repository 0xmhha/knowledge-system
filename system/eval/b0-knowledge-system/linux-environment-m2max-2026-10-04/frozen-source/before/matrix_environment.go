package evalcli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/config"
)

// Snapshots are outside query timing. They are observations, not a proof of
// exclusive resources or human approval; quality_metrics stays null.
type matrixEnvironment struct {
	At          time.Time              `json:"at"`
	Valid       bool                   `json:"valid"`
	Hardware    matrixHardware         `json:"hardware"`
	Model       matrixModelObservation `json:"model"`
	Errors      []string               `json:"errors,omitempty"`
	Limitations []string               `json:"limitations"`
}
type matrixHardware struct {
	OS              string          `json:"os"`
	Arch            string          `json:"arch"`
	OSVersion       string          `json:"os_version"`
	CPUBrand        string          `json:"cpu_brand"`
	LogicalCPUs     int             `json:"logical_cpus"`
	MemoryBytes     int64           `json:"memory_bytes"`
	Load            string          `json:"load"`
	MemoryState     string          `json:"memory_state"`
	Swap            string          `json:"swap"`
	Thermal         string          `json:"thermal,omitempty"`
	ProcessCount    int             `json:"process_count"`
	TopCPUProcesses []matrixProcess `json:"top_cpu_processes"`
	Unavailable     []string        `json:"unavailable,omitempty"`
}
type matrixProcess struct {
	PID        int     `json:"pid"`
	ParentPID  int     `json:"parent_pid"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes   int64   `json:"rss_bytes"`
}
type matrixModelObservation struct {
	Provider       string          `json:"provider"`
	Model          string          `json:"model"`
	Digest         string          `json:"digest,omitempty"`
	Dimension      int             `json:"dimension"`
	Endpoint       string          `json:"endpoint,omitempty"`
	ServerVersion  string          `json:"server_version,omitempty"`
	RuntimeOptions map[string]int  `json:"runtime_options,omitempty"`
	Valid          bool            `json:"valid"`
	ProbePolicy    string          `json:"probe_policy"`
	Residency      json.RawMessage `json:"residency,omitempty"`
	Errors         []string        `json:"errors,omitempty"`
}
type matrixEnvironmentProbe func(context.Context, *config.Config, *setup.DatasetIdentity) matrixEnvironment
type matrixInputCopy struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	SHA256After string `json:"sha256_after,omitempty"`
}

func copyMatrixInputs(out string, extra []string, locked map[string]string) (map[string]matrixInputCopy, error) {
	copies := map[string]matrixInputCopy{}
	for _, source := range extra {
		absolute, err := filepath.Abs(source)
		if err != nil {
			return copies, err
		}
		if _, exists := copies[absolute]; exists {
			continue
		}
		file, err := os.Open(absolute)
		if err != nil {
			return copies, err
		}
		data, err := io.ReadAll(io.LimitReader(file, 8*1024*1024+1))
		_ = file.Close()
		if err != nil {
			return copies, err
		}
		if len(data) > 8*1024*1024 {
			return copies, fmt.Errorf("environment lock-file exceeds 8 MiB: %s", absolute)
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		if hash != locked[absolute] {
			return copies, fmt.Errorf("lock-file changed before copy: %s", absolute)
		}
		relative := filepath.Join("locked-inputs", fmt.Sprintf("%03d-%s", len(copies), filepath.Base(absolute)))
		if err := os.MkdirAll(filepath.Dir(filepath.Join(out, relative)), 0700); err != nil {
			return copies, err
		}
		if err := os.WriteFile(filepath.Join(out, relative), data, 0600); err != nil {
			return copies, err
		}
		copies[absolute] = matrixInputCopy{Path: relative, SHA256: hash}
	}
	return copies, nil
}

func environmentCommand(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil || len(data) > 64*1024 {
		return ""
	}
	return strings.TrimSpace(string(data))
}
func parseMatrixProcesses(raw string) ([]matrixProcess, int) {
	all := []matrixProcess{}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 4 {
			continue
		}
		pid, e1 := strconv.Atoi(fields[0])
		parent, e2 := strconv.Atoi(fields[1])
		cpu, e3 := strconv.ParseFloat(fields[2], 64)
		rss, e4 := strconv.ParseInt(fields[3], 10, 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || pid <= 0 || parent < 0 || cpu < 0 || rss < 0 || math.IsNaN(cpu) || math.IsInf(cpu, 0) {
			continue
		}
		all = append(all, matrixProcess{PID: pid, ParentPID: parent, CPUPercent: cpu, RSSBytes: rss * 1024})
	}
	count := len(all)
	sort.Slice(all, func(i, j int) bool {
		if all[i].CPUPercent == all[j].CPUPercent {
			return all[i].PID < all[j].PID
		}
		return all[i].CPUPercent > all[j].CPUPercent
	})
	if len(all) > 8 {
		all = all[:8]
	}
	return all, count
}
func observeMatrixHardware() matrixHardware {
	h := matrixHardware{OS: runtime.GOOS, Arch: runtime.GOARCH, LogicalCPUs: runtime.NumCPU()}
	if runtime.GOOS == "darwin" {
		h.OSVersion = environmentCommand("sw_vers", "-productVersion")
		h.CPUBrand = environmentCommand("sysctl", "-n", "machdep.cpu.brand_string")
		h.MemoryBytes, _ = strconv.ParseInt(environmentCommand("sysctl", "-n", "hw.memsize"), 10, 64)
		h.Load = environmentCommand("sysctl", "-n", "vm.loadavg")
		h.MemoryState = environmentCommand("vm_stat")
		h.Swap = environmentCommand("sysctl", "-n", "vm.swapusage")
		h.Thermal = environmentCommand("pmset", "-g", "therm")
	} else if runtime.GOOS == "linux" {
		read := func(path string) string {
			file, err := os.Open(path)
			if err != nil {
				return ""
			}
			defer file.Close()
			data, err := io.ReadAll(io.LimitReader(file, 64*1024))
			if err != nil {
				return ""
			}
			return strings.TrimSpace(string(data))
		}
		h.OSVersion = environmentCommand("uname", "-r")
		h.Load = read("/proc/loadavg")
		h.MemoryState = read("/proc/meminfo")
		for _, line := range strings.Split(h.MemoryState, "\n") {
			f := strings.Fields(line)
			if len(f) >= 2 && f[0] == "MemTotal:" {
				v, _ := strconv.ParseInt(f[1], 10, 64)
				h.MemoryBytes = v * 1024
			}
			if strings.HasPrefix(line, "Swap") {
				h.Swap += line + "\n"
			}
		}
		for _, line := range strings.Split(read("/proc/cpuinfo"), "\n") {
			key, value, ok := strings.Cut(line, ":")
			if ok && (strings.TrimSpace(key) == "model name" || strings.TrimSpace(key) == "Hardware") {
				h.CPUBrand = strings.TrimSpace(value)
				break
			}
		}
		h.Thermal = read("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor")
	}
	// Record numeric process pressure, never arguments, command paths or env vars.
	h.TopCPUProcesses, h.ProcessCount = parseMatrixProcesses(environmentCommand("ps", "-axo", "pid=,ppid=,pcpu=,rss="))
	for name, value := range map[string]string{"os_version": h.OSVersion, "cpu_brand": h.CPUBrand, "load": h.Load, "memory_state": h.MemoryState, "swap": h.Swap, "thermal": h.Thermal} {
		if value == "" {
			h.Unavailable = append(h.Unavailable, name)
		}
	}
	if h.MemoryBytes <= 0 {
		h.Unavailable = append(h.Unavailable, "memory_bytes")
	}
	if h.ProcessCount == 0 {
		h.Unavailable = append(h.Unavailable, "process_pressure")
	}
	sort.Strings(h.Unavailable)
	return h
}

func observeMatrixEnvironment(ctx context.Context, cfg *config.Config, identity *setup.DatasetIdentity) matrixEnvironment {
	e := matrixEnvironment{At: time.Now().UTC(), Hardware: observeMatrixHardware(), Model: observeMatrixModel(ctx, cfg, identity),
		Limitations: []string{"two endpoint snapshots; transient changes/concurrent work between snapshots may be missed", "ps CPU percent is platform/window dependent; numeric processes are diagnostics, not resource exclusivity", "model identity embed probes occur before/after capture, outside query timing and backend counters", "operator note and input copies do not approve facts/protocol or certify quality"}}
	if !e.Model.Valid {
		e.Errors = append(e.Errors, "model_identity_not_verified")
	}
	for _, name := range e.Hardware.Unavailable {
		if name != "thermal" {
			e.Errors = append(e.Errors, "hardware_unavailable:"+name)
		}
	}
	e.Valid = len(e.Errors) == 0
	return e
}

func sameMatrixEnvironment(before, after matrixEnvironment) bool {
	a, b := before.Hardware, after.Hardware
	x, y := before.Model, after.Model
	return a.OS == b.OS && a.Arch == b.Arch && a.OSVersion == b.OSVersion && a.CPUBrand == b.CPUBrand && a.MemoryBytes == b.MemoryBytes && a.LogicalCPUs == b.LogicalCPUs &&
		x.Provider == y.Provider && x.Model == y.Model && x.Digest == y.Digest && x.Dimension == y.Dimension && x.Endpoint == y.Endpoint && x.ServerVersion == y.ServerVersion && reflect.DeepEqual(x.RuntimeOptions, y.RuntimeOptions)
}

func observeMatrixModel(ctx context.Context, cfg *config.Config, identity *setup.DatasetIdentity) matrixModelObservation {
	m := matrixModelObservation{Provider: cfg.Backends.CKV.Provider, Model: cfg.Backends.CKV.EmbedModel, ProbePolicy: "dataset declaration only; no model probe"}
	fail := func(reason string) matrixModelObservation { m.Errors = append(m.Errors, reason); return m }
	var pin struct {
		Provider string `json:"Provider"`
		Model    string `json:"Model"`
		Dim      int    `json:"Dim"`
		Digest   string `json:"model_digest"`
		Context  int    `json:"runtime_context_tokens"`
		Batch    int    `json:"runtime_batch_tokens"`
		Checksum string `json:"checksum"`
	}
	if identity == nil || json.Unmarshal(identity.EmbeddingIdentity, &pin) != nil || pin.Dim <= 0 {
		return fail("dataset_model_pin_missing")
	}
	// Canonical legacy mock identities retain provider/normalization in their
	// exact checksum, rather than separate keys. Preserve that existing format.
	if pin.Provider == "" && cfg.Backends.CKV.Provider == "mock" && pin.Checksum == fmt.Sprintf("provider=mock;model=%s;dim=%d;pooling=;normalize=l2", pin.Model, pin.Dim) {
		pin.Provider = "mock"
		m.ProbePolicy = "validated legacy mock checksum declaration; no native model probe"
	}
	if m.Provider != pin.Provider {
		return fail("configured_provider_differs_from_dataset")
	}
	if m.Provider == "mock" {
		if m.Model != "" && m.Model != pin.Model {
			return fail("configured_model_differs_from_dataset")
		}
		m.Model = pin.Model
		m.Dimension = pin.Dim
		m.Valid = true
		return m
	}
	if m.Provider != "ollama" {
		return fail("environment_model_probe_provider_unsupported")
	}
	if !strings.Contains(m.Model, ":") {
		m.Model += ":latest"
	}
	_, digestErr := hex.DecodeString(pin.Digest)
	if m.Model != pin.Model || len(pin.Digest) != 64 || digestErr != nil || pin.Digest != strings.ToLower(pin.Digest) {
		return fail("configured_model_or_dataset_digest_invalid")
	}
	m.Endpoint = cfg.Backends.CKV.OllamaURL
	m.ProbePolicy = "local /api/version; tags; one strict native embed with dataset runtime options; tags again; optional /api/ps; outside query timing"
	u, err := url.Parse(m.Endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || !(u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
		return fail("model_endpoint_must_be_loopback_http")
	}
	client := &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(path string, payload any, dest any) error {
		method := http.MethodGet
		var body io.Reader
		if payload != nil {
			data, e := json.Marshal(payload)
			if e != nil {
				return e
			}
			body = bytes.NewReader(data)
			method = http.MethodPost
		}
		deadline := 3 * time.Second
		if payload != nil {
			deadline = 90 * time.Second
		}
		ctx, cancel := context.WithTimeout(ctx, deadline)
		defer cancel()
		req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(m.Endpoint, "/")+path, body)
		if e != nil {
			return e
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		response, e := client.Do(req)
		if e != nil {
			return e
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return fmt.Errorf("unexpected model HTTP status")
		}
		data, e := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
		if e != nil {
			return e
		}
		if len(data) > 1024*1024 {
			return fmt.Errorf("model metadata exceeds bound")
		}
		return json.Unmarshal(data, dest)
	}
	var version struct {
		Version string `json:"version"`
	}
	if call("/api/version", nil, &version) != nil || version.Version == "" {
		return fail("ollama_version_unavailable")
	}
	m.ServerVersion = version.Version
	tags := func() (string, error) {
		var result struct {
			Models []struct {
				Name   string `json:"name"`
				Digest string `json:"digest"`
			} `json:"models"`
		}
		if e := call("/api/tags", nil, &result); e != nil {
			return "", e
		}
		digest := ""
		matches := 0
		for _, tag := range result.Models {
			if tag.Name == m.Model {
				digest = tag.Digest
				matches++
			}
		}
		if matches != 1 {
			return "", fmt.Errorf("model tag ambiguous/missing")
		}
		return digest, nil
	}
	before, err := tags()
	m.Digest = before
	if err != nil || before != pin.Digest {
		return fail("model_digest_differs_from_dataset")
	}
	m.RuntimeOptions = map[string]int{}
	if pin.Context > 0 {
		m.RuntimeOptions["num_ctx"] = pin.Context
	}
	if pin.Batch > 0 {
		m.RuntimeOptions["num_batch"] = pin.Batch
	}
	var embedded struct {
		Embeddings [][]float64 `json:"embeddings"`
	}
	payload := map[string]any{"model": m.Model, "input": "CKS eval matrix environment identity probe", "truncate": false, "options": m.RuntimeOptions}
	if call("/api/embed", payload, &embedded) != nil || len(embedded.Embeddings) != 1 {
		return fail("native_embedding_probe_failed")
	}
	m.Dimension = len(embedded.Embeddings[0])
	if m.Dimension != pin.Dim {
		return fail("native_dimension_differs_from_dataset")
	}
	for _, v := range embedded.Embeddings[0] {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fail("native_embedding_probe_invalid")
		}
	}
	after, err := tags()
	if err != nil || after != before {
		return fail("model_digest_changed_during_probe")
	}
	var resident struct {
		Models []struct {
			Name          string `json:"name"`
			Digest        string `json:"digest"`
			Size          int64  `json:"size"`
			SizeVRAM      int64  `json:"size_vram"`
			ContextLength int    `json:"context_length"`
			ExpiresAt     string `json:"expires_at"`
		} `json:"models"`
	}
	if call("/api/ps", nil, &resident) == nil {
		selected := resident.Models[:0]
		other := 0
		for _, entry := range resident.Models {
			if entry.Name == m.Model {
				selected = append(selected, entry)
			} else {
				other++
			}
		}
		m.Residency, _ = json.Marshal(map[string]any{"selected_models": selected, "other_resident_model_count": other})
	}
	m.Valid = true
	return m
}
