package p2p

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/logs"
)

type SessionEndpoints struct {
	RendezvousRemote string
	CandidateRemote  string
	ConfirmedRemote  string
}

type confirmedPair struct {
	conn       net.PacketConn
	localAddr  string
	remoteAddr string
}

type runtimeSession struct {
	start     P2PPunchStart
	summary   P2PProbeSummary
	plan      PunchPlan
	control   *conn.Conn
	localConn net.PacketConn
	sockets   []net.PacketConn
	cm        *CandidateManager
	replay    *ReplayWindow

	mu        sync.Mutex
	endpoints SessionEndpoints
	confirmed chan confirmedPair
	nominate  map[string]struct{}
	stats     sessionStats
}

type sessionStats struct {
	tokenMismatchDropped int
	tokenVerified        bool
	replayDropped        int
}

func RunVisitorSession(ctx context.Context, control *conn.Conn, localAddr, transportMode, transportData string, defaultSTUNServers []string) (net.PacketConn, string, string, string, string, string, string, time.Duration, error) {
	start, err := ReadBridgeJSON[P2PPunchStart](control, common.P2P_PUNCH_START)
	if err != nil {
		return nil, "", "", "", "", "", "", 0, err
	}
	return runBridgeSession(ctx, control, start, localAddr, transportMode, transportData, defaultSTUNServers)
}

func RunProviderSession(ctx context.Context, control *conn.Conn, start P2PPunchStart, localAddr, transportMode, transportData string, defaultSTUNServers []string) (net.PacketConn, string, string, string, string, string, string, time.Duration, error) {
	return runBridgeSession(ctx, control, start, localAddr, transportMode, transportData, defaultSTUNServers)
}

func runBridgeSession(ctx context.Context, control *conn.Conn, start P2PPunchStart, localAddr, transportMode, transportData string, defaultSTUNServers []string) (net.PacketConn, string, string, string, string, string, string, time.Duration, error) {
	start.Probe = WithDefaultSTUNEndpoints(start.Probe, defaultSTUNServers)
	localConn, err := conn.NewUdpConnByAddr(localAddr)
	if err != nil {
		return nil, "", "", "", "", "", "", 0, err
	}
	defer func() {
		if err != nil && localConn != nil {
			_ = localConn.Close()
		}
	}()
	var portMapping *PortMappingInfo
	localConn, portMapping = maybeEnablePortMapping(ctx, localConn, start.Probe)

	_ = trySendProgress(control, start.SessionID, start.Role, "probe_started", "")
	observation, err := runProbe(ctx, localConn, start)
	if err != nil {
		_ = trySendProgressWithStatus(control, start.SessionID, start.Role, "probe_failed", "fail", err.Error(), addrString(localConn.LocalAddr()), "", nil, nil)
		_ = trySendAbort(control, start.SessionID, start.Role, fmt.Sprintf("probe failed: %v", err))
		return nil, "", "", "", "", "", "", 0, err
	}
	if portMapping != nil {
		observation.PortMapping = portMapping
	}

	self := P2PPeerInfo{
		Role:          start.Role,
		Nat:           observation,
		LocalAddrs:    buildLocalAddrs(localConn.LocalAddr()),
		TransportMode: transportMode,
		TransportData: transportData,
	}
	report := P2PProbeReport{
		SessionID: start.SessionID,
		Token:     start.Token,
		Role:      start.Role,
		PeerRole:  start.PeerRole,
		Self:      self,
	}
	_ = trySendProgress(control, start.SessionID, start.Role, "probe_completed", observation.NATType)
	if err = WriteBridgeMessage(control, common.P2P_PROBE_REPORT, report); err != nil {
		return nil, "", "", "", "", "", "", 0, err
	}
	_ = trySendProgress(control, start.SessionID, start.Role, "probe_reported", "")

	summary, err := ReadBridgeJSON[P2PProbeSummary](control, common.P2P_PROBE_SUMMARY)
	if err != nil {
		return nil, "", "", "", "", "", "", 0, err
	}
	_ = trySendProgress(control, start.SessionID, start.Role, "summary_received", summary.Peer.Nat.NATType)
	plan := ApplyProbePlanOptions(SelectPunchPlan(summary.Self, summary.Peer, summary.Timeouts), start.Probe, summary.Self, summary.Peer)
	logs.Info("[P2P] selected PunchPlan role=%s mappingConfidenceLow=%v probePortRestricted=%v prediction=%v targetSpray=%v birthday=%v birthdayFallback=%v intervals=%v timeout=%s",
		start.Role, summary.Self.Nat.MappingConfidenceLow, summary.Self.Nat.ProbePortRestricted,
		plan.EnablePrediction, plan.UseTargetSpray, plan.UseBirthdayAttack, plan.AllowBirthdayFallback, plan.predictionIntervals(), plan.HandshakeTimeout)

	rs := &runtimeSession{
		start:     start,
		summary:   summary,
		plan:      plan,
		control:   control,
		localConn: localConn,
		sockets:   []net.PacketConn{localConn},
		cm:        NewCandidateManager(firstObservedAddr(summary.Peer)),
		replay:    NewReplayWindow(30 * time.Second),
		confirmed: make(chan confirmedPair, 1),
		nominate:  make(map[string]struct{}),
	}
	rs.endpoints.RendezvousRemote = firstObservedAddr(summary.Peer)
	defer func() {
		if err != nil {
			rs.closeSockets(nil)
		}
	}()

	punchCtx, cancel := context.WithTimeout(ctx, plan.HandshakeTimeout)
	defer cancel()

	go rs.readLoopOnConn(punchCtx, localConn)
	go rs.candidateMaintenanceLoop(punchCtx)
	go rs.startSpray(punchCtx)
	transportTimeout := time.Duration(summary.Timeouts.TransportTimeoutMs) * time.Millisecond
	if transportTimeout <= 0 {
		transportTimeout = 10 * time.Second
	}

	select {
	case pair := <-rs.confirmed:
		mode := negotiateTransportMode(transportMode, summary.Peer.TransportMode)
		rs.closeSockets(pair.conn)
		_ = rs.sendProgress("candidate_confirmed", "ok", pair.remoteAddr, pair.localAddr, pair.remoteAddr, nil)
		_ = rs.sendProgress("handover_begin", "ok", mode, pair.localAddr, pair.remoteAddr, map[string]string{
			"transport_mode": mode,
		})
		recordPredictionSuccess(summary.Peer.Nat, pair.remoteAddr)
		rs.logTokenStats()
		logs.Info("[P2P] handover begin role=%s confirmed pair=%s local=%s transport=%s", start.Role, pair.remoteAddr, pair.localAddr, mode)
		return pair.conn, pair.remoteAddr, pair.localAddr, start.SessionID, start.Role, mode, summary.Peer.TransportData, transportTimeout, nil
	case <-punchCtx.Done():
		err = mapP2PContextError(punchCtx.Err())
		_ = rs.sendProgress("handshake_timeout", "fail", err.Error(), addrString(localConn.LocalAddr()), rs.cm.CandidateRemote(), nil)
		rs.logTokenStats()
		_ = trySendAbort(control, start.SessionID, start.Role, err.Error())
		return nil, "", "", "", "", "", "", 0, err
	}
}

