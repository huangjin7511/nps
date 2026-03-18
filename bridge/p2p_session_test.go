package bridge

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/p2p"
	"github.com/djylb/nps/server/connection"
	"github.com/djylb/nps/server/p2pstate"
)

func TestP2PBridgeSessionAbortStillRelaysAfterSummary(t *testing.T) {
	visitorServer, visitorClient := net.Pipe()
	providerServer, providerClient := net.Pipe()
	defer func() { _ = visitorServer.Close() }()
	defer func() { _ = visitorClient.Close() }()
	defer func() { _ = providerServer.Close() }()
	defer func() { _ = providerClient.Close() }()

	session := &p2pBridgeSession{
		mgr:             newP2PSessionManager(),
		id:              "session-1",
		token:           "token-1",
		timeouts:        p2p.P2PTimeouts{ProbeTimeoutMs: 1000, HandshakeTimeoutMs: 1000, TransportTimeoutMs: 1000},
		visitorControl:  conn.NewConn(visitorServer),
		providerControl: conn.NewConn(providerServer),
		visitorReport: &p2p.P2PProbeReport{
			SessionID: "session-1",
			Token:     "token-1",
			Role:      common.WORK_P2P_VISITOR,
			PeerRole:  common.WORK_P2P_PROVIDER,
			Self:      p2p.P2PPeerInfo{Role: common.WORK_P2P_VISITOR},
		},
		providerReport: &p2p.P2PProbeReport{
			SessionID: "session-1",
			Token:     "token-1",
			Role:      common.WORK_P2P_PROVIDER,
			PeerRole:  common.WORK_P2P_VISITOR,
			Self:      p2p.P2PPeerInfo{Role: common.WORK_P2P_PROVIDER},
		},
		timer: time.NewTimer(time.Minute),
	}
	session.mgr.sessions.Store(session.id, session)
	p2pstate.Register(session.id, session.token, time.Minute)
	defer p2pstate.Unregister(session.id)

	type result struct {
		flag  string
		abort p2p.P2PPunchAbort
		err   error
	}
	readSide := func(c net.Conn) <-chan result {
		out := make(chan result, 1)
		go func() {
			defer close(out)
			cc := conn.NewConn(c)
			if _, err := p2p.ReadBridgeJSON[p2p.P2PProbeSummary](cc, common.P2P_PROBE_SUMMARY); err != nil {
				out <- result{err: err}
				return
			}
			flag, err := cc.ReadFlag()
			if err != nil {
				out <- result{err: err}
				return
			}
			raw, err := cc.GetShortLenContent()
			if err != nil {
				out <- result{err: err}
				return
			}
			var abort p2p.P2PPunchAbort
			if err := json.Unmarshal(raw, &abort); err != nil {
				out <- result{err: err}
				return
			}
			out <- result{flag: flag, abort: abort}
		}()
		return out
	}

	visitorDone := readSide(visitorClient)
	providerDone := readSide(providerClient)

	session.sendSummary()
	session.abort("late failure")

	visitorResult := <-visitorDone
	providerResult := <-providerDone
	if visitorResult.err != nil {
		t.Fatalf("visitor side read failed: %v", visitorResult.err)
	}
	if providerResult.err != nil {
		t.Fatalf("provider side read failed: %v", providerResult.err)
	}
	if visitorResult.flag != common.P2P_PUNCH_ABORT || providerResult.flag != common.P2P_PUNCH_ABORT {
		t.Fatalf("unexpected abort flags visitor=%q provider=%q", visitorResult.flag, providerResult.flag)
	}
	if visitorResult.abort.Reason != "late failure" || providerResult.abort.Reason != "late failure" {
		t.Fatalf("unexpected abort payload visitor=%#v provider=%#v", visitorResult.abort, providerResult.abort)
	}
}

func TestCollectProbeBaseHostsIncludesBridgeAndConfiguredFamilies(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() { _ = listener.Close() }()

	serverConnCh := make(chan net.Conn, 1)
	go func() {
		serverConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		serverConnCh <- serverConn
	}()

	clientConn, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer func() { _ = clientConn.Close() }()
	serverConn := <-serverConnCh
	defer func() { _ = serverConn.Close() }()

	oldP2pIP := connection.P2pIp
	connection.P2pIp = "::1"
	defer func() { connection.P2pIp = oldP2pIP }()

	hosts := collectProbeBaseHosts(conn.NewConn(serverConn))
	if len(hosts) < 2 {
		t.Fatalf("expected dual-family probe hosts, got %#v", hosts)
	}
	if hosts[0] != "127.0.0.1" {
		t.Fatalf("hosts[0] = %q, want bridge local IPv4", hosts[0])
	}
	foundIPv6 := false
	for _, host := range hosts {
		if host == "::1" {
			foundIPv6 = true
			break
		}
	}
	if !foundIPv6 {
		t.Fatalf("expected configured IPv6 probe host in %#v", hosts)
	}
}

func TestBuildProbeOptionsIncludesPCPFlag(t *testing.T) {
	options := buildProbeOptions()
	value, ok := options["enable_pcp_portmap"]
	if !ok {
		t.Fatal("enable_pcp_portmap should be present in bridge probe options")
	}
	if value != "true" && value != "false" {
		t.Fatalf("unexpected enable_pcp_portmap value %q", value)
	}
}
