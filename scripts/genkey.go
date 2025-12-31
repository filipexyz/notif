//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func main() {
	// Generate hash for a known test key
	testKey := "nsh_test_abcdefghij1234567890ab"
	h := sha256.Sum256([]byte(testKey))
	hash := hex.EncodeToString(h[:])

	fmt.Println("Test Key:", testKey)
	fmt.Println("Hash:", hash)
	fmt.Println()
	fmt.Println("SQL:")
	fmt.Printf(`INSERT INTO api_keys (key_hash, key_prefix, environment, name)
VALUES ('%s', 'nsh_test_abcdef', 'test', 'Dev Test Key');
`, hash)
}