func runProbe(ctx context.Context, localConn net.PacketConn, start P2PPunchStart) (NatObservation, error) {
	endpoints := NormalizeProbeEndpoints(start.Probe)
	if len(endpoints) == 0 {
		return NatObservation{}, fmt.Errorf("empty probe endpoints")
	}
	stunEndpoints := make([]P2PProbeEndpoint, 0, len(endpoints))
	npsEndpoints := make([]P2PProbeEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		switch {
		case endpoint.Provider == ProbeProviderSTUN && endpoint.Mode == ProbeModeBinding && endpoint.Network == ProbeNetworkUDP:
			stunEndpoints = append(stunEndpoints, endpoint)
		case endpoint.Provider == ProbeProviderNPS && endpoint.Mode == ProbeModeUDP && endpoint.Network == ProbeNetworkUDP:
			npsEndpoints = append(npsEndpoints, endpoint)
		default:
			logs.Trace("[P2P] skip unsupported probe endpoint provider=%s mode=%s network=%s address=%s", endpoint.Provider, endpoint.Mode, endpoint.Network, endpoint.Address)
		}
	}
	if len(stunEndpoints) == 0 && len(npsEndpoints) == 0 {
		return NatObservation{}, fmt.Errorf("unsupported probe configuration provider=%s mode=%s network=%s", start.Probe.Provider, start.Probe.Mode, start.Probe.Network)
	}
	if len(npsEndpoints) == 0 {
		return NatObservation{}, fmt.Errorf("missing required nps probe endpoints")
	}

	samples := make([]ProbeSample, 0, len(endpoints))
	if len(stunEndpoints) > 0 {
		stunSamples, err := runSTUNProbes(ctx, localConn, stunEndpoints, start.Timeouts.ProbeTimeoutMs)
		if err != nil {
			logs.Trace("[P2P] stun calibration unavailable err=%v", err)
		}
		if len(stunSamples) > 0 {
			for _, sample := range stunSamples {
				logs.Info("[P2P] stun calibration success endpoint=%s observed=%s", sample.EndpointID, sample.ObservedAddr)
			}
			samples = append(samples, stunSamples...)
		}
	}

	npsSamples, sawExtraReply, err := runNPSProbe(ctx, localConn, start, npsEndpoints)
	if err != nil {
		return NatObservation{}, err
	}
	samples = append(samples, npsSamples...)
	extraReplySeen := effectiveExtraReplySeen(sawExtraReply, start.Probe.ExpectExtraReply)
	return BuildNatObservation(samples, extraReplySeen), nil
}

