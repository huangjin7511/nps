package p2p

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	pionstun "github.com/pion/stun/v3"
)

func resetPredictionHistoryForTest() {
	globalPredictionHistory.mu.Lock()
	defer globalPredictionHistory.mu.Unlock()
	globalPredictionHistory.entries = make(map[string]*predictionHistoryEntry)
}

func TestBuildNatObservation(t *testing.T) {
	obs := BuildNatObservation([]ProbeSample{
		{ProbePort: 10206, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "10.0.0.1:10206"},
		{ProbePort: 10207, ObservedAddr: "1.1.1.1:4002", ServerReplyAddr: "10.0.0.2:10207"},
		{ProbePort: 10208, ObservedAddr: "1.1.1.1:4004", ServerReplyAddr: "10.0.0.3:10208"},
	}, true)
	if obs.PublicIP != "1.1.1.1" {
		t.Fatalf("PublicIP = %q, want 1.1.1.1", obs.PublicIP)
	}
	if obs.ObservedBasePort != 4000 {
		t.Fatalf("ObservedBasePort = %d, want 4000", obs.ObservedBasePort)
	}
	if obs.ObservedInterval != 2 {
		t.Fatalf("ObservedInterval = %d, want 2", obs.ObservedInterval)
	}
	if obs.ProbePortRestricted {
		t.Fatal("extra reply should keep ProbePortRestricted=false")
	}
	if obs.ProbeIPCount != 3 {
		t.Fatalf("ProbeIPCount = %d, want 3", obs.ProbeIPCount)
	}
	if obs.MappingConfidenceLow {
		t.Fatal("multi-probe-ip regular samples should not be low confidence")
	}
	if obs.MappingBehavior != NATMappingEndpointDependent || obs.NATType != NATTypeSymmetric {
		t.Fatalf("unexpected nat classification %#v", obs)
	}
}

func TestBuildNatObservationLowConfidence(t *testing.T) {
	obs := BuildNatObservation([]ProbeSample{
		{ProbePort: 10206, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "10.0.0.1:10206"},
	}, false)
	if !obs.ProbePortRestricted {
		t.Fatal("missing extra reply should mark probe port restricted")
	}
	if !obs.MappingConfidenceLow {
		t.Fatal("single sample should stay low confidence")
	}
}

func TestBuildNatObservationSingleProbeIPKeepsLowConfidence(t *testing.T) {
	obs := BuildNatObservation([]ProbeSample{
		{ProbePort: 10206, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "10.0.0.1:10206"},
		{ProbePort: 10207, ObservedAddr: "1.1.1.1:4002", ServerReplyAddr: "10.0.0.1:10207"},
		{ProbePort: 10208, ObservedAddr: "1.1.1.1:4004", ServerReplyAddr: "10.0.0.1:10208"},
	}, true)
	if obs.ProbeIPCount != 1 {
		t.Fatalf("ProbeIPCount = %d, want 1", obs.ProbeIPCount)
	}
	if !obs.MappingConfidenceLow {
		t.Fatal("single probe ip should keep mapping confidence low")
	}
	if obs.NATType != NATTypeUnknown {
		t.Fatalf("single probe ip nat type = %q, want unknown", obs.NATType)
	}
}

func TestBuildNatObservationConflictingSignalsStayLowConfidence(t *testing.T) {
	obs := BuildNatObservation([]ProbeSample{
		{ProbePort: 10206, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "10.0.0.1:10206"},
		{ProbePort: 10207, ObservedAddr: "1.1.1.1:4002", ServerReplyAddr: "10.0.0.2:10207"},
		{ProbePort: 10208, ObservedAddr: "1.1.1.1:4004", ServerReplyAddr: "10.0.0.3:10208"},
	}, false)
	if !obs.ConflictingSignals {
		t.Fatal("strict filtering with patterned mapping should be marked conflicting")
	}
	if !obs.MappingConfidenceLow {
		t.Fatal("conflicting signals should stay low confidence")
	}
}

func TestBuildNatObservationClassifiesConeWhenSTUNAndNPSAgree(t *testing.T) {
	obs := BuildNatObservation([]ProbeSample{
		{EndpointID: "stun-1", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, ProbePort: 3478, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "198.51.100.10:3478"},
		{EndpointID: "stun-2", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, ProbePort: 3478, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "198.51.100.11:3478"},
		{ProbePort: 10206, Provider: ProbeProviderNPS, Mode: ProbeModeUDP, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "203.0.113.10:10206"},
	}, true)
	if obs.NATType != NATTypeCone || obs.MappingBehavior != NATMappingEndpointIndependent || obs.FilteringBehavior != NATFilteringOpen {
		t.Fatalf("unexpected nat classification %#v", obs)
	}
	if obs.MappingConfidenceLow {
		t.Fatalf("cone classification should not stay low confidence %#v", obs)
	}
}

func TestBuildNatObservationClassifiesPortRestrictedWhenMappingStableAndExtraReplyFails(t *testing.T) {
	obs := BuildNatObservation([]ProbeSample{
		{EndpointID: "stun-1", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, ProbePort: 3478, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "198.51.100.10:3478"},
		{EndpointID: "stun-2", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, ProbePort: 3478, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "198.51.100.11:3478"},
		{ProbePort: 10206, Provider: ProbeProviderNPS, Mode: ProbeModeUDP, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "203.0.113.10:10206"},
	}, false)
	if obs.NATType != NATTypePortRestricted || obs.FilteringBehavior != NATFilteringPortRestricted {
		t.Fatalf("unexpected nat classification %#v", obs)
	}
}

func TestMergeProbeSamplesPreservesClientReplyAndSTUNSamples(t *testing.T) {
	serverSamples := []ProbeSample{
		{
			Provider:        ProbeProviderNPS,
			Mode:            ProbeModeUDP,
			ProbePort:       10206,
			ObservedAddr:    "1.1.1.1:4000",
			ServerReplyAddr: "0.0.0.0:10206",
		},
	}
	clientSamples := []ProbeSample{
		{
			Provider:        ProbeProviderNPS,
			Mode:            ProbeModeUDP,
			ProbePort:       10206,
			ObservedAddr:    "1.1.1.1:4000",
			ServerReplyAddr: "203.0.113.10:10206",
			ExtraReply:      true,
		},
		{
			EndpointID:      "stun-1",
			Provider:        ProbeProviderSTUN,
			Mode:            ProbeModeBinding,
			ProbePort:       3478,
			ObservedAddr:    "1.1.1.1:4010",
			ServerReplyAddr: "198.51.100.20:3478",
		},
	}
	merged := MergeProbeSamples(serverSamples, clientSamples)
	if len(merged) != 2 {
		t.Fatalf("len(merged) = %d, want 2 (%#v)", len(merged), merged)
	}
	if merged[0].Provider != ProbeProviderNPS || merged[0].ServerReplyAddr != "203.0.113.10:10206" || !merged[0].ExtraReply {
		t.Fatalf("unexpected merged NPS sample %#v", merged[0])
	}
	if merged[1].Provider != ProbeProviderSTUN || merged[1].ObservedAddr != "1.1.1.1:4010" {
		t.Fatalf("unexpected merged STUN sample %#v", merged[1])
	}
	obs := BuildNatObservation(merged, true)
	if obs.ProbeIPCount != 2 {
		t.Fatalf("ProbeIPCount = %d, want 2", obs.ProbeIPCount)
	}
}

func TestNormalizeProbeEndpoints(t *testing.T) {
	endpoints := NormalizeProbeEndpoints(P2PProbeConfig{
		Provider: ProbeProviderNPS,
		Mode:     ProbeModeUDP,
		Network:  ProbeNetworkUDP,
		Endpoints: []P2PProbeEndpoint{
			{ID: "probe-1", Address: "1.1.1.1:10206"},
			{ID: "stun-1", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, Address: "stun.example.com:3478"},
		},
	})
	if len(endpoints) != 2 {
		t.Fatalf("len(endpoints) = %d, want 2", len(endpoints))
	}
	if endpoints[0].Provider != ProbeProviderNPS || endpoints[0].Mode != ProbeModeUDP || endpoints[0].Network != ProbeNetworkUDP {
		t.Fatalf("unexpected normalized NPS endpoint %#v", endpoints[0])
	}
	if endpoints[1].Provider != ProbeProviderSTUN || endpoints[1].Mode != ProbeModeBinding || endpoints[1].Network != ProbeNetworkUDP {
		t.Fatalf("unexpected normalized STUN endpoint %#v", endpoints[1])
	}
}

