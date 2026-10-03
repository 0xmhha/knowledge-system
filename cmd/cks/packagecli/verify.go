package packagecli

// Post-install verification complements the standalone Python verifier. This
// command must never be the first trusted operation on an untrusted archive:
// verify-release.py and a separately obtained public key run before extraction.

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const maxArchiveBytes int64 = 512 << 20
const maxMemberBytes int64 = 256 << 20

type release struct {
	SchemaVersion          int    `json:"schema_version"`
	SignatureAlgorithm     string `json:"signature_algorithm"`
	Scope                  string `json:"scope"`
	KeyID                  string `json:"key_id"`
	ArchiveName            string `json:"archive_name"`
	ArchiveSHA256          string `json:"archive_sha256"`
	ArchiveBytes           int64  `json:"archive_bytes"`
	HostOS                 string `json:"host_os"`
	HostArch               string `json:"host_arch"`
	Commit                 string `json:"commit"`
	ManifestSHA256         string `json:"manifest_sha256"`
	ModulesSHA256          string `json:"modules_sha256"`
	LicenseSHA256          string `json:"license_sha256"`
	LicenseInventorySHA256 string `json:"license_inventory_sha256"`
}

type packageManifest struct {
	Scope    string `json:"scope"`
	Commit   string `json:"commit"`
	HostOS   string `json:"host_os"`
	HostArch string `json:"host_arch"`
	Binaries map[string]struct {
		SHA256 string `json:"sha256"`
	} `json:"binaries"`
	ThirdPartyLicenseInventory struct {
		SHA256 string `json:"sha256"`
	} `json:"third_party_license_inventory"`
}

type licenseInventoryEntry struct {
	LicenseFiles []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	} `json:"license_files"`
}

type licenseInventory struct {
	SchemaVersion  int                     `json:"schema_version"`
	ReviewStatus   string                  `json:"review_status"`
	Modules        []licenseInventoryEntry `json:"modules"`
	VendoredAssets []licenseInventoryEntry `json:"vendored_assets"`
}

func NewCmd() *cobra.Command {
	root := &cobra.Command{Use: "package", Short: "Verify installed package artifacts"}
	var publicKey, releasePath, signaturePath, archive, targetOS, targetArch string
	verify := &cobra.Command{Use: "verify", Short: "Verify a signed package sidecar and archive after installation", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := verifyRelease(publicKey, releasePath, signaturePath, archive, targetOS, targetArch)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		}}
	verify.Flags().StringVar(&publicKey, "public-key", "", "independently trusted Ed25519 public key PEM")
	verify.Flags().StringVar(&releasePath, "release", "", "signed release.json")
	verify.Flags().StringVar(&signaturePath, "signature", "", "raw release.json.sig")
	verify.Flags().StringVar(&archive, "archive", "", "package .tar.gz")
	verify.Flags().StringVar(&targetOS, "target-os", runtime.GOOS, "expected operating system")
	verify.Flags().StringVar(&targetArch, "target-arch", runtime.GOARCH, "expected architecture")
	for _, flag := range []string{"public-key", "release", "signature", "archive"} {
		_ = verify.MarkFlagRequired(flag)
	}
	root.AddCommand(verify)
	return root
}