func runNPSProbe(ctx context.Context, localConn net.PacketConn, start P2PPunchStart, endpoints []P2PProbeEndpoint) ([]ProbeSample, bool, error) {
	endpoints = compatibleNPSProbeEndpoints(localConn.LocalAddr(), endpoints)
	if len(endpoints) == 0 {
		return nil, false, fmt.Errorf("no compatible nps probe endpoints for %s", addrString(localConn.LocalAddr()))
	}
	samples := make(map[int]ProbeSample, len(endpoints))
	var extraReplySeen bool
	buf := common.BufPoolUdp.Get()
	defer common.BufPoolUdp.Put(buf)
	resolveNetwork := probeResolveNetwork(localConn.LocalAddr())

	sendProbe := func() {
		for _, endpoint := range endpoints {
			udpAddr, err := net.ResolveUDPAddr(resolveNetwork, endpoint.Address)
			if err != nil {
				continue
			}
			packet := newUDPPacket(start.SessionID, start.Token, start.Role, packetTypeProbe)
			packet.ProbePort = common.GetPortByAddr(endpoint.Address)
			raw, err := EncodeUDPPacket(packet)
			if err != nil {
				continue
			}
			_, _ = localConn.WriteTo(raw, udpAddr)
		}
	}

	sendProbe()
	deadline := time.Now().Add(time.Duration(start.Timeouts.ProbeTimeoutMs) * time.Millisecond)
	if start.Timeouts.ProbeTimeoutMs <= 0 {
		deadline = time.Now().Add(5 * time.Second)
	}
	for time.Now().Before(deadline) {
		if len(samples) >= len(endpoints) && (!start.Probe.ExpectExtraReply || extraReplySeen) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, false, mapP2PContextError(ctx.Err())
		default:
		}
		_ = localConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, replyAddr, err := localConn.ReadFrom(buf)
		_ = localConn.SetReadDeadline(time.Time{})
		if err != nil {
			if conn.IsTimeout(err) {
				sendProbe()
				continue
			}
			if isIgnorableUDPIcmpError(err) {
				continue
			}
			return nil, false, err
		}
		packet, err := DecodeUDPPacket(buf[:n], start.Token)
		if err != nil {
			continue
		}
		if packet.SessionID != start.SessionID || packet.Token != start.Token {
			continue
		}
		if packet.Type != packetTypeAck && packet.Type != packetTypeProbeX {
			continue
		}
		sample := ProbeSample{
			Provider:        ProbeProviderNPS,
			Mode:            ProbeModeUDP,
			ProbePort:       packet.ProbePort,
			ObservedAddr:    packet.ObservedAddr,
			ServerReplyAddr: replyAddr.String(),
			ExtraReply:      packet.ExtraReply || packet.Type == packetTypeProbeX,
		}
		if sample.ExtraReply {
			extraReplySeen = true
		}
		if sample.ProbePort != 0 && sample.ObservedAddr != "" {
			samples[sample.ProbePort] = sample
		}
	}
	out := make([]ProbeSample, 0, len(samples))
	for _, sample := range samples {
		out = append(out, sample)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProbePort < out[j].ProbePort })
	return out, extraReplySeen, nil
}

func compatibleNPSProbeEndpoints(localAddr net.Addr, endpoints []P2PProbeEndpoint) []P2PProbeEndpoint {
	localIP := common.ParseIPFromAddr(addrString(localAddr))
	if localIP == nil {
		return append([]P2PProbeEndpoint(nil), endpoints...)
	}
	wantIPv6 := localIP.To4() == nil
	out := make([]P2PProbeEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		host := common.GetIpByAddr(endpoint.Address)
		ip := net.ParseIP(host)
		if ip != nil && (ip.To4() == nil) != wantIPv6 {
			continue
		}
		out = append(out, endpoint)
	}
	return out
}

func probeResolveNetwork(localAddr net.Addr) string {
	localIP := common.ParseIPFromAddr(addrString(localAddr))
	if localIP != nil && localIP.To4() == nil {
		return "udp6"
	}
	return "udp4"
}