func TestWithDefaultSTUNEndpointsAddsDefaultsOnlyWhenMissing(t *testing.T) {
	probe := P2PProbeConfig{
		Provider: ProbeProviderNPS,
		Mode:     ProbeModeUDP,
		Network:  ProbeNetworkUDP,
		Endpoints: []P2PProbeEndpoint{
			{ID: "probe-1", Address: "1.1.1.1:10206"},
		},
	}
	withDefaults := WithDefaultSTUNEndpoints(probe, []string{"stun.example.com:3478", "stun2.example.com:3478"})
	if len(withDefaults.Endpoints) != 3 {
		t.Fatalf("len(withDefaults.Endpoints) = %d, want 3", len(withDefaults.Endpoints))
	}
	if !HasProbeProvider(withDefaults, ProbeProviderSTUN) {
		t.Fatal("expected default STUN endpoints to be appended")
	}
	serverProvided := WithDefaultSTUNEndpoints(P2PProbeConfig{
		Provider: ProbeProviderNPS,
		Mode:     ProbeModeUDP,
		Network:  ProbeNetworkUDP,
		Endpoints: []P2PProbeEndpoint{
			{ID: "probe-1", Address: "1.1.1.1:10206"},
			{ID: "stun-1", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, Network: ProbeNetworkUDP, Address: "stun.server:3478"},
		},
	}, []string{"stun.default:3478"})
	if len(serverProvided.Endpoints) != 2 {
		t.Fatalf("server-provided STUN list should stay unchanged, got %#v", serverProvided.Endpoints)
	}
}

func TestSelectPunchPlanPredictionDefaults(t *testing.T) {
	plan := SelectPunchPlan(
		P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}},
		P2PPeerInfo{Nat: NatObservation{ObservedBasePort: 5000, ObservedInterval: 0}},
		DefaultTimeouts(),
	)
	if !plan.EnablePrediction {
		t.Fatal("restricted/low-confidence observation should enable prediction")
	}
	if !plan.EnableAggressivePrediction {
		t.Fatal("default main path should keep aggressive prediction enabled")
	}
	if !plan.UseSingleLocalSocket {
		t.Fatal("single local socket must stay enabled")
	}
	if !plan.UseTargetSpray {
		t.Fatal("default main path should use target spray")
	}
	if plan.UseBirthdayAttack {
		t.Fatal("birthday attack must stay disabled by default")
	}
	if plan.AllowBirthdayFallback {
		t.Fatal("single-sided hard NAT should stay on the default target-spray main path")
	}
	if len(plan.PredictionIntervals) == 0 || plan.PredictionIntervals[0] != DefaultPredictionInterval {
		t.Fatalf("unexpected PredictionIntervals %#v", plan.PredictionIntervals)
	}
	if plan.PredictionInterval != DefaultPredictionInterval {
		t.Fatalf("PredictionInterval = %d, want %d", plan.PredictionInterval, DefaultPredictionInterval)
	}
	if plan.HandshakeTimeout != 20*time.Second {
		t.Fatalf("HandshakeTimeout = %s, want 20s", plan.HandshakeTimeout)
	}
}

func TestSelectPunchPlanSingleProbeIPLowConfidenceTriggersPrediction(t *testing.T) {
	self := P2PPeerInfo{Nat: BuildNatObservation([]ProbeSample{
		{ProbePort: 10206, ObservedAddr: "1.1.1.1:4000", ServerReplyAddr: "10.0.0.1:10206"},
		{ProbePort: 10207, ObservedAddr: "1.1.1.1:4002", ServerReplyAddr: "10.0.0.1:10207"},
		{ProbePort: 10208, ObservedAddr: "1.1.1.1:4004", ServerReplyAddr: "10.0.0.1:10208"},
	}, true)}
	peer := P2PPeerInfo{Nat: NatObservation{ObservedBasePort: 5000, ObservedInterval: 2}}
	plan := SelectPunchPlan(self, peer, DefaultTimeouts())
	if !self.Nat.MappingConfidenceLow {
		t.Fatal("single probe ip should be low confidence")
	}
	if !plan.EnablePrediction || !plan.UseTargetSpray {
		t.Fatalf("unexpected plan %#v", plan)
	}
	if len(plan.PredictionIntervals) == 0 {
		t.Fatal("prediction should build interval candidates")
	}
}

func TestSelectPunchPlanAllowsBirthdayFallbackForHardPeers(t *testing.T) {
	plan := SelectPunchPlan(
		P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}},
		P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true, ObservedBasePort: 5000}},
		DefaultTimeouts(),
	)
	if !plan.AllowBirthdayFallback {
		t.Fatal("dual hard/unknown peers should allow birthday fallback after target spray failure")
	}
	if plan.UseBirthdayAttack {
		t.Fatal("birthday attack must still be off until the target-spray path fails")
	}
}

func TestSelectPunchPlanEnablesPredictionForClassifiedSymmetricPeer(t *testing.T) {
	plan := SelectPunchPlan(
		P2PPeerInfo{Nat: NatObservation{NATType: NATTypeSymmetric, MappingBehavior: NATMappingEndpointDependent}},
		P2PPeerInfo{Nat: NatObservation{ObservedBasePort: 5000, NATType: NATTypeCone}},
		DefaultTimeouts(),
	)
	if !plan.EnablePrediction || !plan.UseTargetSpray {
		t.Fatalf("classified symmetric nat should still enable prediction, got %#v", plan)
	}
}

func TestSelectPunchPlanKeepsBirthdayFallbackDisabledForLoosePeers(t *testing.T) {
	plan := SelectPunchPlan(
		P2PPeerInfo{Nat: NatObservation{ObservedBasePort: 5000, ObservedInterval: 2}},
		P2PPeerInfo{Nat: NatObservation{ObservedBasePort: 6000, ObservedInterval: 2}},
		DefaultTimeouts(),
	)
	if plan.AllowBirthdayFallback {
		t.Fatal("loose peers should not enable birthday fallback")
	}
	if plan.UseBirthdayAttack {
		t.Fatal("birthday attack must not be enabled directly by default")
	}
}

func TestApplyProbePlanOptionsRespectsBirthdayDefaultOff(t *testing.T) {
	plan := ApplyProbePlanOptions(SelectPunchPlan(
		P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}},
		P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}},
		DefaultTimeouts(),
	), P2PProbeConfig{
		Options: map[string]string{
			"enable_birthday_attack": "false",
		},
	}, P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}}, P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}})
	if plan.AllowBirthdayFallback {
		t.Fatal("birthday fallback should stay disabled when config option is false")
	}
}

func TestApplyProbePlanOptionsOverridesSprayConfig(t *testing.T) {
	plan := ApplyProbePlanOptions(SelectPunchPlan(
		P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}},
		P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}},
		DefaultTimeouts(),
	), P2PProbeConfig{
		Options: map[string]string{
			"enable_target_spray":          "false",
			"enable_birthday_attack":       "true",
			"default_prediction_interval":  "3",
			"target_spray_span":            "12",
			"target_spray_rounds":          "4",
			"target_spray_burst":           "2",
			"target_spray_packet_sleep_ms": "7",
			"target_spray_burst_gap_ms":    "11",
			"target_spray_phase_gap_ms":    "13",
			"birthday_listen_ports":        "6",
			"birthday_targets_per_port":    "9",
		},
	}, P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}}, P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true, MappingConfidenceLow: true}})
	if plan.UseTargetSpray {
		t.Fatal("enable_target_spray=false should disable target spray")
	}
	if !plan.AllowBirthdayFallback {
		t.Fatal("enable_birthday_attack=true should preserve birthday fallback availability")
	}
	if plan.PredictionInterval != 3 || plan.SpraySpan != 12 || plan.SprayRounds != 4 || plan.SprayBurst != 2 {
		t.Fatalf("unexpected spray config %#v", plan)
	}
	if plan.SprayPerPacketSleep != 7*time.Millisecond || plan.SprayBurstGap != 11*time.Millisecond || plan.SprayPhaseGap != 13*time.Millisecond {
		t.Fatalf("unexpected spray timings %#v", plan)
	}
	if plan.BirthdayListenPorts != 6 || plan.BirthdayTargetsPerPort != 9 {
		t.Fatalf("unexpected birthday config %#v", plan)
	}
}

func TestApplyProbePlanOptionsKeepsBirthdayFallbackForSymmetricPeersWhenEnabled(t *testing.T) {
	self := P2PPeerInfo{Nat: NatObservation{NATType: NATTypeSymmetric, MappingBehavior: NATMappingEndpointDependent}}
	peer := P2PPeerInfo{Nat: NatObservation{NATType: NATTypeSymmetric, MappingBehavior: NATMappingEndpointDependent, ObservedBasePort: 5000}}
	plan := ApplyProbePlanOptions(SelectPunchPlan(self, peer, DefaultTimeouts()), P2PProbeConfig{
		Options: map[string]string{
			"enable_birthday_attack": "true",
		},
	}, self, peer)
	if !plan.AllowBirthdayFallback {
		t.Fatal("symmetric peers should keep birthday fallback when config enables it")
	}
	if plan.UseBirthdayAttack {
		t.Fatal("birthday attack should still wait for target spray failure before enabling")
	}
}

func TestBuildPredictionIntervals(t *testing.T) {
	intervals := BuildPredictionIntervals(NatObservation{
		ObservedInterval: 4,
		Samples: []ProbeSample{
			{ObservedAddr: "1.1.1.1:4000"},
			{ObservedAddr: "1.1.1.1:4004"},
			{ObservedAddr: "1.1.1.1:4008"},
		},
	}, true, true)
	if len(intervals) < 3 {
		t.Fatalf("expected multiple intervals, got %#v", intervals)
	}
	if intervals[0] != 4 {
		t.Fatalf("primary interval = %d, want 4", intervals[0])
	}
	foundOne := false
	for _, interval := range intervals {
		if interval == 1 {
			foundOne = true
			break
		}
	}
	if !foundOne {
		t.Fatalf("interval candidates should include normalized fallback 1, got %#v", intervals)
	}
}

