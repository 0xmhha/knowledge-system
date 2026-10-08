package setup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const readerProtocol = "reader-flock-v1"

type ReaderLease struct{ file *os.File }

func (l *ReaderLease) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return errors.Join(unix.Flock(int(f.Fd()), unix.LOCK_UN), f.Close())
}

func managedDirectory(path string) error {
	if err := os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("managed directory must not be linked")
	}
	return nil
}

func openReaderLock(dataset, version string, create bool) (*os.File, error) {
	if err := validateVersion(version); err != nil {
		return nil, err
	}
	root := filepath.Join(dataset, ".readers")
	if create {
		if err := managedDirectory(root); err != nil {
			return nil, err
		}
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("reader lease directory is absent or linked")
	}
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	if create {
		flags = unix.O_CREAT | unix.O_RDWR | unix.O_NOFOLLOW | unix.O_CLOEXEC
	}
	path := filepath.Join(root, version+".lock")
	fd, err := unix.Open(path, flags, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 {
		f.Close()
		return nil, fmt.Errorf("invalid reader lease inode")
	}
	return f, nil
}

func prepareReaderProtocol(dataset, version string) error {
	f, err := openReaderLock(dataset, version, true)
	if err != nil {
		return err
	}
	if err := errors.Join(f.Sync(), f.Close()); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Join(dataset, ".readers")); err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(dataset, version, "reader-protocol.json"), map[string]string{"protocol": readerProtocol})
}

func hasReaderProtocol(versionDir string) bool {
	var marker map[string]string
	path := filepath.Join(versionDir, "reader-protocol.json")
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && readJSON(path, &marker) == nil && len(marker) == 1 && marker["protocol"] == readerProtocol
}

// PinVersionReader admits a GC-aware reader before it opens any artifact. Older
// candidates return nil: GC retains them because their older readers have no lease.
func PinVersionReader(versionDir string) (*ReaderLease, error) {
	dataset, version := filepath.Dir(versionDir), filepath.Base(versionDir)
	fd, err := unix.Open(filepath.Join(dataset, ".reindex.lock"), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		if os.IsNotExist(err) && !hasReaderProtocol(versionDir) {
			return nil, nil
		}
		return nil, err
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 {
		return nil, fmt.Errorf("invalid reader admission lock")
	}
	if err := unix.Flock(fd, unix.LOCK_SH|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("reader admission conflicts with dataset mutation")
	}
	defer unix.Flock(fd, unix.LOCK_UN)
	if info, err := os.Lstat(versionDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("reader candidate disappeared or changed")
	}
	if !hasReaderProtocol(versionDir) {
		return nil, nil
	}
	if id, err := InspectVersionIdentity(versionDir); err != nil || id == nil {
		return nil, fmt.Errorf("reader requires verified pinned candidate")
	}
	f, err := openReaderLock(dataset, version, false)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("reader candidate is being collected")
	}
	return &ReaderLease{file: f}, nil
}

type GCPolicy struct {
	KeepRecent      int           `json:"keep_recent"`
	MinAge          time.Duration `json:"min_age_ns"`
	MaxBytes        int64         `json:"max_bytes,omitempty"`
	ProtectVersions []string      `json:"protect_versions,omitempty"`
}

type GCVersion struct {
	Version    string            `json:"version"`
	Bytes      int64             `json:"bytes"`
	ModifiedNS int64             `json:"modified_ns"`
	Files      map[string]string `json:"files"`
	Reasons    []string          `json:"protected_reasons"`
	Delete     bool              `json:"delete"`
}

type GCPlan struct {
	SchemaVersion       int         `json:"schema_version"`
	At                  time.Time   `json:"at"`
	Policy              GCPolicy    `json:"policy"`
	Current             string      `json:"current"`
	Versions            []GCVersion `json:"versions"`
	TotalBytes          int64       `json:"total_bytes"`
	ReclaimBytes        int64       `json:"reclaim_bytes"`
	RetainedBytes       int64       `json:"retained_bytes"`
	OverBudget          bool        `json:"over_budget"`
	PendingTrashBytes   int64       `json:"pending_trash_bytes"`
	PendingTrashEntries int         `json:"pending_trash_entries"`
	Digest              string      `json:"digest"`
}

