package main

type verifier interface {
	Verify(string, key, string) error
	VerifyKey(key) error
}
