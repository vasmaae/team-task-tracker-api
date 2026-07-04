package ratelimit

import "testing"

func TestLimiterAllowsBurstAndSeparatesKeys(t *testing.T) {
	limiter := New(1)
	if !limiter.Allow("user:1") {
		t.Fatal("first request should be allowed")
	}
	if limiter.Allow("user:1") {
		t.Fatal("second request should be limited")
	}
	if !limiter.Allow("user:2") {
		t.Fatal("different key should have an independent bucket")
	}
}