func validateGCPolicy(policy GCPolicy) error {
	if policy.KeepRecent < 2 || policy.MinAge < 0 || policy.MaxBytes < 0 {
		return fmt.Errorf("GC requires keep_recent >= 2 and nonnegative age/capacity")
	}
	for _, version := range policy.ProtectVersions {
		if err := validateVersion(version); err != nil {
			return err
		}
	}
	return nil
}

func gcInventory(ctx context.Context, root string) (map[string]string, int64, error) {
	files := map[string]string{}
	var size int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		var stat unix.Stat_t
		if err != nil || !info.Mode().IsRegular() || unix.Lstat(path, &stat) != nil || stat.Nlink != 1 {
			return fmt.Errorf("GC inventory contains linked or nonregular artifact")
		}
		fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		f := os.NewFile(uintptr(fd), path)
		h := sha256.New()
		_, copyErr := io.Copy(h, f)
		if err := errors.Join(copyErr, f.Close()); err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = fmt.Sprintf("%s:%o", hex.EncodeToString(h.Sum(nil)), info.Mode().Perm())
		size += info.Size()
		return nil
	})
	return files, size, err
}

func gcDigest(plan GCPlan) string {
	plan.Digest = ""
	data, _ := json.Marshal(plan)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func gcCurrent(dataset string) (string, error) {
	current, err := os.Readlink(filepath.Join(dataset, "current"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("GC current is not a version pointer")
	}
	if err := validateVersion(current); err != nil {
		return "", err
	}
	path := filepath.Join(dataset, current)
	if info, err := os.Lstat(path); err != nil || !info.IsDir() {
		return "", fmt.Errorf("GC current is absent or linked")
	}
	if _, err := InspectVersionIdentity(path); err != nil {
		return "", fmt.Errorf("GC current identity requires review")
	}
	return current, nil
}

func PlanGC(ctx context.Context, dataset string, policy GCPolicy, at time.Time) (GCPlan, error) {
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		return GCPlan{}, err
	}
	defer lock.release()
	return planGCLocked(ctx, dataset, policy, at)
}

func planGCLocked(ctx context.Context, dataset string, policy GCPolicy, at time.Time) (GCPlan, error) {
	if err := validateGCPolicy(policy); err != nil {
		return GCPlan{}, err
	}
	if at.IsZero() || at.After(time.Now().Add(time.Minute)) {
		return GCPlan{}, fmt.Errorf("invalid GC evaluation time")
	}
	plan := GCPlan{SchemaVersion: 1, At: at.UTC(), Policy: policy, Versions: []GCVersion{}}
	plan.Policy.ProtectVersions = append([]string(nil), policy.ProtectVersions...)
	sort.Strings(plan.Policy.ProtectVersions)
	current, err := gcCurrent(dataset)
	if err != nil {
		return plan, err
	}
	plan.Current = current
	trash := filepath.Join(dataset, ".gc-trash")
	if info, err := os.Lstat(trash); err == nil {
		if !info.IsDir() {
			return plan, fmt.Errorf("GC trash is linked")
		}
		entries, err := os.ReadDir(trash)
		if err != nil {
			return plan, err
		}
		plan.PendingTrashEntries = len(entries)
		if len(entries) > 0 {
			_, size, err := gcInventory(ctx, trash)
			if err != nil {
				return plan, err
			}
			plan.PendingTrashBytes = size
		}
	} else if !os.IsNotExist(err) {
		return plan, err
	}
	var rollback struct {
		Versions []string `json:"versions"`
	}
	if err := readJSON(filepath.Join(dataset, "rollback-targets.json"), &rollback); err != nil && !os.IsNotExist(err) {
		return plan, fmt.Errorf("invalid rollback references")
	}
	for _, v := range rollback.Versions {
		if validateVersion(v) != nil {
			return plan, fmt.Errorf("invalid rollback version")
		}
		if info, err := os.Lstat(filepath.Join(dataset, v)); err != nil || !info.IsDir() {
			return plan, fmt.Errorf("rollback reference is absent or linked")
		}
	}
	entries, err := os.ReadDir(dataset)
	if err != nil {
		return plan, err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return plan, err
		}
		version := entry.Name()
		if strings.HasPrefix(version, ".") || version == "current" || !entry.IsDir() {
			continue
		}
		path := filepath.Join(dataset, version)
		info, err := entry.Info()
		if err != nil {
			return plan, err
		}
		item := GCVersion{Version: version, ModifiedNS: info.ModTime().UnixNano(), Reasons: []string{}}
		if plan.PendingTrashEntries > 0 {
			item.Reasons = append(item.Reasons, "pending_gc_recovery")
		}
		item.Files, item.Bytes, err = gcInventory(ctx, path)
		if err != nil {
			return plan, err
		}
		if version == current {
			item.Reasons = append(item.Reasons, "current")
		}
		for _, v := range rollback.Versions {
			if version == v {
				item.Reasons = append(item.Reasons, "rollback_target")
			}
		}
		for _, v := range plan.Policy.ProtectVersions {
			if version == v {
				item.Reasons = append(item.Reasons, "explicit_protection")
			}
		}
		id, idErr := InspectVersionIdentity(path)
		if idErr != nil || id == nil {
			item.Reasons = append(item.Reasons, "unverified_or_legacy")
		}
		if !hasReaderProtocol(path) {
			item.Reasons = append(item.Reasons, "unknown_legacy_readers")
		} else {
			f, err := openReaderLock(dataset, version, false)
			if err != nil {
				return plan, err
			}
			if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
				item.Reasons = append(item.Reasons, "pinned_reader")
			} else {
				unix.Flock(int(f.Fd()), unix.LOCK_UN)
			}
			if err := f.Close(); err != nil {
				return plan, err
			}
		}
		_, holdErr := os.Lstat(filepath.Join(path, "review-hold.json"))
		_, intentErr := os.Lstat(filepath.Join(path, "review-intent.json"))
		if holdErr == nil || intentErr == nil {
			var release struct {
				ProjectID       string `json:"project_id"`
				DatasetID       string `json:"dataset_id"`
				SnapshotID      string `json:"snapshot_id"`
				PatchID         string `json:"patch_id"`
				DecisionsSHA256 string `json:"decisions_sha256"`
			}
			releasePath := filepath.Join(path, "review-release.json")
			info, err := os.Lstat(releasePath)
			valid := err == nil && info.Mode().IsRegular() && readJSON(releasePath, &release) == nil && id != nil && release.ProjectID == id.Source.ProjectID && release.DatasetID == id.DatasetID && release.SnapshotID == id.Source.SnapshotID && release.PatchID != "" && len(release.DecisionsSHA256) == 64 && strings.Trim(release.DecisionsSHA256, "0123456789abcdef") == ""
			if !valid {
				item.Reasons = append(item.Reasons, "review_hold")
			}
		} else if !os.IsNotExist(holdErr) || !os.IsNotExist(intentErr) {
			return plan, fmt.Errorf("cannot inspect review references")
		}
		if at.Sub(info.ModTime()) < policy.MinAge {
			item.Reasons = append(item.Reasons, "retention_age")
		}
		plan.Versions = append(plan.Versions, item)
	}
	// Keep the most recently written versions; ties have a stable name order.
	order := make([]int, len(plan.Versions))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := plan.Versions[order[i]], plan.Versions[order[j]]
		if a.ModifiedNS != b.ModifiedNS {
			return a.ModifiedNS > b.ModifiedNS
		}
		return a.Version > b.Version
	})
	for i, index := range order {
		if i >= policy.KeepRecent {
			break
		}
		plan.Versions[index].Reasons = append(plan.Versions[index].Reasons, "recent_rollback")
	}
	for i := range plan.Versions {
		item := &plan.Versions[i]
		sort.Strings(item.Reasons)
		item.Delete = len(item.Reasons) == 0
		plan.TotalBytes += item.Bytes
		if item.Delete {
			plan.ReclaimBytes += item.Bytes
		}
	}
	for _, v := range policy.ProtectVersions {
		found := false
		for _, item := range plan.Versions {
			if item.Version == v {
				found = true
			}
		}
		if !found {
			return plan, fmt.Errorf("protected version is absent")
		}
	}
	plan.TotalBytes += plan.PendingTrashBytes
	plan.RetainedBytes = plan.TotalBytes - plan.ReclaimBytes
	plan.OverBudget = policy.MaxBytes > 0 && plan.RetainedBytes > policy.MaxBytes
	plan.Digest = gcDigest(plan)
	return plan, nil
}

