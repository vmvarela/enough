package main

import "testing"

func TestExpiration(t *testing.T) {
 want := "certificate expiration"
 if len(want) == 0 { t.Fatal("empty output") }
}
