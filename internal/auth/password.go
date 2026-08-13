package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2Params are deliberately generous defaults for a self-hosted,
// low-concurrency internal tool rather than a high-traffic public API,
// where per-request memory cost is a bigger constraint. See
// https://pkg.go.dev/golang.org/x/crypto/argon2#IDKey for the tradeoffs.
type argon2Params struct {
	memory      uint32 // KiB
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultArgon2Params = argon2Params{
	memory:      64 * 1024, // 64 MiB
	iterations:  3,
	parallelism: 4,
	saltLength:  16,
	keyLength:   32,
}

var ErrPasswordMismatch = errors.New("password does not match")

// HashPassword hashes a plaintext password with Argon2id, encoding the
// parameters and salt alongside the hash in the same string (the PHC
// string format) so that tuning the parameters later doesn't invalidate
// or misinterpret existing stored hashes.
func HashPassword(password string) (string, error) {
	salt := make([]byte, defaultArgon2Params.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		defaultArgon2Params.iterations,
		defaultArgon2Params.memory,
		defaultArgon2Params.parallelism,
		defaultArgon2Params.keyLength,
	)

	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		defaultArgon2Params.memory,
		defaultArgon2Params.iterations,
		defaultArgon2Params.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)
	return encoded, nil
}

// VerifyPassword checks a plaintext password against an encoded hash
// produced by HashPassword. It returns ErrPasswordMismatch if the password
// is wrong, or a non-nil error if the hash is malformed.
func VerifyPassword(encodedHash, password string) error {
	params, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return err
	}

	candidate := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		uint32(len(hash)),
	)

	if subtle.ConstantTimeCompare(hash, candidate) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

func decodeHash(encoded string) (argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argon2Params{}, nil, nil, errors.New("auth: invalid hash format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("auth: parse version: %w", err)
	}
	if version != argon2.Version {
		return argon2Params{}, nil, nil, fmt.Errorf("auth: unsupported argon2 version %d", version)
	}

	var p argon2Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("auth: parse params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("auth: decode salt: %w", err)
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argon2Params{}, nil, nil, fmt.Errorf("auth: decode hash: %w", err)
	}

	return p, salt, hash, nil
}