func recordRollbackTargets(dataset, prev string, retainOld bool) error {
	versions := []string{}
	if retainOld {
		var old struct {
			Versions []string `json:"versions"`
		}
		if err := readJSON(filepath.Join(dataset, "rollback-targets.json"), &old); err != nil && !os.IsNotExist(err) {
			return err
		}
		versions = old.Versions
	}
	if prev != "" {
		versions = append(versions, prev)
	}
	unique := map[string]bool{}
	for _, v := range versions {
		if err := validateVersion(v); err != nil {
			return err
		}
		unique[v] = true
	}
	versions = []string{}
	for v := range unique {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	return writeJSONAtomic(filepath.Join(dataset, "rollback-targets.json"), map[string]any{"versions": versions})
}

func checkGCReservedVersion(dataset, version string) error {
	trash := filepath.Join(dataset, ".gc-trash")
	info, err := os.Lstat(trash)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.IsDir() {
		return fmt.Errorf("invalid GC trash")
	}
	entries, err := os.ReadDir(trash)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "-"+version+".json") {
			return fmt.Errorf("version has pending GC recovery; resume it first")
		}
	}
	return nil
}

type gcJournal struct {
	SchemaVersion int       `json:"schema_version"`
	PlanDigest    string    `json:"plan_digest"`
	Version       GCVersion `json:"version"`
	Phase         string    `json:"phase"`
}