func TestApplyProbePlanOptionsCanDisableForcedPrediction(t *testing.T) {
	self := P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: true}}
	peer := P2PPeerInfo{Nat: NatObservation{ProbePortRestricted: false, Samples: []ProbeSample{{ObservedAddr: "1.1.1.1:5000"}}}}
	plan := ApplyProbePlanOptions(SelectPunchPlan(self, peer, DefaultTimeouts()), P2PProbeConfig{
		Options: map[string]string{
			"force_predict_on_restricted": "false",
		},
	}, self, peer)
	if plan.EnablePrediction {
		t.Fatal("force_predict_on_restricted=false should disable prediction when low confidence is absent")
	}
	if plan.UseTargetSpray {
		t.Fatal("target spray should be disabled when forced prediction is turned off and peer samples exist")
	}
}

func TestBuildTargetSprayPorts(t *testing.T) {
	ports := BuildTargetSprayPorts(5000, 2, 6)
	wantHead := []int{5000, 5002, 4998}
	for i, want := range wantHead {
		if ports[i] != want {
			t.Fatalf("ports[%d] = %d, want %d (%#v)", i, ports[i], want, ports)
		}
	}
	for _, port := range ports {
		if port < 1 || port > 65535 {
			t.Fatalf("invalid port %d in %#v", port, ports)
		}
	}
}

func TestBuildTargetSprayPortsNormalizesInterval(t *testing.T) {
	ports := BuildTargetSprayPorts(10, 0, 4)
	if len(ports) != 4 {
		t.Fatalf("len(ports) = %d, want 4", len(ports))
	}
	if ports[0] != 10 || ports[1] != 11 || ports[2] != 9 {
		t.Fatalf("unexpected interval-normalized result %#v", ports)
	}
}

func TestBuildTargetSprayPortsMulti(t *testing.T) {
	ports := BuildTargetSprayPortsMulti(5000, []int{4, 2, 1}, 8)
	if len(ports) != 8 {
		t.Fatalf("len(ports) = %d, want 8 (%#v)", len(ports), ports)
	}
	if ports[0] != 5000 {
		t.Fatalf("ports[0] = %d, want 5000", ports[0])
	}
	expected := map[int]bool{5004: false, 4996: false, 5002: false, 4998: false}
	for _, port := range ports {
		if _, ok := expected[port]; ok {
			expected[port] = true
		}
	}
	for port, seen := range expected {
		if !seen {
			t.Fatalf("expected multi-interval spray to include %d in %#v", port, ports)
		}
	}
}

func TestBuildPunchTargetsFiltersAndDedupes(t *testing.T) {
	peer := P2PPeerInfo{
		Nat: NatObservation{
			PublicIP:         "1.1.1.1",
			ObservedBasePort: 5000,
			ObservedInterval: 2,
			Samples: []ProbeSample{
				{ObservedAddr: "1.1.1.1:5000"},
				{ObservedAddr: "1.1.1.1:5002"},
			},
		},
		LocalAddrs: []string{
			"192.168.0.2:5000",
			"[2001:db8::1]:5000",
			"192.168.0.2:5000",
		},
	}
	targets := BuildPunchTargets(peer, PunchPlan{
		PredictionInterval: 2,
		SpraySpan:          6,
	})
	if len(targets) == 0 {
		t.Fatal("expected punch targets")
	}
	for _, target := range targets {
		if target == "[2001:db8::1]:5000" {
			t.Fatalf("ipv6 local addr should be filtered for ipv4 peer: %#v", targets)
		}
	}
	if targets[0] != "1.1.1.1:5000" {
		t.Fatalf("first target = %q, want first observed addr", targets[0])
	}
}

func TestBuildDirectPunchTargetsFiltersAndDedupes(t *testing.T) {
	peer := P2PPeerInfo{
		Nat: NatObservation{
			PublicIP: "1.1.1.1",
			Samples: []ProbeSample{
				{ObservedAddr: "1.1.1.1:5000"},
			},
		},
		LocalAddrs: []string{
			"192.168.0.2:5000",
			"[2001:db8::1]:5000",
			"192.168.0.2:5000",
		},
	}
	targets := BuildDirectPunchTargets(peer)
	if len(targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2 (%#v)", len(targets), targets)
	}
	if targets[0] != "1.1.1.1:5000" || targets[1] != "192.168.0.2:5000" {
		t.Fatalf("unexpected direct targets %#v", targets)
	}
}

func TestBuildDirectPunchTargetsPrefersPortMapping(t *testing.T) {
	peer := P2PPeerInfo{
		Nat: NatObservation{
			PortMapping: &PortMappingInfo{ExternalAddr: "2.2.2.2:6000"},
			Samples: []ProbeSample{
				{ObservedAddr: "1.1.1.1:5000"},
			},
		},
		LocalAddrs: []string{"192.168.0.2:5000"},
	}
	targets := BuildDirectPunchTargets(peer)
	if len(targets) == 0 || targets[0] != "2.2.2.2:6000" {
		t.Fatalf("port mapping target should be first, got %#v", targets)
	}
}

func TestBuildPreferredDirectPunchTargetsPrefersMatchingPeerLocalAddr(t *testing.T) {
	self := P2PPeerInfo{
		LocalAddrs: []string{
			"192.168.0.10:4000",
		},
	}
	peer := P2PPeerInfo{
		Nat: NatObservation{
			PortMapping: &PortMappingInfo{ExternalAddr: "2.2.2.2:6000"},
			Samples: []ProbeSample{
				{ObservedAddr: "1.1.1.1:5000"},
			},
		},
		LocalAddrs: []string{
			"192.168.0.20:5000",
			"[2001:db8::20]:5000",
		},
	}
	targets := BuildPreferredDirectPunchTargets(self, peer)
	if len(targets) == 0 {
		t.Fatal("expected preferred direct targets")
	}
	if targets[0] != "192.168.0.20:5000" {
		t.Fatalf("first preferred direct target = %q, want peer ipv4 local addr (%#v)", targets[0], targets)
	}
	for _, target := range targets {
		if target == "[2001:db8::20]:5000" {
			t.Fatalf("ipv6 peer local addr should be skipped when self only supports ipv4: %#v", targets)
		}
	}
}

func TestBuildPreferredPunchTargetsKeepsDirectCandidatesBeforePrediction(t *testing.T) {
	self := P2PPeerInfo{
		LocalAddrs: []string{"192.168.0.10:4000"},
	}
	peer := P2PPeerInfo{
		Nat: NatObservation{
			PublicIP:         "1.1.1.1",
			ObservedBasePort: 5000,
			ObservedInterval: 2,
			Samples: []ProbeSample{
				{ObservedAddr: "1.1.1.1:5000"},
			},
		},
		LocalAddrs: []string{"192.168.0.20:5000"},
	}
	targets := BuildPreferredPunchTargets(self, peer, PunchPlan{
		PredictionIntervals: []int{2},
		SpraySpan:           6,
	})
	if len(targets) < 3 {
		t.Fatalf("expected enough targets, got %#v", targets)
	}
	if targets[0] != "192.168.0.20:5000" {
		t.Fatalf("first target = %q, want direct local addr first (%#v)", targets[0], targets)
	}
	if targets[1] != "1.1.1.1:5000" {
		t.Fatalf("second target = %q, want observed addr second (%#v)", targets[1], targets)
	}
}

func TestBuildPreferredPunchTargetsUsesPredictionHistoryBeforeDefaultSpray(t *testing.T) {
	resetPredictionHistoryForTest()
	t.Cleanup(resetPredictionHistoryForTest)

	self := P2PPeerInfo{
		LocalAddrs: []string{"192.168.0.10:4000"},
	}
	peer := P2PPeerInfo{
		Nat: NatObservation{
			PublicIP:         "1.1.1.1",
			ObservedBasePort: 5000,
			ObservedInterval: 2,
			NATType:          NATTypeSymmetric,
			MappingBehavior:  NATMappingEndpointDependent,
			Samples: []ProbeSample{
				{ObservedAddr: "1.1.1.1:5000"},
			},
		},
	}
	recordPredictionSuccess(peer.Nat, "1.1.1.1:5006")
	recordPredictionSuccess(peer.Nat, "1.1.1.1:5006")
	recordPredictionSuccess(peer.Nat, "1.1.1.1:5008")

	targets := BuildPreferredPunchTargets(self, peer, PunchPlan{
		PredictionIntervals: []int{2},
		SpraySpan:           8,
	})
	if len(targets) < 3 {
		t.Fatalf("expected enough targets, got %#v", targets)
	}
	if targets[0] != "1.1.1.1:5000" {
		t.Fatalf("first target = %q, want observed addr first (%#v)", targets[0], targets)
	}
	if targets[1] != "1.1.1.1:5006" {
		t.Fatalf("second target = %q, want historical offset first (%#v)", targets[1], targets)
	}
}

func TestBuildBirthdayPunchTargets(t *testing.T) {
	peer := P2PPeerInfo{
		Nat: NatObservation{
			PublicIP:         "1.1.1.1",
			ObservedBasePort: 5000,
			ObservedInterval: 2,
			Samples: []ProbeSample{
				{ObservedAddr: "1.1.1.1:5000"},
			},
		},
	}
	targets := BuildBirthdayPunchTargets(peer, PunchPlan{
		PredictionInterval:     2,
		BirthdayTargetsPerPort: 4,
	})
	if len(targets) < 4 {
		t.Fatalf("expected birthday targets, got %#v", targets)
	}
	if targets[0] != "1.1.1.1:5000" {
		t.Fatalf("first birthday target = %q", targets[0])
	}
}

func TestUDPPacketTokenValidation(t *testing.T) {
	packet := newUDPPacket("session-1", "token-1", common.WORK_P2P_VISITOR, packetTypeProbe)
	raw, err := EncodeUDPPacket(packet)
	if err != nil {
		t.Fatalf("EncodeUDPPacket() error = %v", err)
	}
	decoded, err := DecodeUDPPacket(raw, "token-1")
	if err != nil {
		t.Fatalf("DecodeUDPPacket() error = %v", err)
	}
	if decoded.SessionID != "session-1" || decoded.Token != "token-1" {
		t.Fatalf("decoded packet mismatch: %#v", decoded)
	}
	var tampered map[string]any
	if err := json.Unmarshal(raw, &tampered); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	tampered["timestamp"] = float64(decoded.Timestamp + 1)
	tamperedRaw, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if _, err := DecodeUDPPacket(tamperedRaw, "token-1"); !errors.Is(err, ErrP2PTokenMismatch) {
		t.Fatalf("expected ErrP2PTokenMismatch, got %v", err)
	}
	if _, err := DecodeUDPPacket(raw, "wrong-token"); !errors.Is(err, ErrP2PTokenMismatch) {
		t.Fatalf("expected ErrP2PTokenMismatch for wrong token, got %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("json.Unmarshal(raw) error = %v", err)
	}
	if _, ok := envelope["token"]; ok {
		t.Fatalf("wire packet should not expose token: %#v", envelope)
	}
}

func TestRunSTUNProbeEndpoint(t *testing.T) {
	serverConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(server) error = %v", err)
	}
	defer func() { _ = serverConn.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1500)
		n, addr, err := serverConn.ReadFrom(buf)
		if err != nil {
			return
		}
		raw, err := buildFakeSTUNBindingResponse(buf[:n], addr.(*net.UDPAddr))
		if err != nil {
			t.Errorf("buildFakeSTUNBindingResponse() error = %v", err)
			return
		}
		_, _ = serverConn.WriteTo(raw, addr)
	}()

	localConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(local) error = %v", err)
	}
	defer func() { _ = localConn.Close() }()

	sample, err := runSTUNProbeEndpoint(context.Background(), localConn, P2PProbeEndpoint{
		ID:       "stun-1",
		Provider: ProbeProviderSTUN,
		Mode:     ProbeModeBinding,
		Network:  ProbeNetworkUDP,
		Address:  serverConn.LocalAddr().String(),
	}, 300*time.Millisecond)
	<-done
	if err != nil {
		t.Fatalf("runSTUNProbeEndpoint() error = %v", err)
	}
	if sample.Provider != ProbeProviderSTUN || sample.Mode != ProbeModeBinding {
		t.Fatalf("unexpected STUN sample %#v", sample)
	}
	if common.GetPortByAddr(sample.ObservedAddr) != common.GetPortByAddr(localConn.LocalAddr().String()) {
		t.Fatalf("ObservedAddr port = %d, want %d", common.GetPortByAddr(sample.ObservedAddr), common.GetPortByAddr(localConn.LocalAddr().String()))
	}
	if sample.ServerReplyAddr != serverConn.LocalAddr().String() {
		t.Fatalf("ServerReplyAddr = %q, want %q", sample.ServerReplyAddr, serverConn.LocalAddr().String())
	}
}

