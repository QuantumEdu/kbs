package agenttelemetry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSaltManagerGeneratesSalt(t *testing.T) {
	dir := t.TempDir()
	saltPath := filepath.Join(dir, "salt")

	sm, err := NewSaltManager(saltPath)
	if err != nil {
		t.Fatalf("NewSaltManager: %v", err)
	}

	if len(sm.Salt()) != 32 {
		t.Errorf("salt length = %d, want 32", len(sm.Salt()))
	}

	info, err := os.Stat(saltPath)
	if err != nil {
		t.Fatalf("salt file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("salt perms = %o, want 0600", info.Mode().Perm())
	}
}

func TestNewSaltManagerLoadsExisting(t *testing.T) {
	dir := t.TempDir()
	saltPath := filepath.Join(dir, "salt")

	original := make([]byte, 32)
	for i := range original {
		original[i] = byte('a' + i%26)
	}
	if err := os.WriteFile(saltPath, original, 0600); err != nil {
		t.Fatal(err)
	}

	sm, err := NewSaltManager(saltPath)
	if err != nil {
		t.Fatalf("NewSaltManager: %v", err)
	}

	if string(sm.Salt()) != string(original) {
		t.Error("loaded salt does not match original")
	}
}

func TestValidateSaltPermsWrongPerms(t *testing.T) {
	dir := t.TempDir()
	saltPath := filepath.Join(dir, "salt")

	if err := os.WriteFile(saltPath, make([]byte, 32), 0644); err != nil {
		t.Fatal(err)
	}

	err := ValidateSaltPerms(saltPath)
	if err == nil {
		t.Error("expected error for wrong perms, got nil")
	}
}

func TestValidateSaltPermsCorrectPerms(t *testing.T) {
	dir := t.TempDir()
	saltPath := filepath.Join(dir, "salt")

	if err := os.WriteFile(saltPath, make([]byte, 32), 0600); err != nil {
		t.Fatal(err)
	}

	err := ValidateSaltPerms(saltPath)
	if err != nil {
		t.Errorf("unexpected error for correct perms: %v", err)
	}
}

func TestValidateSaltPermsMissing(t *testing.T) {
	err := ValidateSaltPerms("/nonexistent/salt/path")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestArgHasherHashDeterministic(t *testing.T) {
	salt := []byte("test-salt-32-bytes-long!!!!!!!") // 32 bytes
	h := NewArgHasher(salt)

	args := []string{"curl", "-H", "Authorization: Bearer sk-abc123"}
	hash1 := h.Hash(args)
	hash2 := h.Hash(args)

	if hash1 != hash2 {
		t.Error("Hash is not deterministic")
	}
	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}
}

func TestArgHasherHashDifferentArgsDifferentHash(t *testing.T) {
	salt := []byte("test-salt-32-bytes-long!!!!!!!")
	h := NewArgHasher(salt)

	hash1 := h.Hash([]string{"curl", "example.com"})
	hash2 := h.Hash([]string{"curl", "other.com"})

	if hash1 == hash2 {
		t.Error("different args produced same hash")
	}
}

func TestArgHasherHashDifferentSaltDifferentHash(t *testing.T) {
	salt1 := []byte("salt-aaaaaaaaaaaaaaaaaaaaaaaaaa")
	salt2 := []byte("salt-bbbbbbbbbbbbbbbbbbbbbbbbbb")
	h1 := NewArgHasher(salt1)
	h2 := NewArgHasher(salt2)

	args := []string{"curl", "example.com"}
	if h1.Hash(args) == h2.Hash(args) {
		t.Error("different salts produced same hash")
	}
}

func TestArgHasherVerify(t *testing.T) {
	salt := []byte("test-salt-32-bytes-long!!!!!!!")
	h := NewArgHasher(salt)

	args := []string{"bash", "-c", "echo hello"}
	hash := h.Hash(args)

	if !h.Verify(args, hash) {
		t.Error("Verify returned false for correct args")
	}
	if h.Verify([]string{"different"}, hash) {
		t.Error("Verify returned true for wrong args")
	}
}

func TestSaltFingerprint(t *testing.T) {
	salt := []byte("test-salt-32-bytes-long!!!!!!!")
	sm := &SaltManager{salt: salt}

	fp := sm.SaltFingerprint()
	if len(fp) != 8 {
		t.Errorf("fingerprint length = %d, want 8", len(fp))
	}
}

func TestSaltFingerprintDeterministic(t *testing.T) {
	salt := []byte("test-salt-32-bytes-long!!!!!!!")
	sm1 := &SaltManager{salt: append([]byte{}, salt...)}
	sm2 := &SaltManager{salt: append([]byte{}, salt...)}

	if sm1.SaltFingerprint() != sm2.SaltFingerprint() {
		t.Error("SaltFingerprint is not deterministic")
	}
}