var gcFault func(stage, path string) error // private failure injection

func gcStep(stage, path string) error {
	if gcFault != nil {
		return gcFault(stage, path)
	}
	return nil
}

func ApplyGC(ctx context.Context, dataset string, reviewed GCPlan) error {
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		return err
	}
	defer lock.release()
	if reviewed.SchemaVersion != 1 || reviewed.Digest == "" || gcDigest(reviewed) != reviewed.Digest {
		return fmt.Errorf("invalid reviewed GC plan")
	}
	actual, err := planGCLocked(ctx, dataset, reviewed.Policy, reviewed.At)
	if err != nil {
		return err
	}
	if actual.Digest != reviewed.Digest {
		return fmt.Errorf("stale GC plan; review a fresh dry-run")
	}
	if actual.PendingTrashEntries > 0 {
		return fmt.Errorf("resume pending GC before applying another plan")
	}
	trash := filepath.Join(dataset, ".gc-trash")
	if err := managedDirectory(trash); err != nil {
		return err
	}
	for _, version := range reviewed.Versions {
		if !version.Delete {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		j := gcJournal{SchemaVersion: 1, PlanDigest: reviewed.Digest, Version: version, Phase: "prepared"}
		name := reviewed.Digest + "-" + version.Version
		jpath := filepath.Join(trash, name+".json")
		target := filepath.Join(trash, name)
		if _, err := os.Lstat(jpath); !os.IsNotExist(err) {
			return fmt.Errorf("pending GC journal requires resume")
		}
		if err := writeJSONAtomic(jpath, j); err != nil {
			return err
		}
		if err := gcStep("before-trash-rename", target); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(dataset, version.Version), target); err != nil {
			return err
		}
		if err := errors.Join(syncDirectory(dataset), syncDirectory(trash)); err != nil {
			return err
		}
		if err := gcStep("after-trash-rename", target); err != nil {
			return err
		}
		if err := deleteGCTrash(ctx, dataset, jpath, target, j); err != nil {
			return err
		}
	}
	left, err := os.ReadDir(trash)
	if err != nil {
		return err
	}
	if len(left) > 0 {
		return fmt.Errorf("unrecognized GC trash requires review")
	}
	return nil
}