func TestRunSTUNCalibrationFallsBackToNextEndpoint(t *testing.T) {
	silentConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(silent) error = %v", err)
	}
	defer func() { _ = silentConn.Close() }()
	serverConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(server) error = %v", err)
	}
	defer func() { _ = serverConn.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1500)
		n, addr, err := serverConn.ReadFrom(buf)
		if err != nil {
			return
		}
		raw, err := buildFakeSTUNBindingResponse(buf[:n], addr.(*net.UDPAddr))
		if err != nil {
			t.Errorf("buildFakeSTUNBindingResponse() error = %v", err)
			return
		}
		_, _ = serverConn.WriteTo(raw, addr)
	}()
	localConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(local) error = %v", err)
	}
	defer func() { _ = localConn.Close() }()
	sample, err := runSTUNCalibration(context.Background(), localConn, []P2PProbeEndpoint{
		{ID: "stun-1", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, Network: ProbeNetworkUDP, Address: silentConn.LocalAddr().String(), Options: map[string]string{"timeout_ms": "120"}},
		{ID: "stun-2", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, Network: ProbeNetworkUDP, Address: serverConn.LocalAddr().String(), Options: map[string]string{"timeout_ms": "120"}},
	}, 1000)
	<-done
	if err != nil {
		t.Fatalf("runSTUNCalibration() error = %v", err)
	}
	if sample.EndpointID != "stun-2" {
		t.Fatalf("EndpointID = %q, want stun-2", sample.EndpointID)
	}
}

func TestRunSTUNProbesDiscoversOtherAddress(t *testing.T) {
	server1, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(server1) error = %v", err)
	}
	defer func() { _ = server1.Close() }()
	server2, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(server2) error = %v", err)
	}
	defer func() { _ = server2.Close() }()

	done1 := make(chan struct{})
	go func() {
		defer close(done1)
		buf := make([]byte, 1500)
		n, addr, err := server1.ReadFrom(buf)
		if err != nil {
			return
		}
		raw, err := buildFakeSTUNBindingResponseWithAttrs(buf[:n], addr.(*net.UDPAddr), &net.UDPAddr{IP: net.ParseIP("198.51.100.10"), Port: 3478}, server2.LocalAddr().(*net.UDPAddr))
		if err != nil {
			t.Errorf("buildFakeSTUNBindingResponseWithAttrs(server1) error = %v", err)
			return
		}
		_, _ = server1.WriteTo(raw, addr)
	}()
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		buf := make([]byte, 1500)
		n, addr, err := server2.ReadFrom(buf)
		if err != nil {
			return
		}
		raw, err := buildFakeSTUNBindingResponseWithAttrs(buf[:n], addr.(*net.UDPAddr), &net.UDPAddr{IP: net.ParseIP("198.51.100.11"), Port: 3478}, nil)
		if err != nil {
			t.Errorf("buildFakeSTUNBindingResponseWithAttrs(server2) error = %v", err)
			return
		}
		_, _ = server2.WriteTo(raw, addr)
	}()

	localConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(local) error = %v", err)
	}
	defer func() { _ = localConn.Close() }()

	samples, err := runSTUNProbes(context.Background(), localConn, []P2PProbeEndpoint{
		{ID: "stun-1", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, Network: ProbeNetworkUDP, Address: server1.LocalAddr().String(), Options: map[string]string{"timeout_ms": "150"}},
	}, 1000)
	<-done1
	<-done2
	if err != nil {
		t.Fatalf("runSTUNProbes() error = %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("len(samples) = %d, want 2 (%#v)", len(samples), samples)
	}
	if samples[0].ServerReplyAddr != "198.51.100.10:3478" {
		t.Fatalf("samples[0].ServerReplyAddr = %q, want %q", samples[0].ServerReplyAddr, "198.51.100.10:3478")
	}
	if samples[1].ServerReplyAddr != "198.51.100.11:3478" {
		t.Fatalf("samples[1].ServerReplyAddr = %q, want %q", samples[1].ServerReplyAddr, "198.51.100.11:3478")
	}
	obs := BuildNatObservation(samples, true)
	if obs.ProbeIPCount != 2 {
		t.Fatalf("ProbeIPCount = %d, want 2", obs.ProbeIPCount)
	}
	if obs.MappingBehavior != NATMappingEndpointIndependent {
		t.Fatalf("MappingBehavior = %q, want %q (%#v)", obs.MappingBehavior, NATMappingEndpointIndependent, obs)
	}
}

func TestRunSTUNProbeEndpointTimesOutQuickly(t *testing.T) {
	silentConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(silent) error = %v", err)
	}
	defer func() { _ = silentConn.Close() }()
	localConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(local) error = %v", err)
	}
	defer func() { _ = localConn.Close() }()
	started := time.Now()
	_, err = runSTUNProbeEndpoint(context.Background(), localConn, P2PProbeEndpoint{
		ID:       "stun-1",
		Provider: ProbeProviderSTUN,
		Mode:     ProbeModeBinding,
		Network:  ProbeNetworkUDP,
		Address:  silentConn.LocalAddr().String(),
	}, 120*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout from silent STUN endpoint")
	}
	if elapsed := time.Since(started); elapsed > 400*time.Millisecond {
		t.Fatalf("stun timeout took too long: %s", elapsed)
	}
}

