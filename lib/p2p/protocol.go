package p2p

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
)

const (
	udpPacketVersion = 2
	packetTypeProbe  = "probe"
	packetTypeAck    = "probe_ack"
	packetTypeProbeX = "probe_extra"
	packetTypePunch  = "punch"
	packetTypeSucc   = "success"
	packetTypeEnd    = "end"
	packetTypeAccept = "accept"

	ProbeProviderNPS  = "nps"
	ProbeProviderSTUN = "stun"
	ProbeModeUDP      = "udp_probe"
	ProbeModeBinding  = "stun_binding"
	ProbeNetworkUDP   = "udp"

	NATMappingUnknown             = "unknown"
	NATMappingEndpointIndependent = "endpoint_independent"
	NATMappingEndpointDependent   = "endpoint_dependent"
	NATFilteringUnknown           = "unknown"
	NATFilteringOpen              = "open_or_address_restricted"
	NATFilteringPortRestricted    = "port_restricted"
	NATTypeUnknown                = "unknown"
	NATTypeCone                   = "cone"
	NATTypePortRestricted         = "port_restricted"
	NATTypeSymmetric              = "symmetric"
	ClassificationConfidenceLow   = "low"
	ClassificationConfidenceMed   = "medium"
	ClassificationConfidenceHigh  = "high"
)