func hashBytes(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func validHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func verifyRelease(publicKeyPath, releasePath, signaturePath, archivePath, targetOS, targetArch string) (map[string]string, error) {
	if publicKeyPath == "" || releasePath == "" || signaturePath == "" || archivePath == "" {
		return nil, errors.New("all verification paths are required")
	}
	keyPEM, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(keyPEM)
	if block == nil || len(rest) != 0 || block.Type != "PUBLIC KEY" {
		return nil, errors.New("expected one public key PEM block")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not Ed25519")
	}
	data, err := os.ReadFile(releasePath)
	if err != nil {
		return nil, err
	}
	if len(data) > 4096 {
		return nil, errors.New("release manifest too large")
	}
	sig, err := os.ReadFile(signaturePath)
	if err != nil {
		return nil, err
	}
	if len(sig) != ed25519.SignatureSize || !ed25519.Verify(key, data, sig) {
		return nil, errors.New("release signature verification failed")
	}
	var rel release
	if err := json.Unmarshal(data, &rel); err != nil {
		return nil, err
	}
	canonicalMap := map[string]any{}
	if err := json.Unmarshal(data, &canonicalMap); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(canonicalMap)
	if err != nil {
		return nil, err
	}
	if string(data) != string(canonical)+"\n" || len(canonicalMap) != 14 {
		return nil, errors.New("release manifest is not canonical contract JSON")
	}
	if rel.SchemaVersion != 1 || rel.SignatureAlgorithm != "Ed25519" || rel.Scope != "test-signed-preview" {
		return nil, errors.New("unsupported release contract")
	}
	if rel.KeyID != hashBytes(block.Bytes) || !validHash(rel.KeyID) {
		return nil, errors.New("trusted key ID differs")
	}
	for _, digest := range []string{rel.ArchiveSHA256, rel.ManifestSHA256, rel.ModulesSHA256, rel.LicenseSHA256, rel.LicenseInventorySHA256} {
		if !validHash(digest) {
			return nil, errors.New("invalid digest")
		}
	}
	if rel.HostOS != targetOS || rel.HostArch != targetArch || (targetOS != "darwin" && targetOS != "linux") || (targetArch != "arm64" && targetArch != "amd64") {
		return nil, errors.New("package target differs from requested runtime")
	}
	if !validCommit(rel.Commit) || rel.ArchiveBytes <= 0 || rel.ArchiveBytes > maxArchiveBytes {
		return nil, errors.New("invalid commit or archive size")
	}
	if filepath.Base(rel.ArchiveName) != rel.ArchiveName || !strings.HasSuffix(rel.ArchiveName, ".tar.gz") || !ascii(rel.ArchiveName) || filepath.Base(archivePath) != rel.ArchiveName {
		return nil, errors.New("archive name differs from signed release")
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	info, err := archive.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() != rel.ArchiveBytes {
		return nil, errors.New("archive size differs from signed release")
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, archive); err != nil {
		return nil, err
	}
	if hex.EncodeToString(hasher.Sum(nil)) != rel.ArchiveSHA256 {
		return nil, errors.New("archive bytes differ from signed release")
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	zipped, err := gzip.NewReader(archive)
	if err != nil {
		return nil, err
	}
	defer zipped.Close()
	reader := tar.NewReader(zipped)
	files := map[string][]byte{}
	modes := map[string]int64{}
	var root string
	var total int64
	count := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		count++
		if count > 100 || header.Size < 0 || header.Size > maxMemberBytes || total+header.Size > maxArchiveBytes {
			return nil, errors.New("archive member bound exceeded")
		}
		name := header.Name
		if header.Typeflag == tar.TypeDir {
			name = strings.TrimSuffix(name, "/")
		}
		if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || path.Clean(name) != name || strings.Contains("/"+name+"/", "/../") {
			return nil, errors.New("unsafe archive member")
		}
		parts := strings.SplitN(name, "/", 2)
		if root == "" {
			root = parts[0]
		}
		if root != parts[0] {
			return nil, errors.New("archive has multiple roots")
		}
		if _, exists := modes[name]; exists {
			return nil, errors.New("duplicate archive member")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && header.Typeflag != tar.TypeDir {
			return nil, errors.New("unsafe archive entry type")
		}
		modes[name] = header.Mode
		if header.Typeflag == tar.TypeDir {
			if header.Size != 0 {
				return nil, errors.New("directory has contents")
			}
			continue
		}
		total += header.Size
		relative := strings.TrimPrefix(name, root+"/")
		if _, keep := map[string]bool{"manifest.json": true, "modules.txt": true, "LICENSE": true, "INSTALLATION.md": true, "policies/sanitization_rules.yaml": true, "third-party-licenses.json": true, "cks": true, "ckg": true, "ckv": true}[relative]; keep || strings.HasPrefix(relative, "third-party-licenses/") {
			body, err := io.ReadAll(io.LimitReader(reader, header.Size+1))
			if err != nil {
				return nil, err
			}
			if int64(len(body)) != header.Size {
				return nil, errors.New("truncated archive member")
			}
			files[relative] = body
		} else if _, err := io.Copy(io.Discard, reader); err != nil {
			return nil, err
		}
	}
	if root == "" {
		return nil, errors.New("empty archive")
	}
	for name, digest := range map[string]string{"manifest.json": rel.ManifestSHA256, "modules.txt": rel.ModulesSHA256, "LICENSE": rel.LicenseSHA256, "third-party-licenses.json": rel.LicenseInventorySHA256} {
		if hashBytes(files[name]) != digest {
			return nil, fmt.Errorf("%s differs from signed hash", name)
		}
	}
	if len(files["INSTALLATION.md"]) == 0 || len(files["policies/sanitization_rules.yaml"]) == 0 {
		return nil, errors.New("package metadata missing")
	}
	var manifest packageManifest
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		return nil, err
	}
	if manifest.Scope != "host-preview" || manifest.Commit != rel.Commit || manifest.HostOS != targetOS || manifest.HostArch != targetArch {
		return nil, errors.New("package manifest differs from signed target")
	}
	if manifest.ThirdPartyLicenseInventory.SHA256 != rel.LicenseInventorySHA256 {
		return nil, errors.New("package license inventory digest differs")
	}
	var inventory licenseInventory
	if err := json.Unmarshal(files["third-party-licenses.json"], &inventory); err != nil {
		return nil, err
	}
	if inventory.SchemaVersion != 1 || inventory.ReviewStatus != "pending" || len(inventory.Modules) == 0 {
		return nil, errors.New("invalid license inventory")
	}
	for _, module := range append(inventory.Modules, inventory.VendoredAssets...) {
		for _, licenseFile := range module.LicenseFiles {
			if !strings.HasPrefix(licenseFile.Path, "third-party-licenses/") || !validHash(licenseFile.SHA256) || hashBytes(files[licenseFile.Path]) != licenseFile.SHA256 {
				return nil, errors.New("third-party license differs from inventory")
			}
		}
	}
	for _, name := range []string{"cks", "ckg", "ckv"} {
		if len(files[name]) == 0 || modes[root+"/"+name]&0o111 == 0 || hashBytes(files[name]) != manifest.Binaries[name].SHA256 {
			return nil, fmt.Errorf("%s differs from package manifest", name)
		}
	}
	return map[string]string{"status": "verified", "scope": rel.Scope, "archive": archivePath, "target": targetOS + "/" + targetArch, "commit": rel.Commit, "key_id": rel.KeyID}, nil
}

func validCommit(value string) bool { return len(value) == 40 && validHex(value) }
func validHex(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
func ascii(value string) bool {
	for _, r := range value {
		if r > 127 {
			return false
		}
	}
	return true
}