func TestRunProbeRequiresNPSEndpoint(t *testing.T) {
	localConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(local) error = %v", err)
	}
	defer func() { _ = localConn.Close() }()
	_, err = runProbe(context.Background(), localConn, P2PPunchStart{
		SessionID: "session-1",
		Token:     "token-1",
		Role:      common.WORK_P2P_VISITOR,
		Probe: P2PProbeConfig{
			Version:  2,
			Provider: ProbeProviderSTUN,
			Mode:     ProbeModeBinding,
			Network:  ProbeNetworkUDP,
			Endpoints: []P2PProbeEndpoint{
				{ID: "stun-1", Provider: ProbeProviderSTUN, Mode: ProbeModeBinding, Network: ProbeNetworkUDP, Address: "127.0.0.1:3478"},
			},
		},
		Timeouts: DefaultTimeouts(),
	})
	if err == nil || !strings.Contains(err.Error(), "missing required nps probe endpoints") {
		t.Fatalf("expected missing nps probe endpoint error, got %v", err)
	}
}

func TestCompatibleNPSProbeEndpointsFiltersBySocketFamily(t *testing.T) {
	v4Conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(v4) error = %v", err)
	}
	defer func() { _ = v4Conn.Close() }()

	endpoints := compatibleNPSProbeEndpoints(v4Conn.LocalAddr(), []P2PProbeEndpoint{
		{ID: "v4", Provider: ProbeProviderNPS, Mode: ProbeModeUDP, Network: ProbeNetworkUDP, Address: "127.0.0.1:10206"},
		{ID: "v6", Provider: ProbeProviderNPS, Mode: ProbeModeUDP, Network: ProbeNetworkUDP, Address: "[::1]:10206"},
		{ID: "host", Provider: ProbeProviderNPS, Mode: ProbeModeUDP, Network: ProbeNetworkUDP, Address: "probe.example.test:10206"},
	})
	if len(endpoints) != 2 {
		t.Fatalf("len(endpoints) = %d, want 2 (%#v)", len(endpoints), endpoints)
	}
	if endpoints[0].ID != "v4" || endpoints[1].ID != "host" {
		t.Fatalf("unexpected compatible endpoints %#v", endpoints)
	}
	if network := probeResolveNetwork(v4Conn.LocalAddr()); network != "udp4" {
		t.Fatalf("probeResolveNetwork(v4) = %q, want udp4", network)
	}
}

func TestPeriodicSprayDelayBackoffAndCap(t *testing.T) {
	if delay := periodicSprayDelay(0); delay != 1500*time.Millisecond {
		t.Fatalf("attempt 0 delay = %s, want 1500ms", delay)
	}
	if delay := periodicSprayDelay(1); delay != 2100*time.Millisecond {
		t.Fatalf("attempt 1 delay = %s, want 2100ms", delay)
	}
	if delay := periodicSprayDelay(2); delay != 2400*time.Millisecond {
		t.Fatalf("attempt 2 delay = %s, want 2400ms", delay)
	}
	if delay := periodicSprayDelay(6); delay != 4*time.Second {
		t.Fatalf("attempt 6 delay = %s, want capped 4s", delay)
	}
}

func TestRuntimeSessionStartSprayEnablesBirthdayFallbackAfterTargetSpray(t *testing.T) {
	localConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(local) error = %v", err)
	}
	defer func() { _ = localConn.Close() }()
	targetConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(target) error = %v", err)
	}
	defer func() { _ = targetConn.Close() }()

	session := &runtimeSession{
		start:     P2PPunchStart{SessionID: "session-1", Token: "token-1", Role: common.WORK_P2P_VISITOR},
		summary:   P2PProbeSummary{Self: P2PPeerInfo{LocalAddrs: []string{localConn.LocalAddr().String()}}, Peer: P2PPeerInfo{Nat: NatObservation{PublicIP: "127.0.0.1", ObservedBasePort: common.GetPortByAddr(targetConn.LocalAddr().String()), Samples: []ProbeSample{{ObservedAddr: targetConn.LocalAddr().String()}}}}},
		plan:      PunchPlan{UseTargetSpray: true, AllowBirthdayFallback: true, SprayRounds: 1, SprayBurst: 1, SprayPerPacketSleep: 0, SprayBurstGap: 0, SprayPhaseGap: 0, BirthdayListenPorts: 2, BirthdayTargetsPerPort: 1, PredictionIntervals: []int{1}},
		localConn: localConn,
		sockets:   []net.PacketConn{localConn},
		cm:        NewCandidateManager(""),
		confirmed: make(chan confirmedPair, 1),
		nominate:  make(map[string]struct{}),
	}
	defer session.closeSockets(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	session.startSpray(ctx)

	if !session.plan.UseBirthdayAttack {
		t.Fatal("birthday fallback should be enabled after target spray fails")
	}
	if len(session.snapshotSockets()) < 2 {
		t.Fatalf("birthday fallback should open additional sockets, got %d", len(session.snapshotSockets()))
	}
}

func TestRuntimeSessionStartSprayDoesNotEnableBirthdayFallbackWhenDisabled(t *testing.T) {
	localConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(local) error = %v", err)
	}
	defer func() { _ = localConn.Close() }()
	targetConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(target) error = %v", err)
	}
	defer func() { _ = targetConn.Close() }()

	session := &runtimeSession{
		start:     P2PPunchStart{SessionID: "session-1", Token: "token-1", Role: common.WORK_P2P_VISITOR},
		summary:   P2PProbeSummary{Self: P2PPeerInfo{LocalAddrs: []string{localConn.LocalAddr().String()}}, Peer: P2PPeerInfo{Nat: NatObservation{PublicIP: "127.0.0.1", ObservedBasePort: common.GetPortByAddr(targetConn.LocalAddr().String()), Samples: []ProbeSample{{ObservedAddr: targetConn.LocalAddr().String()}}}}},
		plan:      PunchPlan{UseTargetSpray: true, AllowBirthdayFallback: false, SprayRounds: 1, SprayBurst: 1, SprayPerPacketSleep: 0, SprayBurstGap: 0, SprayPhaseGap: 0, PredictionIntervals: []int{1}},
		localConn: localConn,
		sockets:   []net.PacketConn{localConn},
		cm:        NewCandidateManager(""),
		confirmed: make(chan confirmedPair, 1),
		nominate:  make(map[string]struct{}),
	}
	defer session.closeSockets(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	session.startSpray(ctx)

	if session.plan.UseBirthdayAttack {
		t.Fatal("birthday fallback should stay disabled when AllowBirthdayFallback=false")
	}
	if len(session.snapshotSockets()) != 1 {
		t.Fatalf("disabled birthday fallback should keep one socket, got %d", len(session.snapshotSockets()))
	}
}

func TestReadBridgeJSONHandlesAbort(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		bridgeConn := conn.NewConn(server)
		_ = WriteBridgeMessage(bridgeConn, common.P2P_PUNCH_ABORT, P2PPunchAbort{
			SessionID: "session-1",
			Role:      common.WORK_P2P_VISITOR,
			Reason:    "probe failed",
		})
	}()

	_, err := ReadBridgeJSON[P2PProbeSummary](conn.NewConn(client), common.P2P_PROBE_SUMMARY)
	<-writeDone
	if !errors.Is(err, ErrP2PSessionAbort) {
		t.Fatalf("expected ErrP2PSessionAbort, got %v", err)
	}
}

func TestCandidateManagerConfirmedRemote(t *testing.T) {
	manager := NewCandidateManager("1.1.1.1:5000")
	manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	manager.MarkSucceeded("0.0.0.0:3000", "1.1.1.1:5002")
	manager.TryNominate("0.0.0.0:3000", "1.1.1.1:5002")
	manager.Confirm("0.0.0.0:3000", "1.1.1.1:5002")
	if got := manager.ConfirmedRemote(); got != "1.1.1.1:5002" {
		t.Fatalf("ConfirmedRemote() = %q, want %q", got, "1.1.1.1:5002")
	}
}

func TestCandidateManagerKeepsConfirmedRemoteStable(t *testing.T) {
	manager := NewCandidateManager("1.1.1.1:5000")
	manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	manager.MarkSucceeded("0.0.0.0:3000", "1.1.1.1:5002")
	manager.TryNominate("0.0.0.0:3000", "1.1.1.1:5002")
	manager.Confirm("0.0.0.0:3000", "1.1.1.1:5002")
	manager.Observe("0.0.0.0:3000", "1.1.1.1:5010")
	manager.MarkSucceeded("0.0.0.0:3000", "1.1.1.1:5010")
	manager.TryNominate("0.0.0.0:3000", "1.1.1.1:5010")
	if got := manager.ConfirmedRemote(); got != "1.1.1.1:5002" {
		t.Fatalf("ConfirmedRemote() changed to %q", got)
	}
	if got := manager.CandidateRemote(); got != "1.1.1.1:5002" {
		t.Fatalf("CandidateRemote() should stay on confirmed remote, got %q", got)
	}
}