func deleteGCTrash(ctx context.Context, dataset, jpath, target string, j gcJournal) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if current, err := os.Readlink(filepath.Join(dataset, "current")); err == nil && current == j.Version.Version {
		return fmt.Errorf("GC trash conflicts with current")
	}
	if _, err := os.Lstat(filepath.Join(dataset, j.Version.Version)); !os.IsNotExist(err) {
		return fmt.Errorf("GC trash conflicts with a live version")
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("GC trash must be a real directory")
		}
		remaining, _, err := gcInventory(ctx, target)
		if err != nil {
			return err
		}
		if j.Phase != "deleting" && len(remaining) != len(j.Version.Files) {
			return fmt.Errorf("GC trash inventory changed")
		}
		for file, digest := range remaining {
			if j.Version.Files[file] != digest {
				return fmt.Errorf("GC trash contains foreign or changed bytes")
			}
		}
		j.Phase = "deleting"
		if err := writeJSONAtomic(jpath, j); err != nil {
			return err
		}
		if err := gcStep("before-trash-delete", target); err != nil {
			return err
		}
		// Each file deletion is cancellable. Directories disappear only after the
		// retained inventory was verified and all selected files are gone.
		paths := make([]string, 0, len(remaining))
		for file := range remaining {
			paths = append(paths, file)
		}
		sort.Strings(paths)
		for _, file := range paths {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := os.Remove(filepath.Join(target, filepath.FromSlash(file))); err != nil {
				return err
			}
			if err := gcStep("after-trash-file", target); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := syncDirectory(filepath.Dir(target)); err != nil {
		return err
	}
	if err := os.Remove(jpath); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(jpath))
}

func ResumeGC(ctx context.Context, dataset string) error {
	lock, err := acquireReindexLock(dataset)
	if err != nil {
		return err
	}
	defer lock.release()
	if _, err := gcCurrent(dataset); err != nil {
		return err
	}
	trash := filepath.Join(dataset, ".gc-trash")
	info, err := os.Lstat(trash)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.IsDir() {
		return fmt.Errorf("GC trash is linked or invalid")
	}
	entries, err := os.ReadDir(trash)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("GC journal is not regular")
		}
		path := filepath.Join(trash, entry.Name())
		var j gcJournal
		if err := readJSON(path, &j); err != nil {
			return err
		}
		if j.SchemaVersion != 1 || len(j.PlanDigest) != 64 || strings.Trim(j.PlanDigest, "0123456789abcdef") != "" || validateVersion(j.Version.Version) != nil || !j.Version.Delete ||
			entry.Name() != j.PlanDigest+"-"+j.Version.Version+".json" || (j.Phase != "prepared" && j.Phase != "deleting") {
			return fmt.Errorf("invalid GC journal")
		}
		for file := range j.Version.Files {
			if file == "" || filepath.IsAbs(file) || filepath.ToSlash(filepath.Clean(file)) != file || file == ".." || strings.HasPrefix(file, "../") {
				return fmt.Errorf("invalid GC journal file")
			}
		}
		target := strings.TrimSuffix(path, ".json")
		if _, err := os.Lstat(target); os.IsNotExist(err) && j.Phase == "prepared" {
			// Interruption before rename: do not delete the still-live candidate.
			if _, err := os.Lstat(filepath.Join(dataset, j.Version.Version)); err != nil {
				return fmt.Errorf("prepared GC version disappeared")
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			if err := syncDirectory(trash); err != nil {
				return err
			}
			continue
		}
		if err := deleteGCTrash(ctx, dataset, path, target, j); err != nil {
			return err
		}
	}
	left, err := os.ReadDir(trash)
	if err != nil {
		return err
	}
	if len(left) > 0 {
		return fmt.Errorf("unrecognized GC trash requires review")
	}
	return nil
}
