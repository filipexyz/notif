package accounts

import (
	"strings"
	"testing"
)

func TestOperatorKeyFromSeed_Valid(t *testing.T) {
	kp, err := GenerateOperatorKey()
	if err != nil {
		t.Fatalf("GenerateOperatorKey: %v", err)
	}
	seed, err := Seed(kp)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	restored, err := OperatorKeyFromSeed(seed)
	if err != nil {
		t.Fatalf("OperatorKeyFromSeed: %v", err)
	}

	pub1, _ := kp.PublicKey()
	pub2, _ := restored.PublicKey()
	if pub1 != pub2 {
		t.Fatalf("public keys differ: %s vs %s", pub1, pub2)
	}
}

func TestOperatorKeyFromSeed_RejectsAccountSeed(t *testing.T) {
	kp, err := GenerateAccountKey()
	if err != nil {
		t.Fatalf("GenerateAccountKey: %v", err)
	}
	seed, err := Seed(kp)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	_, err = OperatorKeyFromSeed(seed)
	if err == nil {
		t.Fatal("expected error for account seed passed to OperatorKeyFromSeed")
	}
	if !strings.Contains(err.Error(), "not an operator key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAccountKeyFromSeed_Valid(t *testing.T) {
	kp, err := GenerateAccountKey()
	if err != nil {
		t.Fatalf("GenerateAccountKey: %v", err)
	}
	seed, err := Seed(kp)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	restored, err := AccountKeyFromSeed(seed)
	if err != nil {
		t.Fatalf("AccountKeyFromSeed: %v", err)
	}

	pub1, _ := kp.PublicKey()
	pub2, _ := restored.PublicKey()
	if pub1 != pub2 {
		t.Fatalf("public keys differ: %s vs %s", pub1, pub2)
	}
}

func TestAccountKeyFromSeed_RejectsOperatorSeed(t *testing.T) {
	kp, err := GenerateOperatorKey()
	if err != nil {
		t.Fatalf("GenerateOperatorKey: %v", err)
	}
	seed, err := Seed(kp)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	_, err = AccountKeyFromSeed(seed)
	if err == nil {
		t.Fatal("expected error for operator seed passed to AccountKeyFromSeed")
	}
	if !strings.Contains(err.Error(), "not an account key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAccountKeyFromSeed_RejectsUserSeed(t *testing.T) {
	kp, err := GenerateUserKey()
	if err != nil {
		t.Fatalf("GenerateUserKey: %v", err)
	}
	seed, err := Seed(kp)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	_, err = AccountKeyFromSeed(seed)
	if err == nil {
		t.Fatal("expected error for user seed passed to AccountKeyFromSeed")
	}
	if !strings.Contains(err.Error(), "not an account key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKeyFromSeed_InvalidInput(t *testing.T) {
	_, err := OperatorKeyFromSeed("garbage")
	if err == nil {
		t.Fatal("expected error for invalid seed")
	}

	_, err = AccountKeyFromSeed("")
	if err == nil {
		t.Fatal("expected error for empty seed")
	}
}