func TestCandidateManagerSingleNomination(t *testing.T) {
	manager := NewCandidateManager("1.1.1.1:5000")
	manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	manager.Observe("0.0.0.0:3001", "1.1.1.1:5010")
	manager.MarkSucceeded("0.0.0.0:3000", "1.1.1.1:5002")
	manager.MarkSucceeded("0.0.0.0:3001", "1.1.1.1:5010")
	if _, ok := manager.TryNominate("0.0.0.0:3000", "1.1.1.1:5002"); !ok {
		t.Fatal("first nomination should succeed")
	}
	if _, ok := manager.TryNominate("0.0.0.0:3001", "1.1.1.1:5010"); ok {
		t.Fatal("second nomination should be blocked")
	}
	pair := manager.NominatedPair()
	if pair == nil || pair.LocalAddr != "0.0.0.0:3000" || pair.RemoteAddr != "1.1.1.1:5002" {
		t.Fatalf("unexpected nominated pair %#v", pair)
	}
}

func TestCandidateManagerRequiresNominationBeforeConfirm(t *testing.T) {
	manager := NewCandidateManager("1.1.1.1:5000")
	manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	manager.MarkSucceeded("0.0.0.0:3000", "1.1.1.1:5002")
	if pair := manager.Confirm("0.0.0.0:3000", "1.1.1.1:5002"); pair != nil {
		t.Fatalf("confirm should fail without nomination, got %#v", pair)
	}
	if manager.ConfirmedPair() != nil {
		t.Fatal("confirmed pair should stay empty without nomination")
	}
}

func TestCandidateManagerRejectsCrossPairConfirm(t *testing.T) {
	manager := NewCandidateManager("1.1.1.1:5000")
	manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	manager.Observe("0.0.0.0:3001", "1.1.1.1:5010")
	manager.MarkSucceeded("0.0.0.0:3000", "1.1.1.1:5002")
	manager.MarkSucceeded("0.0.0.0:3001", "1.1.1.1:5010")
	if _, ok := manager.TryNominate("0.0.0.0:3000", "1.1.1.1:5002"); !ok {
		t.Fatal("nomination should succeed for first pair")
	}
	if pair := manager.Confirm("0.0.0.0:3001", "1.1.1.1:5010"); pair != nil {
		t.Fatalf("cross-pair confirm should be rejected, got %#v", pair)
	}
	if pair := manager.ConfirmedPair(); pair != nil {
		t.Fatalf("unexpected confirmed pair %#v", pair)
	}
	if pair := manager.Confirm("0.0.0.0:3000", "1.1.1.1:5002"); pair == nil {
		t.Fatal("nominated pair should confirm successfully")
	}
}

func TestCandidateManagerClosesOtherPairsAfterConfirm(t *testing.T) {
	manager := NewCandidateManager("1.1.1.1:5000")
	manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	manager.Observe("0.0.0.0:3001", "1.1.1.1:5010")
	manager.MarkSucceeded("0.0.0.0:3000", "1.1.1.1:5002")
	manager.MarkSucceeded("0.0.0.0:3001", "1.1.1.1:5010")
	manager.TryNominate("0.0.0.0:3000", "1.1.1.1:5002")
	if pair := manager.Confirm("0.0.0.0:3000", "1.1.1.1:5002"); pair == nil {
		t.Fatal("expected nominated pair to confirm")
	}
	if state := manager.candidates[candidateKey("0.0.0.0:3001", "1.1.1.1:5010")].State; state != CandidateClosed {
		t.Fatalf("other candidate state = %s, want %s", state, CandidateClosed)
	}
	manager.MarkSucceeded("0.0.0.0:3001", "1.1.1.1:5010")
	if state := manager.candidates[candidateKey("0.0.0.0:3001", "1.1.1.1:5010")].State; state != CandidateClosed {
		t.Fatalf("closed candidate should stay closed, got %s", state)
	}
}

func TestSplitBrainNominationConvergesToSinglePair(t *testing.T) {
	visitor := NewCandidateManager("1.1.1.1:5000")
	provider := NewCandidateManager("2.2.2.2:6000")

	visitor.Observe("v-local-1", "p-remote-1")
	visitor.Observe("v-local-2", "p-remote-2")
	visitor.MarkSucceeded("v-local-1", "p-remote-1")
	visitor.MarkSucceeded("v-local-2", "p-remote-2")

	provider.Observe("p-local-1", "v-remote-1")
	provider.Observe("p-local-2", "v-remote-2")
	provider.MarkSucceeded("p-local-1", "v-remote-1")
	provider.MarkSucceeded("p-local-2", "v-remote-2")

	if _, ok := visitor.TryNominate("v-local-2", "p-remote-2"); !ok {
		t.Fatal("visitor should nominate one winning pair")
	}
	if _, ok := provider.TryNominate("p-local-2", "v-remote-2"); !ok {
		t.Fatal("provider should accept the same nominated pair once END arrives")
	}
	if provider.Confirm("p-local-2", "v-remote-2") == nil {
		t.Fatal("provider should confirm nominated pair")
	}
	if visitor.Confirm("v-local-2", "p-remote-2") == nil {
		t.Fatal("visitor should confirm nominated pair after ACCEPT")
	}
	if pair := visitor.ConfirmedPair(); pair == nil || pair.LocalAddr != "v-local-2" || pair.RemoteAddr != "p-remote-2" {
		t.Fatalf("unexpected visitor confirmed pair %#v", pair)
	}
	if pair := visitor.candidates[candidateKey("v-local-1", "p-remote-1")]; pair == nil || pair.State != CandidateClosed {
		t.Fatalf("losing visitor pair should close, got %#v", pair)
	}
	if pair := provider.candidates[candidateKey("p-local-1", "v-remote-1")]; pair == nil || pair.State != CandidateClosed {
		t.Fatalf("losing provider pair should close, got %#v", pair)
	}
}

func TestRuntimeSessionKeepsConfirmedRemoteStable(t *testing.T) {
	session := &runtimeSession{}
	session.updateCandidateRemote("1.1.1.1:5002")
	session.updateConfirmedRemote("1.1.1.1:5002")
	session.updateCandidateRemote("1.1.1.1:5010")
	if session.endpoints.CandidateRemote != "1.1.1.1:5002" {
		t.Fatalf("CandidateRemote changed to %q", session.endpoints.CandidateRemote)
	}
	if session.endpoints.ConfirmedRemote != "1.1.1.1:5002" {
		t.Fatalf("ConfirmedRemote changed to %q", session.endpoints.ConfirmedRemote)
	}
}

