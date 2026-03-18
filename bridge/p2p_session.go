package bridge

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beego/beego"
	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/crypt"
	"github.com/djylb/nps/lib/file"
	"github.com/djylb/nps/lib/logs"
	"github.com/djylb/nps/lib/p2p"
	"github.com/djylb/nps/server/connection"
	"github.com/djylb/nps/server/p2pstate"
)

type p2pSessionManager struct {
	sessions sync.Map
}

type p2pBridgeSession struct {
	mgr             *p2pSessionManager
	id              string
	token           string
	visitorID       int
	providerID      int
	task            *file.Tunnel
	timeouts        p2p.P2PTimeouts
	visitorControl  *conn.Conn
	providerControl *conn.Conn
	visitorReport   *p2p.P2PProbeReport
	providerReport  *p2p.P2PProbeReport
	visitorStart    p2p.P2PPunchStart
	providerStart   p2p.P2PPunchStart
	timer           *time.Timer
	mu              sync.Mutex
	closed          bool
	summarySent     bool
}

func newP2PSessionManager() *p2pSessionManager {
	return &p2pSessionManager{}
}

func (m *p2pSessionManager) create(visitorID, providerID int, task *file.Tunnel, visitorControl *conn.Conn, visitorProbe, providerProbe p2p.P2PProbeConfig) (*p2pBridgeSession, error) {
	if !p2p.HasProbeProvider(visitorProbe, p2p.ProbeProviderNPS) || !p2p.HasProbeProvider(providerProbe, p2p.ProbeProviderNPS) {
		return nil, fmt.Errorf("p2p probe port is not configured")
	}
	sessionID := crypt.GenerateUUID(strconv.Itoa(visitorID), strconv.Itoa(providerID), task.Password, time.Now().String()).String()
	token, err := generateP2PSessionToken()
	if err != nil {
		return nil, err
	}
	timeouts := loadP2PTimeouts()
	ttl := sessionTTL(timeouts)
	session := &p2pBridgeSession{
		mgr:            m,
		id:             sessionID,
		token:          token,
		visitorID:      visitorID,
		providerID:     providerID,
		task:           task,
		timeouts:       timeouts,
		visitorControl: visitorControl,
		visitorStart: p2p.P2PPunchStart{
			SessionID: sessionID,
			Token:     token,
			Role:      common.WORK_P2P_VISITOR,
			PeerRole:  common.WORK_P2P_PROVIDER,
			Probe:     visitorProbe,
			Timeouts:  timeouts,
		},
		providerStart: p2p.P2PPunchStart{
			SessionID: sessionID,
			Token:     token,
			Role:      common.WORK_P2P_PROVIDER,
			PeerRole:  common.WORK_P2P_VISITOR,
			Probe:     providerProbe,
			Timeouts:  timeouts,
		},
	}
	session.timer = time.AfterFunc(ttl, func() {
		session.abort("session timeout")
	})
	m.sessions.Store(sessionID, session)
	p2pstate.Register(sessionID, token, ttl)
	return session, nil
}

func (m *p2pSessionManager) get(sessionID string) (*p2pBridgeSession, bool) {
	value, ok := m.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	session, ok := value.(*p2pBridgeSession)
	return session, ok
}

func (s *p2pBridgeSession) attachProvider(control *conn.Conn) {
	s.mu.Lock()
	s.providerControl = control
	s.mu.Unlock()
}

func (s *p2pBridgeSession) serve(role string, control *conn.Conn) {
	for {
		flag, err := control.ReadFlag()
		if err != nil {
			s.abortPending(fmt.Sprintf("%s control closed", role))
			return
		}
		switch flag {
		case common.P2P_PROBE_REPORT:
			raw, err := control.GetShortLenContent()
			if err != nil {
				s.abortPending(fmt.Sprintf("%s report read failed", role))
				continue
			}
			var report p2p.P2PProbeReport
			if err := json.Unmarshal(raw, &report); err != nil {
				s.abortPending(fmt.Sprintf("%s report decode failed", role))
				continue
			}
			s.handleReport(role, &report)
		case common.P2P_PUNCH_PROGRESS:
			raw, err := control.GetShortLenContent()
			if err != nil {
				continue
			}
			var progress p2p.P2PPunchProgress
			if err := json.Unmarshal(raw, &progress); err != nil {
				continue
			}
			logs.Info("[P2P] session=%s role=%s stage=%s status=%s local=%s remote=%s detail=%s meta=%v counters=%v",
				progress.SessionID, progress.Role, progress.Stage, progress.Status, progress.LocalAddr, progress.RemoteAddr, progress.Detail, progress.Meta, progress.Counters)
		case common.P2P_PUNCH_ABORT:
			raw, err := control.GetShortLenContent()
			if err != nil {
				s.abortPending(fmt.Sprintf("%s abort read failed", role))
				continue
			}
			var abort p2p.P2PPunchAbort
			if err := json.Unmarshal(raw, &abort); err != nil {
				s.abortPending(fmt.Sprintf("%s abort decode failed", role))
				continue
			}
			s.abort(abort.Reason)
		default:
			if _, err := control.GetShortLenContent(); err != nil {
				return
			}
		}
	}
}

