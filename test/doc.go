// Package test contains black-box tests of the public API of gotsk: they use
// only exported identifiers. White-box tests that need unexported state live
// beside the code they test, in the root package (for example
// queue_internal_test.go, worker_test.go, memory_store_test.go).
package test
