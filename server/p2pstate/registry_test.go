package p2pstate

import (
	"testing"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/p2p"
)

func TestRegistryLifecycle(t *testing.T) {
	const sessionID = "session-1"
	const token = "token-1"
	Register(sessionID, token, time.Second)
	if !ValidateToken(sessionID, token) {
		t.Fatal("expected token to validate")
	}
	RecordObservation(sessionID, common.WORK_P2P_VISITOR, p2p.ProbeSample{
		ProbePort:       10206,
		ObservedAddr:    "1.1.1.1:4000",
		ServerReplyAddr: "1.1.1.1:4000",
	})
	RecordObservation(sessionID, common.WORK_P2P_VISITOR, p2p.ProbeSample{
		ProbePort:       10208,
		ObservedAddr:    "1.1.1.1:4004",
		ServerReplyAddr: "1.1.1.1:4004",
	})
	samples := GetObservations(sessionID, common.WORK_P2P_VISITOR)
	if len(samples) != 2 {
		t.Fatalf("len(samples) = %d, want 2", len(samples))
	}
	if samples[0].ProbePort != 10206 || samples[1].ProbePort != 10208 {
		t.Fatalf("unexpected sample ordering %#v", samples)
	}
	Unregister(sessionID)
	if ValidateToken(sessionID, token) {
		t.Fatal("token should be invalid after unregister")
	}
	if samples := GetObservations(sessionID, common.WORK_P2P_VISITOR); len(samples) != 0 {
		t.Fatalf("observations should be empty after unregister, got %#v", samples)
	}
}