func (s *p2pBridgeSession) handleReport(role string, report *p2p.P2PProbeReport) {
	s.mu.Lock()
	if s.closed || report == nil || report.SessionID != s.id || report.Token != s.token {
		s.mu.Unlock()
		return
	}
	switch role {
	case common.WORK_P2P_VISITOR:
		s.visitorReport = report
	case common.WORK_P2P_PROVIDER:
		s.providerReport = report
	}
	ready := s.visitorReport != nil && s.providerReport != nil
	s.mu.Unlock()
	if ready {
		s.sendSummary()
	}
}

func (s *p2pBridgeSession) sendSummary() {
	s.mu.Lock()
	if s.closed || s.summarySent || s.visitorReport == nil || s.providerReport == nil {
		s.mu.Unlock()
		return
	}
	visitorSelf := mergeProbeObservation(s.id, s.visitorReport.Self)
	providerSelf := mergeProbeObservation(s.id, s.providerReport.Self)
	visitorSummary := p2p.P2PProbeSummary{
		SessionID: s.id,
		Token:     s.token,
		Role:      common.WORK_P2P_VISITOR,
		PeerRole:  common.WORK_P2P_PROVIDER,
		Self:      visitorSelf,
		Peer:      providerSelf,
		Timeouts:  s.timeouts,
		Hints:     buildSummaryHints(visitorSelf, providerSelf),
	}
	providerSummary := p2p.P2PProbeSummary{
		SessionID: s.id,
		Token:     s.token,
		Role:      common.WORK_P2P_PROVIDER,
		PeerRole:  common.WORK_P2P_VISITOR,
		Self:      providerSelf,
		Peer:      visitorSelf,
		Timeouts:  s.timeouts,
		Hints:     visitorSummary.Hints,
	}
	visitorControl := s.visitorControl
	providerControl := s.providerControl
	s.summarySent = true
	s.mu.Unlock()

	if visitorControl != nil {
		if err := p2p.WriteBridgeMessage(visitorControl, common.P2P_PROBE_SUMMARY, visitorSummary); err != nil {
			s.abort("send visitor summary failed")
			return
		}
	}
	if providerControl != nil {
		if err := p2p.WriteBridgeMessage(providerControl, common.P2P_PROBE_SUMMARY, providerSummary); err != nil {
			s.abort("send provider summary failed")
			return
		}
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.mu.Unlock()
	logs.Info("[P2P] session=%s summary sent via bridge", s.id)
	p2pstate.Unregister(s.id)
	s.mgr.sessions.Delete(s.id)
}

func (s *p2pBridgeSession) abort(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
	}
	p2pstate.Unregister(s.id)
	s.mgr.sessions.Delete(s.id)
	if s.visitorControl != nil {
		_ = p2p.WriteBridgeMessage(s.visitorControl, common.P2P_PUNCH_ABORT, p2p.P2PPunchAbort{
			SessionID: s.id,
			Role:      common.WORK_P2P_VISITOR,
			Reason:    reason,
		})
		_ = s.visitorControl.Close()
	}
	if s.providerControl != nil {
		_ = p2p.WriteBridgeMessage(s.providerControl, common.P2P_PUNCH_ABORT, p2p.P2PPunchAbort{
			SessionID: s.id,
			Role:      common.WORK_P2P_PROVIDER,
			Reason:    reason,
		})
		_ = s.providerControl.Close()
	}
}

func (s *p2pBridgeSession) abortPending(reason string) {
	s.mu.Lock()
	shouldAbort := !s.closed && !s.summarySent
	s.mu.Unlock()
	if shouldAbort {
		s.abort(reason)
	}
}

