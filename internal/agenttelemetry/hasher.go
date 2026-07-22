package agenttelemetry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// SaltManager manages the hashing salt file.
type SaltManager struct {
	saltPath string
	salt     []byte
}

// NewSaltManager creates or loads a salt file. If the file doesn't exist, it
// generates 32 random bytes and writes them with 0600 permissions. If the file
// exists, it loads the salt into memory.
func NewSaltManager(saltPath string) (*SaltManager, error) {
	salt, err := os.ReadFile(saltPath)
	if err != nil {
		if os.IsNotExist(err) {
			salt = make([]byte, 32)
			if _, err := rand.Read(salt); err != nil {
				return nil, fmt.Errorf("generate salt: %w", err)
			}
			if err := os.WriteFile(saltPath, salt, 0600); err != nil {
				return nil, fmt.Errorf("write salt: %w", err)
			}
			return &SaltManager{saltPath: saltPath, salt: salt}, nil
		}
		return nil, fmt.Errorf("read salt: %w", err)
	}
	return &SaltManager{saltPath: saltPath, salt: salt}, nil
}

// Salt returns the raw salt bytes.
func (sm *SaltManager) Salt() []byte {
	return sm.salt
}

// SaltFingerprint returns the first 8 hex characters of SHA-256(salt).
func (sm *SaltManager) SaltFingerprint() string {
	h := sha256.Sum256(sm.salt)
	return hex.EncodeToString(h[:])[:8]
}

// ValidateSaltPerms checks that the salt file has 0600 permissions. Returns an
// error if permissions are too open or the file cannot be stat'd.
func ValidateSaltPerms(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat salt: %w", err)
	}
	if info.Mode().Perm() != 0600 {
		return fmt.Errorf("salt file permissions too open: %o (want 0600)", info.Mode().Perm())
	}
	return nil
}

// ArgHasher hashes tool/command arguments using a salt.
type ArgHasher struct {
	salt []byte
}

// NewArgHasher creates a new ArgHasher with the given salt.
func NewArgHasher(salt []byte) *ArgHasher {
	return &ArgHasher{salt: salt}
}

// Hash computes SHA-256(salt + strings.Join(args, "\x00")) and returns the
// hex-encoded hash.
func (h *ArgHasher) Hash(args []string) string {
	data := string(h.salt) + strings.Join(args, "\x00")
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

// Verify checks whether the given args produce the expected hash.
func (h *ArgHasher) Verify(args []string, hash string) bool {
	return h.Hash(args) == hash
}
