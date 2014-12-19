package main

type Verifier interface {
	Verify(string, Key, string) error
}