func buildProbeConfig(c *conn.Conn) p2p.P2PProbeConfig {
	expectExtraReply := beego.AppConfig.DefaultBool("p2p_probe_extra_reply", true)
	cfg := p2p.P2PProbeConfig{
		Version:          2,
		Provider:         p2p.ProbeProviderNPS,
		Mode:             p2p.ProbeModeUDP,
		Network:          p2p.ProbeNetworkUDP,
		ExpectExtraReply: expectExtraReply,
		Options:          buildProbeOptions(),
	}
	baseHosts := collectProbeBaseHosts(c)
	endpoints := make([]p2p.P2PProbeEndpoint, 0, len(baseHosts)*3+len(configuredSTUNServers()))
	if connection.P2pPort > 0 {
		for hostIndex, host := range baseHosts {
			for i := 0; i < 3; i++ {
				endpoints = append(endpoints, p2p.P2PProbeEndpoint{
					ID:       fmt.Sprintf("probe-%d-%d", hostIndex+1, i+1),
					Provider: p2p.ProbeProviderNPS,
					Mode:     p2p.ProbeModeUDP,
					Network:  p2p.ProbeNetworkUDP,
					Address:  common.BuildAddress(host, strconv.Itoa(connection.P2pPort+i)),
				})
			}
		}
	}
	for i, addr := range configuredSTUNServers() {
		endpoints = append(endpoints, p2p.P2PProbeEndpoint{
			ID:       fmt.Sprintf("stun-%d", i+1),
			Provider: p2p.ProbeProviderSTUN,
			Mode:     p2p.ProbeModeBinding,
			Network:  p2p.ProbeNetworkUDP,
			Address:  addr,
		})
	}
	cfg.Endpoints = endpoints
	if connection.P2pPort > 0 {
		cfg.Options["base_port"] = strconv.Itoa(connection.P2pPort)
	}
	return cfg
}

func collectProbeBaseHosts(c *conn.Conn) []string {
	hosts := make([]string, 0, 3)
	seen := make(map[string]struct{}, 4)
	addHost := func(host string) {
		host = strings.TrimSpace(host)
		host = strings.Trim(host, "[]")
		if host == "" {
			return
		}
		if ip := net.ParseIP(host); ip != nil {
			ip = common.NormalizeIP(ip)
			if ip == nil || common.IsZeroIP(ip) || ip.IsUnspecified() {
				return
			}
			host = ip.String()
		}
		if _, ok := seen[host]; ok {
			return
		}
		seen[host] = struct{}{}
		hosts = append(hosts, host)
	}

	if c != nil && c.Conn != nil {
		addHost(common.GetIpByAddr(c.LocalAddr().String()))
	}
	addHost(strings.TrimSpace(connection.P2pIp))
	addHost(common.GetServerIp(connection.P2pIp))
	return hosts
}

func mergeProbeObservation(sessionID string, self p2p.P2PPeerInfo) p2p.P2PPeerInfo {
	serverSamples := p2pstate.GetObservations(sessionID, self.Role)
	if len(serverSamples) == 0 {
		return self
	}
	combinedSamples := p2p.MergeProbeSamples(serverSamples, self.Nat.Samples)
	mergedNat := p2p.BuildNatObservation(combinedSamples, !self.Nat.ProbePortRestricted)
	if self.Nat.MappingConfidenceLow {
		mergedNat.MappingConfidenceLow = true
	}
	mergedNat.ConflictingSignals = mergedNat.ConflictingSignals || self.Nat.ConflictingSignals
	if self.Nat.PortMapping != nil {
		mergedNat.PortMapping = self.Nat.PortMapping
	}
	self.Nat = mergedNat
	return self
}

func generateP2PSessionToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func buildSummaryHints(self, peer p2p.P2PPeerInfo) map[string]any {
	return map[string]any{
		"probe_port_restricted":     self.Nat.ProbePortRestricted || peer.Nat.ProbePortRestricted,
		"mapping_confidence_low":    self.Nat.MappingConfidenceLow || peer.Nat.MappingConfidenceLow,
		"filtering_likely":          self.Nat.ProbePortRestricted || peer.Nat.ProbePortRestricted,
		"self_probe_ip_count":       self.Nat.ProbeIPCount,
		"peer_probe_ip_count":       peer.Nat.ProbeIPCount,
		"self_conflicting_signals":  self.Nat.ConflictingSignals,
		"peer_conflicting_signals":  peer.Nat.ConflictingSignals,
		"self_port_mapping_method":  portMappingMethod(self.Nat.PortMapping),
		"peer_port_mapping_method":  portMappingMethod(peer.Nat.PortMapping),
		"self_probe_provider_count": probeProviderCount(self.Nat.Samples),
		"peer_probe_provider_count": probeProviderCount(peer.Nat.Samples),
		"self_observed_base_port":   self.Nat.ObservedBasePort,
		"self_observed_interval":    self.Nat.ObservedInterval,
		"self_mapping_behavior":     self.Nat.MappingBehavior,
		"self_filtering_behavior":   self.Nat.FilteringBehavior,
		"self_classification_level": self.Nat.ClassificationLevel,
		"peer_observed_base_port":   peer.Nat.ObservedBasePort,
		"peer_observed_interval":    peer.Nat.ObservedInterval,
		"peer_mapping_behavior":     peer.Nat.MappingBehavior,
		"peer_filtering_behavior":   peer.Nat.FilteringBehavior,
		"peer_classification_level": peer.Nat.ClassificationLevel,
	}
}