func buildLocalAddrs(localAddr net.Addr) []string {
	out := make([]string, 0, 8)
	port := ""
	if localAddr != nil {
		port = common.GetPortStrByAddr(localAddr.String())
	}
	add := func(ip net.IP) {
		if ip == nil || port == "" {
			return
		}
		addr := net.JoinHostPort(ip.String(), port)
		if addr == "" {
			return
		}
		for _, existing := range out {
			if existing == addr {
				return
			}
		}
		out = append(out, addr)
	}
	if ip := common.ParseIPFromAddr(addrString(localAddr)); ip != nil {
		add(ip)
	}
	if udp4, err := common.GetLocalUdp4IP(); err == nil {
		add(udp4)
	}
	if udp6, err := common.GetLocalUdp6IP(); err == nil {
		add(udp6)
	}
	for _, ip := range listInterfaceIPs() {
		add(ip)
	}
	return out
}

func listInterfaceIPs() []net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make([]net.IP, 0, 8)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			ip = common.NormalizeIP(ip)
			if ip == nil || common.IsZeroIP(ip) || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			out = append(out, ip)
		}
	}
	return out
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

func firstObservedAddr(peer P2PPeerInfo) string {
	for _, sample := range peer.Nat.Samples {
		if addr := common.ValidateAddr(sample.ObservedAddr); addr != "" {
			return addr
		}
	}
	if peer.Nat.PublicIP != "" && peer.Nat.ObservedBasePort > 0 {
		return net.JoinHostPort(peer.Nat.PublicIP, fmt.Sprintf("%d", peer.Nat.ObservedBasePort))
	}
	return ""
}

func negotiateTransportMode(selfMode, peerMode string) string {
	selfMode = strings.TrimSpace(selfMode)
	peerMode = strings.TrimSpace(peerMode)
	if selfMode == "" {
		selfMode = common.CONN_KCP
	}
	if peerMode == "" {
		peerMode = selfMode
	}
	if selfMode == peerMode {
		return selfMode
	}
	return common.CONN_KCP
}

func ChooseLocalProbeAddr(probe P2PProbeConfig) (string, error) {
	for _, endpoint := range NormalizeProbeEndpoints(probe) {
		host := common.GetIpByAddr(endpoint.Address)
		if host == "" {
			continue
		}
		ip := net.ParseIP(host)
		if ip == nil {
			continue
		}
		if ip.To4() == nil {
			addr, err := common.GetLocalUdp6Addr()
			if err != nil {
				return "", err
			}
			return addr.String(), nil
		}
		addr, err := common.GetLocalUdp4Addr()
		if err != nil {
			return "", err
		}
		return addr.String(), nil
	}
	addr, err := common.GetLocalUdp4Addr()
	if err == nil {
		return addr.String(), nil
	}
	addr6, err6 := common.GetLocalUdp6Addr()
	if err6 != nil {
		return "", err
	}
	return addr6.String(), nil
}

func probeEndpointAddrs(probe P2PProbeConfig) []string {
	endpoints := NormalizeProbeEndpoints(probe)
	addrs := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		addr := endpoint.Address
		if endpoint.Provider == ProbeProviderSTUN {
			if host, port, err := net.SplitHostPort(addr); err == nil && host != "" && port != "" && common.GetPortByAddr(addr) > 0 {
				addrs = append(addrs, addr)
			}
			continue
		}
		if validated := common.ValidateAddr(addr); validated != "" {
			addrs = append(addrs, validated)
		}
	}
	return addrs
}

func (s *runtimeSession) addSocket(socket net.PacketConn) {
	if socket == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.sockets {
		if existing == socket {
			return
		}
	}
	s.sockets = append(s.sockets, socket)
}

func (s *runtimeSession) snapshotSockets() []net.PacketConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]net.PacketConn, 0, len(s.sockets))
	out = append(out, s.sockets...)
	return out
}

func (s *runtimeSession) closeSockets(keep net.PacketConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, socket := range s.sockets {
		if socket == nil || socket == keep {
			continue
		}
		_ = socket.Close()
	}
	if keep != nil {
		s.sockets = []net.PacketConn{keep}
		return
	}
	s.sockets = nil
}

func (s *runtimeSession) enableBirthdayFallback() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.plan.UseBirthdayAttack = true
}

func (s *runtimeSession) candidateMaintenanceLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	maxAge := 5 * time.Second
	if s.plan.HandshakeTimeout > 0 && s.plan.HandshakeTimeout < 20*time.Second {
		maxAge = s.plan.HandshakeTimeout / 3
		if maxAge < 2*time.Second {
			maxAge = 2 * time.Second
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pruned := s.cm.PruneStale(maxAge)
			s.cm.CleanupClosed(maxAge)
			if pruned > 0 {
				_ = s.sendProgress("candidate_pruned", "ok", fmt.Sprintf("%d", pruned), addrString(s.localConn.LocalAddr()), s.cm.CandidateRemote(), map[string]string{
					"max_age_ms": fmt.Sprintf("%d", maxAge.Milliseconds()),
				})
			}
		}
	}
}
