package accounts

import (
	"fmt"

	"github.com/nats-io/nkeys"
)

// GenerateOperatorKey creates a new operator NKey pair.
func GenerateOperatorKey() (nkeys.KeyPair, error) {
	kp, err := nkeys.CreateOperator()
	if err != nil {
		return nil, fmt.Errorf("create operator key: %w", err)
	}
	return kp, nil
}

// GenerateAccountKey creates a new account NKey pair.
func GenerateAccountKey() (nkeys.KeyPair, error) {
	kp, err := nkeys.CreateAccount()
	if err != nil {
		return nil, fmt.Errorf("create account key: %w", err)
	}
	return kp, nil
}

// GenerateUserKey creates a new user NKey pair (ephemeral, in-memory only).
func GenerateUserKey() (nkeys.KeyPair, error) {
	kp, err := nkeys.CreateUser()
	if err != nil {
		return nil, fmt.Errorf("create user key: %w", err)
	}
	return kp, nil
}

// OperatorKeyFromSeed reconstructs an operator key pair from a seed string.
// Returns an error if the seed is not an operator key (prefix "O").
func OperatorKeyFromSeed(seed string) (nkeys.KeyPair, error) {
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return nil, fmt.Errorf("parse operator seed: %w", err)
	}
	pub, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("extract operator public key: %w", err)
	}
	if !nkeys.IsValidPublicOperatorKey(pub) {
		return nil, fmt.Errorf("seed is not an operator key (public key prefix %q)", pub[:1])
	}
	return kp, nil
}

// AccountKeyFromSeed reconstructs an account key pair from a seed string.
// Returns an error if the seed is not an account key (prefix "A").
func AccountKeyFromSeed(seed string) (nkeys.KeyPair, error) {
	kp, err := nkeys.FromSeed([]byte(seed))
	if err != nil {
		return nil, fmt.Errorf("parse account seed: %w", err)
	}
	pub, err := kp.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("extract account public key: %w", err)
	}
	if !nkeys.IsValidPublicAccountKey(pub) {
		return nil, fmt.Errorf("seed is not an account key (public key prefix %q)", pub[:1])
	}
	return kp, nil
}

// PublicKey extracts the public key string from a key pair.
func PublicKey(kp nkeys.KeyPair) (string, error) {
	pub, err := kp.PublicKey()
	if err != nil {
		return "", fmt.Errorf("extract public key: %w", err)
	}
	return pub, nil
}

// Seed extracts the seed string from a key pair.
func Seed(kp nkeys.KeyPair) (string, error) {
	seed, err := kp.Seed()
	if err != nil {
		return "", fmt.Errorf("extract seed: %w", err)
	}
	return string(seed), nil
}