func buildProbeOptions() map[string]string {
	return map[string]string{
		"layout":                       "triple-port",
		"probe_extra_reply":            strconv.FormatBool(beego.AppConfig.DefaultBool("p2p_probe_extra_reply", true)),
		"force_predict_on_restricted":  strconv.FormatBool(beego.AppConfig.DefaultBool("p2p_force_predict_on_restricted", true)),
		"enable_target_spray":          strconv.FormatBool(beego.AppConfig.DefaultBool("p2p_enable_target_spray", true)),
		"enable_birthday_attack":       strconv.FormatBool(beego.AppConfig.DefaultBool("p2p_enable_birthday_attack", false)),
		"enable_upnp_portmap":          strconv.FormatBool(beego.AppConfig.DefaultBool("p2p_enable_upnp_portmap", false)),
		"enable_pcp_portmap":           strconv.FormatBool(beego.AppConfig.DefaultBool("p2p_enable_pcp_portmap", false)),
		"enable_natpmp_portmap":        strconv.FormatBool(beego.AppConfig.DefaultBool("p2p_enable_natpmp_portmap", false)),
		"portmap_lease_seconds":        strconv.Itoa(beego.AppConfig.DefaultInt("p2p_portmap_lease_seconds", 3600)),
		"default_prediction_interval":  strconv.Itoa(beego.AppConfig.DefaultInt("p2p_default_prediction_interval", p2p.DefaultPredictionInterval)),
		"target_spray_span":            strconv.Itoa(beego.AppConfig.DefaultInt("p2p_target_spray_span", p2p.DefaultSpraySpan)),
		"target_spray_rounds":          strconv.Itoa(beego.AppConfig.DefaultInt("p2p_target_spray_rounds", p2p.DefaultSprayRounds)),
		"target_spray_burst":           strconv.Itoa(beego.AppConfig.DefaultInt("p2p_target_spray_burst", p2p.DefaultSprayBurst)),
		"target_spray_packet_sleep_ms": strconv.Itoa(beego.AppConfig.DefaultInt("p2p_target_spray_packet_sleep_ms", 3)),
		"target_spray_burst_gap_ms":    strconv.Itoa(beego.AppConfig.DefaultInt("p2p_target_spray_burst_gap_ms", 10)),
		"target_spray_phase_gap_ms":    strconv.Itoa(beego.AppConfig.DefaultInt("p2p_target_spray_phase_gap_ms", 40)),
		"birthday_listen_ports":        strconv.Itoa(beego.AppConfig.DefaultInt("p2p_birthday_listen_ports", p2p.DefaultBirthdayPorts)),
		"birthday_targets_per_port":    strconv.Itoa(beego.AppConfig.DefaultInt("p2p_birthday_targets_per_port", p2p.DefaultBirthdayTargets)),
	}
}

func loadP2PTimeouts() p2p.P2PTimeouts {
	return p2p.P2PTimeouts{
		ProbeTimeoutMs:     beego.AppConfig.DefaultInt("p2p_probe_timeout_ms", 5000),
		HandshakeTimeoutMs: beego.AppConfig.DefaultInt("p2p_handshake_timeout_ms", 20000),
		TransportTimeoutMs: beego.AppConfig.DefaultInt("p2p_transport_timeout_ms", 10000),
	}
}

func sessionTTL(timeouts p2p.P2PTimeouts) time.Duration {
	ttl := time.Duration(timeouts.ProbeTimeoutMs+timeouts.HandshakeTimeoutMs+5000) * time.Millisecond
	if ttl < 30*time.Second {
		return 30 * time.Second
	}
	return ttl
}

func portMappingMethod(info *p2p.PortMappingInfo) string {
	if info == nil {
		return ""
	}
	return info.Method
}

func configuredSTUNServers() []string {
	return p2p.ParseSTUNServerList(beego.AppConfig.String("p2p_stun_servers"))
}

func probeProviderCount(samples []p2p.ProbeSample) int {
	if len(samples) == 0 {
		return 0
	}
	providers := make(map[string]struct{}, len(samples))
	for _, sample := range samples {
		provider := sample.Provider
		if provider == "" {
			provider = p2p.ProbeProviderNPS
		}
		providers[provider] = struct{}{}
	}
	return len(providers)
}