func TestRuntimeSessionDropsWrongTokenSilently(t *testing.T) {
	readConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(read) error = %v", err)
	}
	defer func() { _ = readConn.Close() }()
	sendConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(send) error = %v", err)
	}
	defer func() { _ = sendConn.Close() }()

	session := &runtimeSession{
		start: P2PPunchStart{
			SessionID: "session-1",
			Token:     "token-1",
			Role:      common.WORK_P2P_VISITOR,
		},
		cm:        NewCandidateManager(""),
		confirmed: make(chan confirmedPair, 1),
		nominate:  make(map[string]struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.readLoopOnConn(ctx, readConn)

	wrong := newUDPPacket("session-1", "wrong-token", common.WORK_P2P_PROVIDER, packetTypeSucc)
	wrongRaw, err := EncodeUDPPacket(wrong)
	if err != nil {
		t.Fatalf("EncodeUDPPacket(wrong) error = %v", err)
	}
	if _, err := sendConn.WriteTo(wrongRaw, readConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo(wrong) error = %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	if session.cm.NominatedPair() != nil || session.cm.ConfirmedPair() != nil || session.endpoints.CandidateRemote != "" {
		t.Fatal("wrong-token packet should be silently dropped")
	}
	if session.stats.tokenMismatchDropped == 0 {
		t.Fatal("wrong-token packet should increase mismatch counter")
	}

	right := newUDPPacket("session-1", "token-1", common.WORK_P2P_PROVIDER, packetTypeSucc)
	rightRaw, err := EncodeUDPPacket(right)
	if err != nil {
		t.Fatalf("EncodeUDPPacket(right) error = %v", err)
	}
	if _, err := sendConn.WriteTo(rightRaw, readConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo(right) error = %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	pair := session.cm.NominatedPair()
	if pair == nil {
		t.Fatal("valid packet should still progress handshake after wrong-token packet")
	}
	if pair.RemoteAddr != sendConn.LocalAddr().String() {
		t.Fatalf("nominated remote = %q, want %q", pair.RemoteAddr, sendConn.LocalAddr().String())
	}
	if !session.stats.tokenVerified {
		t.Fatal("valid packet should mark token verified")
	}
}

func TestRuntimeSessionDropsReplaySilently(t *testing.T) {
	readConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(read) error = %v", err)
	}
	defer func() { _ = readConn.Close() }()
	sendConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(send) error = %v", err)
	}
	defer func() { _ = sendConn.Close() }()

	session := &runtimeSession{
		start: P2PPunchStart{
			SessionID: "session-1",
			Token:     "token-1",
			Role:      common.WORK_P2P_PROVIDER,
		},
		cm:        NewCandidateManager(""),
		replay:    NewReplayWindow(30 * time.Second),
		confirmed: make(chan confirmedPair, 1),
		nominate:  make(map[string]struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.readLoopOnConn(ctx, readConn)

	packet := newUDPPacket("session-1", "token-1", common.WORK_P2P_VISITOR, packetTypeProbe)
	raw, err := EncodeUDPPacket(packet)
	if err != nil {
		t.Fatalf("EncodeUDPPacket() error = %v", err)
	}
	if _, err := sendConn.WriteTo(raw, readConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo(first) error = %v", err)
	}
	if _, err := sendConn.WriteTo(raw, readConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo(replay) error = %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	if session.stats.replayDropped == 0 {
		t.Fatal("replayed packet should increase replay counter")
	}
}

func TestCandidateManagerPruneAndCleanup(t *testing.T) {
	manager := NewCandidateManager("1.1.1.1:5000")
	pair := manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	pair.LastSeenAt = time.Now().Add(-10 * time.Second)
	if pruned := manager.PruneStale(2 * time.Second); pruned != 1 {
		t.Fatalf("PruneStale() = %d, want 1", pruned)
	}
	if pair.State != CandidateClosed {
		t.Fatalf("pair state = %s, want %s", pair.State, CandidateClosed)
	}
	pair.LastSeenAt = time.Now().Add(-10 * time.Second)
	if removed := manager.CleanupClosed(2 * time.Second); removed != 1 {
		t.Fatalf("CleanupClosed() = %d, want 1", removed)
	}
	if len(manager.candidates) != 0 {
		t.Fatalf("cleanup should remove closed candidate, still have %#v", manager.candidates)
	}
}

func TestCandidateManagerCanReopenPrunedCandidateBeforeConfirm(t *testing.T) {
	manager := NewCandidateManager("1.1.1.1:5000")
	pair := manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	pair.LastSeenAt = time.Now().Add(-10 * time.Second)
	manager.PruneStale(2 * time.Second)
	reopened := manager.Observe("0.0.0.0:3000", "1.1.1.1:5002")
	if reopened.State != CandidateDiscovered {
		t.Fatalf("reopened candidate state = %s, want %s", reopened.State, CandidateDiscovered)
	}
	manager.MarkSucceeded("0.0.0.0:3000", "1.1.1.1:5002")
	if reopened.State != CandidateSucceeded {
		t.Fatalf("reopened candidate should succeed again, got %s", reopened.State)
	}
}

func TestRuntimeSessionDoesNotHandoverBeforeConfirm(t *testing.T) {
	readConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(read) error = %v", err)
	}
	defer func() { _ = readConn.Close() }()
	sendConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(send) error = %v", err)
	}
	defer func() { _ = sendConn.Close() }()

	session := &runtimeSession{
		start: P2PPunchStart{
			SessionID: "session-1",
			Token:     "token-1",
			Role:      common.WORK_P2P_VISITOR,
		},
		cm:        NewCandidateManager(""),
		confirmed: make(chan confirmedPair, 1),
		nominate:  make(map[string]struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go session.readLoopOnConn(ctx, readConn)

	success := newUDPPacket("session-1", "token-1", common.WORK_P2P_PROVIDER, packetTypeSucc)
	raw, err := EncodeUDPPacket(success)
	if err != nil {
		t.Fatalf("EncodeUDPPacket(success) error = %v", err)
	}
	if _, err := sendConn.WriteTo(raw, readConn.LocalAddr()); err != nil {
		t.Fatalf("WriteTo(success) error = %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	select {
	case pair := <-session.confirmed:
		t.Fatalf("session should not handover before confirm, got %#v", pair)
	default:
	}
	if session.cm.ConfirmedPair() != nil {
		t.Fatal("confirmed pair should stay empty before ACCEPT")
	}
}

func TestCrosstalkPacketDoesNotAffectOtherSession(t *testing.T) {
	sessionA := &runtimeSession{
		start:     P2PPunchStart{SessionID: "session-a", Token: "token-a", Role: common.WORK_P2P_VISITOR},
		cm:        NewCandidateManager(""),
		confirmed: make(chan confirmedPair, 1),
		nominate:  make(map[string]struct{}),
	}
	sessionB := &runtimeSession{
		start:     P2PPunchStart{SessionID: "session-b", Token: "token-b", Role: common.WORK_P2P_VISITOR},
		cm:        NewCandidateManager(""),
		confirmed: make(chan confirmedPair, 1),
		nominate:  make(map[string]struct{}),
	}

	readA, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(readA) error = %v", err)
	}
	defer func() { _ = readA.Close() }()
	readB, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(readB) error = %v", err)
	}
	defer func() { _ = readB.Close() }()
	sendConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(send) error = %v", err)
	}
	defer func() { _ = sendConn.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sessionA.readLoopOnConn(ctx, readA)
	go sessionB.readLoopOnConn(ctx, readB)

	packetA := newUDPPacket("session-a", "token-a", common.WORK_P2P_PROVIDER, packetTypeSucc)
	rawA, err := EncodeUDPPacket(packetA)
	if err != nil {
		t.Fatalf("EncodeUDPPacket(packetA) error = %v", err)
	}
	if _, err := sendConn.WriteTo(rawA, readA.LocalAddr()); err != nil {
		t.Fatalf("WriteTo(readA) error = %v", err)
	}
	if _, err := sendConn.WriteTo(rawA, readB.LocalAddr()); err != nil {
		t.Fatalf("WriteTo(readB) error = %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	if sessionA.cm.NominatedPair() == nil {
		t.Fatal("session A should accept its own packet")
	}
	if sessionB.cm.NominatedPair() != nil || sessionB.cm.ConfirmedPair() != nil || sessionB.endpoints.CandidateRemote != "" {
		t.Fatal("session B should ignore crosstalk packet from session A")
	}
	if sessionB.stats.tokenMismatchDropped == 0 {
		t.Fatal("session B should count mismatched crosstalk packet")
	}
}

func TestDefaultTimeouts(t *testing.T) {
	timeouts := DefaultTimeouts()
	if timeouts.HandshakeTimeoutMs != 20000 {
		t.Fatalf("HandshakeTimeoutMs = %d, want 20000", timeouts.HandshakeTimeoutMs)
	}
	if timeouts.TransportTimeoutMs != 10000 {
		t.Fatalf("TransportTimeoutMs = %d, want 10000", timeouts.TransportTimeoutMs)
	}
}

func TestMapP2PContextError(t *testing.T) {
	if !errors.Is(mapP2PContextError(context.DeadlineExceeded), ErrNATNotSupportP2P) {
		t.Fatal("deadline should map to ErrNATNotSupportP2P")
	}
	plainErr := errors.New("plain")
	if !errors.Is(mapP2PContextError(plainErr), plainErr) {
		t.Fatal("plain error should be returned directly")
	}
}

func TestEffectiveExtraReplySeen(t *testing.T) {
	if !effectiveExtraReplySeen(false, false) {
		t.Fatal("disabled extra reply should not force restricted inference")
	}
	if effectiveExtraReplySeen(false, true) {
		t.Fatal("missing expected extra reply should stay false")
	}
	if !effectiveExtraReplySeen(true, true) {
		t.Fatal("observed extra reply should stay true")
	}
}

func TestIsIgnorableUDPIcmpError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "windows 10054", err: errors.New("wsarecvfrom: 10054"), want: true},
		{name: "connection refused", err: errors.New("read udp: connection refused"), want: true},
		{name: "connection reset by peer", err: errors.New("connection reset by peer"), want: true},
		{name: "other", err: errors.New("use of closed network connection"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIgnorableUDPIcmpError(tt.err); got != tt.want {
				t.Fatalf("isIgnorableUDPIcmpError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestProbeEndpointAddrs(t *testing.T) {
	addrs := probeEndpointAddrs(P2PProbeConfig{
		Version:  1,
		Provider: ProbeProviderNPS,
		Mode:     ProbeModeUDP,
		Network:  ProbeNetworkUDP,
		Endpoints: []P2PProbeEndpoint{
			{ID: "probe-1", Network: ProbeNetworkUDP, Address: "1.1.1.1:10206"},
			{ID: "probe-2", Network: ProbeNetworkUDP, Address: "bad"},
			{ID: "probe-3", Network: ProbeNetworkUDP, Address: "1.1.1.1:10208"},
		},
	})
	if len(addrs) != 2 {
		t.Fatalf("len(addrs) = %d, want 2 (%#v)", len(addrs), addrs)
	}
	if addrs[0] != "1.1.1.1:10206" || addrs[1] != "1.1.1.1:10208" {
		t.Fatalf("unexpected addrs %#v", addrs)
	}
}

func TestDecodeUDPPacketWithLookup(t *testing.T) {
	packet := newUDPPacket("session-lookup", "token-lookup", common.WORK_P2P_VISITOR, packetTypeProbe)
	raw, err := EncodeUDPPacket(packet)
	if err != nil {
		t.Fatalf("EncodeUDPPacket() error = %v", err)
	}
	decoded, err := DecodeUDPPacketWithLookup(raw, func(sessionID string) (string, bool) {
		if sessionID != "session-lookup" {
			return "", false
		}
		return "token-lookup", true
	})
	if err != nil {
		t.Fatalf("DecodeUDPPacketWithLookup() error = %v", err)
	}
	if decoded.SessionID != "session-lookup" || decoded.Token != "token-lookup" {
		t.Fatalf("decoded packet mismatch: %#v", decoded)
	}
	if _, err := DecodeUDPPacketWithLookup(raw, func(sessionID string) (string, bool) {
		return "", false
	}); !errors.Is(err, ErrP2PTokenMismatch) {
		t.Fatalf("expected ErrP2PTokenMismatch for missing lookup, got %v", err)
	}
}

func TestReplayWindowRejectsExpiredFutureAndDuplicatePackets(t *testing.T) {
	window := NewReplayWindow(100 * time.Millisecond)
	now := time.Now().UnixMilli()
	if window.Accept(now-500, "expired") {
		t.Fatal("expired packet should be rejected")
	}
	if window.Accept(now+500, "future") {
		t.Fatal("future packet should be rejected")
	}
	if !window.Accept(now, "ok") {
		t.Fatal("fresh packet should be accepted")
	}
	if window.Accept(now, "ok") {
		t.Fatal("duplicate nonce should be rejected")
	}
}

func TestCompatibleSTUNDerivedEndpoint(t *testing.T) {
	localV4, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket(v4) error = %v", err)
	}
	defer func() { _ = localV4.Close() }()

	if !compatibleSTUNDerivedEndpoint(localV4.LocalAddr(), "127.0.0.1:3479", "127.0.0.1:3478") {
		t.Fatal("same-family different endpoint should be accepted")
	}
	if compatibleSTUNDerivedEndpoint(localV4.LocalAddr(), "[::1]:3479", "") {
		t.Fatal("different-family endpoint should be rejected")
	}
	if compatibleSTUNDerivedEndpoint(localV4.LocalAddr(), "127.0.0.1:3478", "127.0.0.1:3478") {
		t.Fatal("response-origin duplicate endpoint should be rejected")
	}
}

func TestBuildHistoricalPredictionPortsOrdersByFrequency(t *testing.T) {
	resetPredictionHistoryForTest()
	t.Cleanup(resetPredictionHistoryForTest)

	obs := NatObservation{
		PublicIP:         "1.1.1.1",
		ObservedBasePort: 5000,
		ObservedInterval: 2,
		NATType:          NATTypeSymmetric,
		MappingBehavior:  NATMappingEndpointDependent,
	}
	recordPredictionSuccess(obs, "1.1.1.1:5006")
	recordPredictionSuccess(obs, "1.1.1.1:5006")
	recordPredictionSuccess(obs, "1.1.1.1:4998")
	recordPredictionSuccess(obs, "1.1.1.1:5004")

	ports := buildHistoricalPredictionPorts(obs, 4)
	if len(ports) < 3 {
		t.Fatalf("expected historical prediction ports, got %#v", ports)
	}
	if ports[0] != 5006 {
		t.Fatalf("ports[0] = %d, want 5006", ports[0])
	}
}

func TestBuildPCPRequestMappingPacket(t *testing.T) {
	clientIP := netip.MustParseAddr("192.168.1.10")
	prevExternalIP := netip.MustParseAddr("203.0.113.20")
	packet, nonce := buildPCPRequestMappingPacket(clientIP, 12345, 54321, 7200, prevExternalIP)
	if len(packet) != 60 {
		t.Fatalf("len(packet) = %d, want 60", len(packet))
	}
	if packet[0] != pcpVersion {
		t.Fatalf("packet version = %d, want %d", packet[0], pcpVersion)
	}
	if packet[1] != pcpOpMap {
		t.Fatalf("packet opcode = %d, want %d", packet[1], pcpOpMap)
	}
	if got := binary.BigEndian.Uint32(packet[4:8]); got != 7200 {
		t.Fatalf("lifetime = %d, want 7200", got)
	}
	var clientIP16 [16]byte
	copy(clientIP16[:], packet[8:24])
	if got := netip.AddrFrom16(clientIP16).Unmap(); got != clientIP {
		t.Fatalf("client ip = %s, want %s", got, clientIP)
	}
	if packet[36] != pcpUDPMapping {
		t.Fatalf("protocol = %d, want %d", packet[36], pcpUDPMapping)
	}
	if string(packet[24:36]) != string(nonce[:]) {
		t.Fatal("request nonce should be embedded in packet")
	}
	if got := binary.BigEndian.Uint16(packet[40:42]); got != 12345 {
		t.Fatalf("local port = %d, want 12345", got)
	}
	if got := binary.BigEndian.Uint16(packet[42:44]); got != 54321 {
		t.Fatalf("previous external port = %d, want 54321", got)
	}
	var externalIP16 [16]byte
	copy(externalIP16[:], packet[44:60])
	if got := netip.AddrFrom16(externalIP16).Unmap(); got != prevExternalIP {
		t.Fatalf("previous external ip = %s, want %s", got, prevExternalIP)
	}
}

func TestParsePCPMapResponse(t *testing.T) {
	response := make([]byte, 60)
	response[0] = pcpVersion
	response[1] = pcpOpMap | pcpOpReply
	response[3] = byte(pcpCodeOK)
	binary.BigEndian.PutUint32(response[4:8], 3600)
	binary.BigEndian.PutUint32(response[8:12], 42)
	var nonce [12]byte
	copy(nonce[:], []byte("pcpnonce1234"))
	copy(response[24:36], nonce[:])
	binary.BigEndian.PutUint16(response[42:44], 54321)
	externalIP := netip.MustParseAddr("198.51.100.25").As16()
	copy(response[44:60], externalIP[:])

	parsed, err := parsePCPMapResponse(response, nonce)
	if err != nil {
		t.Fatalf("parsePCPMapResponse() error = %v", err)
	}
	if parsed.ResultCode != pcpCodeOK {
		t.Fatalf("ResultCode = %s, want %s", parsed.ResultCode, pcpCodeOK)
	}
	if parsed.LifetimeSecs != 3600 {
		t.Fatalf("LifetimeSecs = %d, want 3600", parsed.LifetimeSecs)
	}
	if parsed.Epoch != 42 {
		t.Fatalf("Epoch = %d, want 42", parsed.Epoch)
	}
	if parsed.ExternalAddr != netip.MustParseAddrPort("198.51.100.25:54321") {
		t.Fatalf("ExternalAddr = %s, want 198.51.100.25:54321", parsed.ExternalAddr)
	}
}

func TestParsePCPMapResponseRejectsNonOKCodes(t *testing.T) {
	response := make([]byte, 60)
	response[0] = pcpVersion
	response[1] = pcpOpMap | pcpOpReply
	response[3] = byte(pcpCodeNotAuthorized)
	if _, err := parsePCPMapResponse(response, [12]byte{}); err == nil {
		t.Fatal("parsePCPMapResponse() should reject non-OK response codes")
	}
}

func TestParsePCPMapResponseRejectsNonceMismatch(t *testing.T) {
	response := make([]byte, 60)
	response[0] = pcpVersion
	response[1] = pcpOpMap | pcpOpReply
	response[3] = byte(pcpCodeOK)
	copy(response[24:36], []byte("pcpnonce1234"))
	binary.BigEndian.PutUint16(response[42:44], 54321)
	externalIP := netip.MustParseAddr("198.51.100.25").As16()
	copy(response[44:60], externalIP[:])

	if _, err := parsePCPMapResponse(response, [12]byte{'w', 'r', 'o', 'n', 'g'}); err == nil {
		t.Fatal("parsePCPMapResponse() should reject nonce mismatch")
	}
}

func buildFakeSTUNBindingResponse(request []byte, addr *net.UDPAddr) ([]byte, error) {
	return buildFakeSTUNBindingResponseWithAttrs(request, addr, nil, nil)
}

func buildFakeSTUNBindingResponseWithAttrs(request []byte, addr, responseOrigin, otherAddr *net.UDPAddr) ([]byte, error) {
	message := new(pionstun.Message)
	if err := message.UnmarshalBinary(request); err != nil {
		return nil, err
	}
	setters := []pionstun.Setter{
		pionstun.NewTransactionIDSetter(message.TransactionID),
		pionstun.BindingSuccess,
		&pionstun.XORMappedAddress{
			IP:   addr.IP,
			Port: addr.Port,
		},
	}
	if responseOrigin != nil {
		setters = append(setters, &pionstun.ResponseOrigin{
			IP:   responseOrigin.IP,
			Port: responseOrigin.Port,
		})
	}
	if otherAddr != nil {
		setters = append(setters, &pionstun.OtherAddress{
			IP:   otherAddr.IP,
			Port: otherAddr.Port,
		})
	}
	setters = append(setters, pionstun.Fingerprint)
	response, err := pionstun.Build(setters...)
	if err != nil {
		return nil, err
	}
	return response.Raw, nil
}