type ProbeSample struct {
	EndpointID      string `json:"endpoint_id,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Mode            string `json:"mode,omitempty"`
	ProbePort       int    `json:"probe_port"`
	ObservedAddr    string `json:"observed_addr"`
	ServerReplyAddr string `json:"server_reply_addr,omitempty"`
	ExtraReply      bool   `json:"extra_reply,omitempty"`
}

type PortMappingInfo struct {
	Method       string `json:"method,omitempty"`
	ExternalAddr string `json:"external_addr,omitempty"`
	InternalAddr string `json:"internal_addr,omitempty"`
	LeaseSeconds int    `json:"lease_seconds,omitempty"`
}

type NatObservation struct {
	PublicIP             string           `json:"public_ip"`
	ObservedBasePort     int              `json:"observed_base_port"`
	ObservedInterval     int              `json:"observed_interval"`
	ProbePortRestricted  bool             `json:"probe_port_restricted"`
	MappingBehavior      string           `json:"mapping_behavior,omitempty"`
	FilteringBehavior    string           `json:"filtering_behavior,omitempty"`
	NATType              string           `json:"nat_type,omitempty"`
	ClassificationLevel  string           `json:"classification_level,omitempty"`
	ProbeIPCount         int              `json:"probe_ip_count,omitempty"`
	ConflictingSignals   bool             `json:"conflicting_signals,omitempty"`
	MappingConfidenceLow bool             `json:"mapping_confidence_low"`
	PortMapping          *PortMappingInfo `json:"port_mapping,omitempty"`
	Samples              []ProbeSample    `json:"samples,omitempty"`
}

type P2PTimeouts struct {
	ProbeTimeoutMs     int `json:"probe_timeout_ms"`
	HandshakeTimeoutMs int `json:"handshake_timeout_ms"`
	TransportTimeoutMs int `json:"transport_timeout_ms"`
}

type P2PPeerInfo struct {
	Role          string         `json:"role"`
	Nat           NatObservation `json:"nat"`
	LocalAddrs    []string       `json:"local_addrs,omitempty"`
	TransportMode string         `json:"transport_mode,omitempty"`
	TransportData string         `json:"transport_data,omitempty"`
}

type P2PProbeEndpoint struct {
	ID       string            `json:"id,omitempty"`
	Provider string            `json:"provider,omitempty"`
	Mode     string            `json:"mode,omitempty"`
	Network  string            `json:"network,omitempty"`
	Address  string            `json:"address"`
	Options  map[string]string `json:"options,omitempty"`
}

type P2PProbeConfig struct {
	Version          int                `json:"version"`
	Provider         string             `json:"provider"`
	Mode             string             `json:"mode"`
	Network          string             `json:"network"`
	Endpoints        []P2PProbeEndpoint `json:"endpoints"`
	ExpectExtraReply bool               `json:"expect_extra_reply,omitempty"`
	Options          map[string]string  `json:"options,omitempty"`
}

type P2PPunchStart struct {
	SessionID string         `json:"session_id"`
	Token     string         `json:"token"`
	Role      string         `json:"role"`
	PeerRole  string         `json:"peer_role"`
	Probe     P2PProbeConfig `json:"probe"`
	Timeouts  P2PTimeouts    `json:"timeouts"`
}

type P2PProbeReport struct {
	SessionID string      `json:"session_id"`
	Token     string      `json:"token"`
	Role      string      `json:"role"`
	PeerRole  string      `json:"peer_role"`
	Self      P2PPeerInfo `json:"self"`
}

type P2PProbeSummary struct {
	SessionID string         `json:"session_id"`
	Token     string         `json:"token"`
	Role      string         `json:"role"`
	PeerRole  string         `json:"peer_role"`
	Self      P2PPeerInfo    `json:"self"`
	Peer      P2PPeerInfo    `json:"peer"`
	Timeouts  P2PTimeouts    `json:"timeouts"`
	Hints     map[string]any `json:"hints,omitempty"`
}

type P2PPunchProgress struct {
	SessionID  string            `json:"session_id"`
	Role       string            `json:"role"`
	Stage      string            `json:"stage"`
	Status     string            `json:"status,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	LocalAddr  string            `json:"local_addr,omitempty"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
	Timestamp  int64             `json:"timestamp,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
	Counters   map[string]int    `json:"counters,omitempty"`
}

type P2PPunchAbort struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Reason    string `json:"reason"`
}

type P2PSessionJoin struct {
	SessionID string `json:"session_id"`
	Token     string `json:"token"`
}

type UDPPacket struct {
	Version      int    `json:"version"`
	SessionID    string `json:"session_id"`
	Token        string `json:"-"`
	Type         string `json:"type,omitempty"`
	Role         string `json:"role,omitempty"`
	ProbePort    int    `json:"probe_port,omitempty"`
	ObservedAddr string `json:"observed_addr,omitempty"`
	ExtraReply   bool   `json:"extra_reply,omitempty"`
	Timestamp    int64  `json:"timestamp"`
	Nonce        string `json:"nonce,omitempty"`
	Ciphertext   string `json:"ciphertext,omitempty"`
	HMAC         string `json:"hmac,omitempty"`
}

type udpWirePacket struct {
	Version    int    `json:"version"`
	SessionID  string `json:"session_id"`
	Timestamp  int64  `json:"timestamp"`
	Nonce      string `json:"nonce,omitempty"`
	Ciphertext string `json:"ciphertext,omitempty"`
	HMAC       string `json:"hmac,omitempty"`
}

type udpPacketPayload struct {
	Type         string `json:"type"`
	Role         string `json:"role,omitempty"`
	ProbePort    int    `json:"probe_port,omitempty"`
	ObservedAddr string `json:"observed_addr,omitempty"`
	ExtraReply   bool   `json:"extra_reply,omitempty"`
}

type udpPacketAAD struct {
	Version   int    `json:"version"`
	SessionID string `json:"session_id"`
	Token     string `json:"token"`
	Timestamp int64  `json:"timestamp"`
	Nonce     string `json:"nonce"`
}

func newUDPPacket(sessionID, token, role, typ string) *UDPPacket {
	return &UDPPacket{
		Version:   udpPacketVersion,
		Type:      typ,
		SessionID: sessionID,
		Token:     token,
		Role:      role,
		Timestamp: time.Now().UnixMilli(),
		Nonce:     newNonce(8),
	}
}

func EncodeUDPPacket(pkt *UDPPacket) ([]byte, error) {
	if pkt == nil {
		return nil, fmt.Errorf("nil udp packet")
	}
	if pkt.Version == 0 {
		pkt.Version = udpPacketVersion
	}
	if pkt.Timestamp == 0 {
		pkt.Timestamp = time.Now().UnixMilli()
	}
	payload, err := json.Marshal(udpPacketPayload{
		Type:         pkt.Type,
		Role:         pkt.Role,
		ProbePort:    pkt.ProbePort,
		ObservedAddr: pkt.ObservedAddr,
		ExtraReply:   pkt.ExtraReply,
	})
	if err != nil {
		return nil, err
	}
	sealed, err := sealUDPPayload(pkt, payload)
	if err != nil {
		return nil, err
	}
	wire := udpWirePacket{
		Version:    pkt.Version,
		SessionID:  pkt.SessionID,
		Timestamp:  pkt.Timestamp,
		Nonce:      pkt.Nonce,
		Ciphertext: base64.RawURLEncoding.EncodeToString(sealed),
	}
	aad, err := encodeUDPPacketAAD(wire.Version, wire.SessionID, pkt.Token, wire.Timestamp, wire.Nonce)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, []byte(pkt.Token))
	_, _ = mac.Write(aad)
	_, _ = mac.Write(sealed)
	wire.HMAC = hex.EncodeToString(mac.Sum(nil))
	return json.Marshal(wire)
}

func DecodeUDPPacket(data []byte, token string) (*UDPPacket, error) {
	wire, err := parseUDPPacketWire(data)
	if err != nil {
		return nil, err
	}
	return decodeUDPPacketWire(wire, token)
}

func DecodeUDPPacketWithLookup(data []byte, lookup func(sessionID string) (string, bool)) (*UDPPacket, error) {
	wire, err := parseUDPPacketWire(data)
	if err != nil {
		return nil, err
	}
	if lookup == nil {
		return nil, ErrP2PTokenMismatch
	}
	token, ok := lookup(wire.SessionID)
	if !ok || token == "" {
		return nil, ErrP2PTokenMismatch
	}
	return decodeUDPPacketWire(wire, token)
}

func parseUDPPacketWire(data []byte) (udpWirePacket, error) {
	var wire udpWirePacket
	if err := json.Unmarshal(data, &wire); err != nil {
		return wire, err
	}
	if wire.SessionID == "" || wire.HMAC == "" || wire.Ciphertext == "" || wire.Nonce == "" {
		return wire, ErrP2PTokenMismatch
	}
	return wire, nil
}

func decodeUDPPacketWire(wire udpWirePacket, token string) (*UDPPacket, error) {
	if token == "" {
		return nil, ErrP2PTokenMismatch
	}
	gotMac, err := hex.DecodeString(wire.HMAC)
	if err != nil {
		return nil, ErrP2PTokenMismatch
	}
	sealed, err := base64.RawURLEncoding.DecodeString(wire.Ciphertext)
	if err != nil {
		return nil, ErrP2PTokenMismatch
	}
	aad, err := encodeUDPPacketAAD(wire.Version, wire.SessionID, token, wire.Timestamp, wire.Nonce)
	if err != nil {
		return nil, err
	}
	want := hmac.New(sha256.New, []byte(token))
	_, _ = want.Write(aad)
	_, _ = want.Write(sealed)
	if !hmac.Equal(gotMac, want.Sum(nil)) {
		return nil, ErrP2PTokenMismatch
	}
	plaintext, err := openUDPPayload(UDPPacket{
		Version:   wire.Version,
		SessionID: wire.SessionID,
		Token:     token,
		Timestamp: wire.Timestamp,
		Nonce:     wire.Nonce,
	}, sealed)
	if err != nil {
		return nil, ErrP2PTokenMismatch
	}
	var payload udpPacketPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, ErrP2PTokenMismatch
	}
	pkt := &UDPPacket{
		Version:      wire.Version,
		SessionID:    wire.SessionID,
		Token:        token,
		Type:         payload.Type,
		Role:         payload.Role,
		ProbePort:    payload.ProbePort,
		ObservedAddr: payload.ObservedAddr,
		ExtraReply:   payload.ExtraReply,
		Timestamp:    wire.Timestamp,
		Nonce:        wire.Nonce,
		Ciphertext:   wire.Ciphertext,
		HMAC:         wire.HMAC,
	}
	return pkt, nil
}

func WriteBridgeMessage(c *conn.Conn, flag string, payload any) error {
	if c == nil {
		return fmt.Errorf("nil bridge conn")
	}
	_, err := c.SendInfo(payload, flag)
	return err
}

func WritePunchProgress(c *conn.Conn, progress P2PPunchProgress) error {
	if progress.Timestamp == 0 {
		progress.Timestamp = time.Now().UnixMilli()
	}
	return WriteBridgeMessage(c, common.P2P_PUNCH_PROGRESS, progress)
}

func ReadBridgeJSON[T any](c *conn.Conn, expectedFlag string) (T, error) {
	var zero T
	if c == nil {
		return zero, fmt.Errorf("nil bridge conn")
	}
	flag, err := c.ReadFlag()
	if err != nil {
		return zero, err
	}
	raw, err := c.GetShortLenContent()
	if err != nil {
		return zero, err
	}
	if flag == common.P2P_PUNCH_ABORT {
		var abort P2PPunchAbort
		if err := json.Unmarshal(raw, &abort); err != nil {
			return zero, ErrP2PSessionAbort
		}
		if abort.Reason == "" {
			return zero, ErrP2PSessionAbort
		}
		return zero, fmt.Errorf("%w: %s", ErrP2PSessionAbort, abort.Reason)
	}
	if expectedFlag != "" && flag != expectedFlag {
		return zero, fmt.Errorf("unexpected flag %q", flag)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, err
	}
	return out, nil
}

func DefaultTimeouts() P2PTimeouts {
	return P2PTimeouts{
		ProbeTimeoutMs:     5000,
		HandshakeTimeoutMs: 20000,
		TransportTimeoutMs: 10000,
	}
}

func NewProbeAckPacket(sessionID, token, role string, probePort int, observedAddr string, extraReply bool) *UDPPacket {
	packetType := packetTypeAck
	if extraReply {
		packetType = packetTypeProbeX
	}
	packet := newUDPPacket(sessionID, token, role, packetType)
	packet.ProbePort = probePort
	packet.ObservedAddr = observedAddr
	packet.ExtraReply = extraReply
	return packet
}

func newNonce(size int) string {
	if size <= 0 {
		size = 12
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func encodeUDPPacketAAD(version int, sessionID, token string, timestamp int64, nonce string) ([]byte, error) {
	return json.Marshal(udpPacketAAD{
		Version:   version,
		SessionID: sessionID,
		Token:     token,
		Timestamp: timestamp,
		Nonce:     nonce,
	})
}

func sealUDPPayload(pkt *UDPPacket, payload []byte) ([]byte, error) {
	aead, nonce, aad, err := udpPacketCipher(pkt.Version, pkt.SessionID, pkt.Token, pkt.Timestamp, pkt.Nonce)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, payload, aad), nil
}

func openUDPPayload(pkt UDPPacket, sealed []byte) ([]byte, error) {
	aead, nonce, aad, err := udpPacketCipher(pkt.Version, pkt.SessionID, pkt.Token, pkt.Timestamp, pkt.Nonce)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, sealed, aad)
}

func udpPacketCipher(version int, sessionID, token string, timestamp int64, nonce string) (cipher.AEAD, []byte, []byte, error) {
	key := sha256.Sum256([]byte("nps-p2p-aead|" + sessionID + "|" + token))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}
	aad, err := encodeUDPPacketAAD(version, sessionID, token, timestamp, nonce)
	if err != nil {
		return nil, nil, nil, err
	}
	nonceSeed := sha256.Sum256([]byte("nps-p2p-nonce|" + nonce))
	return aead, nonceSeed[:aead.NonceSize()], aad, nil
}

func NormalizeProbeEndpoint(probe P2PProbeConfig, endpoint P2PProbeEndpoint) P2PProbeEndpoint {
	if endpoint.Provider == "" {
		endpoint.Provider = probe.Provider
	}
	if endpoint.Mode == "" {
		endpoint.Mode = probe.Mode
	}
	if endpoint.Network == "" {
		endpoint.Network = probe.Network
	}
	return endpoint
}

func NormalizeProbeEndpoints(probe P2PProbeConfig) []P2PProbeEndpoint {
	endpoints := make([]P2PProbeEndpoint, 0, len(probe.Endpoints))
	for _, endpoint := range probe.Endpoints {
		normalized := NormalizeProbeEndpoint(probe, endpoint)
		if normalized.Address == "" {
			continue
		}
		endpoints = append(endpoints, normalized)
	}
	return endpoints
}

func MergeProbeSamples(serverSamples, clientSamples []ProbeSample) []ProbeSample {
	merged := make(map[string]ProbeSample, len(serverSamples)+len(clientSamples))
	order := make([]string, 0, len(serverSamples)+len(clientSamples))
	merge := func(sample ProbeSample, preferReply bool) {
		normalizeProbeSample(&sample)
		key := probeSampleMergeKey(sample)
		existing, ok := merged[key]
		if !ok {
			merged[key] = sample
			order = append(order, key)
			return
		}
		if existing.EndpointID == "" {
			existing.EndpointID = sample.EndpointID
		}
		if existing.Provider == "" {
			existing.Provider = sample.Provider
		}
		if existing.Mode == "" {
			existing.Mode = sample.Mode
		}
		if existing.ProbePort == 0 {
			existing.ProbePort = sample.ProbePort
		}
		if existing.ObservedAddr == "" {
			existing.ObservedAddr = sample.ObservedAddr
		}
		if preferReply {
			if sample.ServerReplyAddr != "" {
				existing.ServerReplyAddr = sample.ServerReplyAddr
			}
		} else if existing.ServerReplyAddr == "" {
			existing.ServerReplyAddr = sample.ServerReplyAddr
		}
		existing.ExtraReply = existing.ExtraReply || sample.ExtraReply
		merged[key] = existing
	}
	for _, sample := range serverSamples {
		merge(sample, false)
	}
	for _, sample := range clientSamples {
		merge(sample, true)
	}
	out := make([]ProbeSample, 0, len(order))
	for _, key := range order {
		out = append(out, merged[key])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		if out[i].ProbePort != out[j].ProbePort {
			return out[i].ProbePort < out[j].ProbePort
		}
		if out[i].EndpointID != out[j].EndpointID {
			return out[i].EndpointID < out[j].EndpointID
		}
		if out[i].ObservedAddr != out[j].ObservedAddr {
			return out[i].ObservedAddr < out[j].ObservedAddr
		}
		return out[i].ServerReplyAddr < out[j].ServerReplyAddr
	})
	return out
}

func probeSampleMergeKey(sample ProbeSample) string {
	if sample.Provider == ProbeProviderNPS && sample.ProbePort > 0 {
		return sample.Provider + "|" + sample.Mode + "|" + strconv.Itoa(sample.ProbePort)
	}
	if sample.EndpointID != "" {
		return sample.Provider + "|" + sample.Mode + "|" + sample.EndpointID
	}
	return sample.Provider + "|" + sample.Mode + "|" + strconv.Itoa(sample.ProbePort) + "|" + sample.ObservedAddr + "|" + sample.ServerReplyAddr
}

func normalizeProbeSample(sample *ProbeSample) {
	if sample == nil {
		return
	}
	if sample.Provider == "" {
		sample.Provider = ProbeProviderNPS
	}
	if sample.Mode == "" {
		if sample.Provider == ProbeProviderSTUN {
			sample.Mode = ProbeModeBinding
		} else {
			sample.Mode = ProbeModeUDP
		}
	}
}
