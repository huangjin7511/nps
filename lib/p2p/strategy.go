package p2p

import (
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/djylb/nps/lib/common"
)

const (
	DefaultPredictionInterval = 1
	DefaultSpraySpan          = 64
	DefaultSprayRounds        = 2
	DefaultSprayBurst         = 8
	DefaultBirthdayPorts      = 8
	DefaultBirthdayTargets    = 16
)

type PunchPlan struct {
	EnablePrediction           bool
	EnableAggressivePrediction bool
	UseSingleLocalSocket       bool
	UseTargetSpray             bool
	UseBirthdayAttack          bool
	AllowBirthdayFallback      bool
	PredictionInterval         int
	PredictionIntervals        []int
	HandshakeTimeout           time.Duration
	SpraySpan                  int
	SprayRounds                int
	SprayBurst                 int
	SprayPerPacketSleep        time.Duration
	SprayBurstGap              time.Duration
	SprayPhaseGap              time.Duration
	BirthdayListenPorts        int
	BirthdayTargetsPerPort     int
}

func SelectPunchPlan(self, peer P2PPeerInfo, timeouts P2PTimeouts) PunchPlan {
	enablePrediction := self.Nat.ProbePortRestricted || peer.Nat.ProbePortRestricted ||
		self.Nat.MappingConfidenceLow || peer.Nat.MappingConfidenceLow ||
		self.Nat.NATType == NATTypeSymmetric || peer.Nat.NATType == NATTypeSymmetric
	allowBirthdayFallback := shouldAllowBirthdayFallback(self, peer)
	intervals := BuildPredictionIntervals(peer.Nat, enablePrediction, enablePrediction)
	interval := 0
	if len(intervals) > 0 {
		interval = intervals[0]
	}
	handshakeTimeout := time.Duration(timeouts.HandshakeTimeoutMs) * time.Millisecond
	if handshakeTimeout <= 0 {
		handshakeTimeout = 20 * time.Second
	}
	return PunchPlan{
		EnablePrediction:           enablePrediction,
		EnableAggressivePrediction: enablePrediction,
		UseSingleLocalSocket:       true,
		UseTargetSpray:             enablePrediction || len(peer.Nat.Samples) == 0,
		UseBirthdayAttack:          false,
		AllowBirthdayFallback:      allowBirthdayFallback,
		PredictionInterval:         interval,
		PredictionIntervals:        intervals,
		HandshakeTimeout:           handshakeTimeout,
		SpraySpan:                  DefaultSpraySpan,
		SprayRounds:                DefaultSprayRounds,
		SprayBurst:                 DefaultSprayBurst,
		SprayPerPacketSleep:        3 * time.Millisecond,
		SprayBurstGap:              10 * time.Millisecond,
		SprayPhaseGap:              40 * time.Millisecond,
		BirthdayListenPorts:        DefaultBirthdayPorts,
		BirthdayTargetsPerPort:     DefaultBirthdayTargets,
	}
}

func ApplyProbePlanOptions(plan PunchPlan, probe P2PProbeConfig, self, peer P2PPeerInfo) PunchPlan {
	if value, ok := parseProbeBoolOption(probe.Options, "force_predict_on_restricted"); ok && !value {
		if !self.Nat.MappingConfidenceLow && !peer.Nat.MappingConfidenceLow && (self.Nat.ProbePortRestricted || peer.Nat.ProbePortRestricted) {
			plan.EnablePrediction = false
			plan.EnableAggressivePrediction = false
			plan.PredictionInterval = 0
			if len(peer.Nat.Samples) > 0 {
				plan.UseTargetSpray = false
			}
		}
	}
	if value, ok := parseProbeBoolOption(probe.Options, "enable_target_spray"); ok {
		plan.UseTargetSpray = value
	}
	if value, ok := parseProbeIntOption(probe.Options, "default_prediction_interval"); ok && value > 0 && plan.EnablePrediction && plan.PredictionInterval == DefaultPredictionInterval {
		plan.PredictionInterval = value
		if len(plan.PredictionIntervals) == 0 {
			plan.PredictionIntervals = []int{value}
		} else {
			plan.PredictionIntervals[0] = value
		}
	}
	if value, ok := parseProbeIntOption(probe.Options, "target_spray_span"); ok && value > 0 {
		plan.SpraySpan = value
	}
	if value, ok := parseProbeIntOption(probe.Options, "target_spray_rounds"); ok && value > 0 {
		plan.SprayRounds = value
	}
	if value, ok := parseProbeIntOption(probe.Options, "target_spray_burst"); ok && value > 0 {
		plan.SprayBurst = value
	}
	if value, ok := parseProbeIntOption(probe.Options, "target_spray_packet_sleep_ms"); ok && value >= 0 {
		plan.SprayPerPacketSleep = time.Duration(value) * time.Millisecond
	}
	if value, ok := parseProbeIntOption(probe.Options, "target_spray_burst_gap_ms"); ok && value >= 0 {
		plan.SprayBurstGap = time.Duration(value) * time.Millisecond
	}
	if value, ok := parseProbeIntOption(probe.Options, "target_spray_phase_gap_ms"); ok && value >= 0 {
		plan.SprayPhaseGap = time.Duration(value) * time.Millisecond
	}
	if value, ok := parseProbeIntOption(probe.Options, "birthday_listen_ports"); ok && value > 0 {
		plan.BirthdayListenPorts = value
	}
	if value, ok := parseProbeIntOption(probe.Options, "birthday_targets_per_port"); ok && value > 0 {
		plan.BirthdayTargetsPerPort = value
	}
	if value, ok := parseProbeBoolOption(probe.Options, "enable_birthday_attack"); !ok || !value {
		plan.AllowBirthdayFallback = false
		plan.UseBirthdayAttack = false
	}
	return plan
}

func BuildPredictionIntervals(obs NatObservation, enablePrediction, aggressive bool) []int {
	if !enablePrediction {
		return nil
	}
	candidates := make([]int, 0, 8)
	seen := make(map[int]struct{}, 8)
	add := func(interval int) {
		if interval < 0 {
			interval = -interval
		}
		if interval <= 0 {
			return
		}
		if interval > 512 {
			return
		}
		if _, ok := seen[interval]; ok {
			return
		}
		seen[interval] = struct{}{}
		candidates = append(candidates, interval)
	}

	add(obs.ObservedInterval)
	diffs := samplePortDiffs(obs.Samples)
	add(minPositive(diffs))
	add(medianPositive(diffs))
	add(gcdPositive(diffs))
	if aggressive {
		base := append([]int(nil), candidates...)
		for _, interval := range base {
			add(interval - 1)
			add(interval + 1)
		}
	}
	add(DefaultPredictionInterval)
	if len(candidates) > 6 {
		candidates = candidates[:6]
	}
	return candidates
}

func shouldAllowBirthdayFallback(self, peer P2PPeerInfo) bool {
	if self.Nat.NATType == NATTypeSymmetric && peer.Nat.NATType == NATTypeSymmetric {
		return true
	}
	return isHardOrUnknownNat(self.Nat) && isHardOrUnknownNat(peer.Nat)
}

func isHardOrUnknownNat(obs NatObservation) bool {
	return obs.ProbePortRestricted || obs.MappingConfidenceLow
}

func parseProbeBoolOption(options map[string]string, key string) (bool, bool) {
	if len(options) == 0 {
		return false, false
	}
	raw, ok := options[key]
	if !ok {
		return false, false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}
	return value, true
}

func parseProbeIntOption(options map[string]string, key string) (int, bool) {
	if len(options) == 0 {
		return 0, false
	}
	raw, ok := options[key]
	if !ok {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func BuildNatObservation(samples []ProbeSample, extraReplySeen bool) NatObservation {
	obs := NatObservation{
		ProbePortRestricted: !extraReplySeen,
		MappingBehavior:     NATMappingUnknown,
		FilteringBehavior:   NATFilteringUnknown,
		NATType:             NATTypeUnknown,
		ClassificationLevel: ClassificationConfidenceLow,
		Samples:             append([]ProbeSample(nil), samples...),
	}
	if len(samples) == 0 {
		obs.MappingConfidenceLow = true
		return obs
	}
	sorted := append([]ProbeSample(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ProbePort < sorted[j].ProbePort })
	first := sorted[0]
	obs.PublicIP = common.GetIpByAddr(first.ObservedAddr)
	ports := make([]int, 0, len(sorted))
	probeIPs := make(map[string]struct{}, len(sorted))
	publicIPs := make(map[string]struct{}, len(sorted))
	uniquePorts := make(map[int]struct{}, len(sorted))
	var hasNPS bool
	for _, sample := range sorted {
		if publicIP := common.GetIpByAddr(sample.ObservedAddr); publicIP != "" {
			publicIPs[publicIP] = struct{}{}
		}
		if replyIP := common.GetIpByAddr(sample.ServerReplyAddr); replyIP != "" {
			probeIPs[replyIP] = struct{}{}
		}
		if sample.Provider == ProbeProviderNPS || sample.Provider == "" {
			hasNPS = true
		}
		if p := common.GetPortByAddr(sample.ObservedAddr); p > 0 {
			ports = append(ports, p)
			uniquePorts[p] = struct{}{}
		}
	}
	obs.ObservedBasePort = minPositive(ports)
	obs.ProbeIPCount = len(probeIPs)
	if obs.ProbeIPCount == 0 {
		obs.ProbeIPCount = 1
	}
	obs.ObservedInterval = computeObservedInterval(ports)
	obs.ConflictingSignals = len(publicIPs) > 1 || (obs.ProbePortRestricted && obs.ObservedInterval > 0)
	mappingEvidenceStrong := obs.ProbeIPCount > 1
	switch {
	case mappingEvidenceStrong && len(uniquePorts) == 1:
		obs.MappingBehavior = NATMappingEndpointIndependent
	case mappingEvidenceStrong && len(uniquePorts) > 1:
		obs.MappingBehavior = NATMappingEndpointDependent
	}
	if hasNPS {
		if obs.ProbePortRestricted {
			obs.FilteringBehavior = NATFilteringPortRestricted
		} else {
			obs.FilteringBehavior = NATFilteringOpen
		}
	}
	switch {
	case obs.MappingBehavior == NATMappingEndpointDependent:
		obs.NATType = NATTypeSymmetric
		obs.ClassificationLevel = ClassificationConfidenceHigh
	case obs.MappingBehavior == NATMappingEndpointIndependent && obs.FilteringBehavior == NATFilteringPortRestricted:
		obs.NATType = NATTypePortRestricted
		obs.ClassificationLevel = ClassificationConfidenceMed
	case obs.MappingBehavior == NATMappingEndpointIndependent && obs.FilteringBehavior == NATFilteringOpen:
		obs.NATType = NATTypeCone
		obs.ClassificationLevel = ClassificationConfidenceMed
	}
	obs.MappingConfidenceLow = len(ports) < 3 || obs.ProbeIPCount <= 1 || (obs.ObservedInterval == 0 && obs.MappingBehavior == NATMappingUnknown) || obs.ConflictingSignals
	if !obs.ConflictingSignals && obs.NATType == NATTypeSymmetric && obs.ProbeIPCount > 1 && len(uniquePorts) > 1 {
		obs.MappingConfidenceLow = false
	}
	if !obs.ConflictingSignals && (obs.NATType == NATTypePortRestricted || obs.NATType == NATTypeCone) {
		if obs.MappingBehavior == NATMappingEndpointIndependent && obs.ProbeIPCount > 1 {
			obs.MappingConfidenceLow = false
		}
	}
	if obs.MappingConfidenceLow {
		obs.ClassificationLevel = ClassificationConfidenceLow
		if obs.NATType == NATTypeSymmetric && mappingEvidenceStrong {
			obs.ClassificationLevel = ClassificationConfidenceMed
		}
	}
	return obs
}

func BuildPunchTargets(peer P2PPeerInfo, plan PunchPlan) []string {
	targets := BuildDirectPunchTargets(peer)
	seen := make(map[string]struct{}, len(targets)+plan.SpraySpan)
	for _, target := range targets {
		seen[target] = struct{}{}
	}
	add := func(addr string) {
		addr = common.ValidateAddr(addr)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		targets = append(targets, addr)
	}
	if peer.Nat.PublicIP != "" && peer.Nat.ObservedBasePort > 0 {
		for _, port := range buildHistoricalPredictionPorts(peer.Nat, plan.SpraySpan) {
			add(net.JoinHostPort(peer.Nat.PublicIP, strconv.Itoa(port)))
		}
		for _, port := range BuildTargetSprayPortsMulti(peer.Nat.ObservedBasePort, plan.predictionIntervals(), plan.SpraySpan) {
			add(net.JoinHostPort(peer.Nat.PublicIP, strconv.Itoa(port)))
		}
	}
	return targets
}

func BuildPreferredPunchTargets(self, peer P2PPeerInfo, plan PunchPlan) []string {
	targets := BuildPreferredDirectPunchTargets(self, peer)
	seen := make(map[string]struct{}, len(targets)+plan.SpraySpan)
	for _, target := range targets {
		seen[target] = struct{}{}
	}
	add := func(addr string) {
		addr = common.ValidateAddr(addr)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		targets = append(targets, addr)
	}
	if peer.Nat.PublicIP != "" && peer.Nat.ObservedBasePort > 0 {
		for _, port := range buildHistoricalPredictionPorts(peer.Nat, plan.SpraySpan) {
			add(net.JoinHostPort(peer.Nat.PublicIP, strconv.Itoa(port)))
		}
		for _, port := range BuildTargetSprayPortsMulti(peer.Nat.ObservedBasePort, plan.predictionIntervals(), plan.SpraySpan) {
			add(net.JoinHostPort(peer.Nat.PublicIP, strconv.Itoa(port)))
		}
	}
	return targets
}

func BuildDirectPunchTargets(peer P2PPeerInfo) []string {
	targets := make([]string, 0, len(peer.Nat.Samples)+len(peer.LocalAddrs))
	seen := make(map[string]struct{})
	add := func(addr string) {
		addr = common.ValidateAddr(addr)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		targets = append(targets, addr)
	}
	if peer.Nat.PortMapping != nil {
		add(peer.Nat.PortMapping.ExternalAddr)
	}
	for _, sample := range peer.Nat.Samples {
		add(sample.ObservedAddr)
	}
	wantIPv6 := peer.Nat.PublicIP != "" && net.ParseIP(peer.Nat.PublicIP) != nil && net.ParseIP(peer.Nat.PublicIP).To4() == nil
	for _, localAddr := range peer.LocalAddrs {
		host := common.GetIpByAddr(localAddr)
		ip := net.ParseIP(host)
		if ip != nil {
			isIPv6 := ip.To4() == nil
			if peer.Nat.PublicIP != "" && isIPv6 != wantIPv6 {
				continue
			}
		}
		add(localAddr)
	}
	return targets
}

func BuildPreferredDirectPunchTargets(self, peer P2PPeerInfo) []string {
	targets := make([]string, 0, len(peer.Nat.Samples)+len(peer.LocalAddrs)+1)
	seen := make(map[string]struct{})
	add := func(addr string) {
		addr = common.ValidateAddr(addr)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		targets = append(targets, addr)
	}
	supportV4, supportV6 := supportedLocalFamilies(self.LocalAddrs)
	for _, localAddr := range peer.LocalAddrs {
		if !supportsPeerLocalFamily(localAddr, supportV4, supportV6) || !isLikelyDirectLocalAddr(localAddr) {
			continue
		}
		add(localAddr)
	}
	if peer.Nat.PortMapping != nil {
		add(peer.Nat.PortMapping.ExternalAddr)
	}
	for _, sample := range peer.Nat.Samples {
		add(sample.ObservedAddr)
	}
	for _, localAddr := range peer.LocalAddrs {
		if !supportsPeerLocalFamily(localAddr, supportV4, supportV6) {
			continue
		}
		add(localAddr)
	}
	return targets
}

func BuildBirthdayPunchTargets(peer P2PPeerInfo, plan PunchPlan) []string {
	targets := make([]string, 0, len(peer.Nat.Samples)+plan.BirthdayTargetsPerPort)
	seen := make(map[string]struct{})
	add := func(addr string) {
		addr = common.ValidateAddr(addr)
		if addr == "" {
			return
		}
		if _, ok := seen[addr]; ok {
			return
		}
		seen[addr] = struct{}{}
		targets = append(targets, addr)
	}
	for _, sample := range peer.Nat.Samples {
		add(sample.ObservedAddr)
	}
	if peer.Nat.PublicIP != "" && peer.Nat.ObservedBasePort > 0 {
		for _, port := range buildHistoricalPredictionPorts(peer.Nat, plan.BirthdayTargetsPerPort) {
			add(net.JoinHostPort(peer.Nat.PublicIP, strconv.Itoa(port)))
		}
		for _, port := range BuildTargetSprayPortsMulti(peer.Nat.ObservedBasePort, plan.predictionIntervals(), plan.BirthdayTargetsPerPort) {
			add(net.JoinHostPort(peer.Nat.PublicIP, strconv.Itoa(port)))
		}
	}
	return targets
}

func supportedLocalFamilies(localAddrs []string) (bool, bool) {
	if len(localAddrs) == 0 {
		return true, true
	}
	var supportV4, supportV6 bool
	for _, addr := range localAddrs {
		ip := common.ParseIPFromAddr(addr)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			supportV4 = true
		} else {
			supportV6 = true
		}
	}
	if !supportV4 && !supportV6 {
		return true, true
	}
	return supportV4, supportV6
}

func supportsPeerLocalFamily(addr string, supportV4, supportV6 bool) bool {
	ip := common.ParseIPFromAddr(addr)
	if ip == nil {
		return false
	}
	if ip.To4() != nil {
		return supportV4
	}
	return supportV6
}

func isLikelyDirectLocalAddr(addr string) bool {
	ip := common.ParseIPFromAddr(addr)
	if ip == nil {
		return false
	}
	ip = common.NormalizeIP(ip)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast() {
		return false
	}
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}
	return !common.IsPublicIPStrict(ip)
}

func (p PunchPlan) predictionIntervals() []int {
	if len(p.PredictionIntervals) != 0 {
		return p.PredictionIntervals
	}
	if p.PredictionInterval > 0 {
		return []int{p.PredictionInterval}
	}
	return []int{DefaultPredictionInterval}
}

func BuildTargetSprayPortsMulti(basePort int, intervals []int, span int) []int {
	if span <= 0 || basePort <= 0 {
		return nil
	}
	if len(intervals) == 0 {
		intervals = []int{DefaultPredictionInterval}
	}
	ports := make([]int, 0, span)
	seen := make(map[int]struct{}, span)
	appendPort := func(port int) {
		if len(ports) >= span {
			return
		}
		if port < 1 || port > 65535 {
			return
		}
		if _, ok := seen[port]; ok {
			return
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	appendPort(basePort)
	for step := 1; len(ports) < span/2; step++ {
		for _, interval := range intervals {
			if interval <= 0 {
				continue
			}
			appendPort(basePort + step*interval)
			appendPort(basePort - step*interval)
			if len(ports) >= span/2 {
				break
			}
		}
	}
	for delta := 1; len(ports) < span; delta++ {
		appendPort(basePort - delta)
		appendPort(basePort + delta)
	}
	return ports
}

func buildHistoricalPredictionPorts(obs NatObservation, span int) []int {
	if span <= 0 || obs.ObservedBasePort <= 0 {
		return nil
	}
	offsets := predictionHistoryOffsets(obs)
	if len(offsets) == 0 {
		return nil
	}
	out := make([]int, 0, len(offsets))
	seen := make(map[int]struct{}, len(offsets))
	for _, offset := range offsets {
		port := obs.ObservedBasePort + offset
		if port < 1 || port > 65535 {
			continue
		}
		if _, ok := seen[port]; ok {
			continue
		}
		seen[port] = struct{}{}
		out = append(out, port)
		if len(out) >= span {
			break
		}
	}
	return out
}

func BuildTargetSprayPorts(basePort, interval, span int) []int {
	if span <= 0 {
		return nil
	}
	if basePort <= 0 {
		return nil
	}
	if interval <= 0 {
		interval = DefaultPredictionInterval
	}
	ports := make([]int, 0, span)
	seen := make(map[int]struct{}, span)
	appendPort := func(port int) {
		if len(ports) >= span {
			return
		}
		if port < 1 || port > 65535 {
			return
		}
		if _, ok := seen[port]; ok {
			return
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}
	appendPort(basePort)
	for delta := 1; len(ports) < span/2; delta++ {
		appendPort(basePort + delta*interval)
		appendPort(basePort - delta*interval)
	}
	for delta := 1; len(ports) < span; delta++ {
		appendPort(basePort - delta)
		appendPort(basePort + delta)
	}
	return ports
}

func computeObservedInterval(ports []int) int {
	if len(ports) < 2 {
		return 0
	}
	sorted := append([]int(nil), ports...)
	sort.Ints(sorted)
	diffs := make([]int, 0, len(sorted)-1)
	for i := 1; i < len(sorted); i++ {
		diff := sorted[i] - sorted[i-1]
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			diffs = append(diffs, diff)
		}
	}
	if len(diffs) == 0 {
		return 0
	}
	if dominant := dominantPositive(diffs); dominant > 0 {
		return dominant
	}
	if gcd := gcdPositive(diffs); gcd > 1 {
		return gcd
	}
	return minPositive(diffs)
}

func samplePortDiffs(samples []ProbeSample) []int {
	if len(samples) < 2 {
		return nil
	}
	ports := make([]int, 0, len(samples))
	for _, sample := range samples {
		if port := common.GetPortByAddr(sample.ObservedAddr); port > 0 {
			ports = append(ports, port)
		}
	}
	if len(ports) < 2 {
		return nil
	}
	sort.Ints(ports)
	diffs := make([]int, 0, len(ports)-1)
	for i := 1; i < len(ports); i++ {
		diff := ports[i] - ports[i-1]
		if diff < 0 {
			diff = -diff
		}
		if diff > 0 {
			diffs = append(diffs, diff)
		}
	}
	return diffs
}

func minPositive(values []int) int {
	minValue := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if minValue == 0 || value < minValue {
			minValue = value
		}
	}
	return minValue
}

func medianPositive(values []int) int {
	filtered := make([]int, 0, len(values))
	for _, value := range values {
		if value > 0 {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return 0
	}
	sort.Ints(filtered)
	return filtered[len(filtered)/2]
}

func gcdPositive(values []int) int {
	gcd := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if gcd == 0 {
			gcd = value
			continue
		}
		gcd = gcdPair(gcd, value)
	}
	return gcd
}

func dominantPositive(values []int) int {
	counts := make(map[int]int, len(values))
	bestValue := 0
	bestCount := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		counts[value]++
		if counts[value] > bestCount || (counts[value] == bestCount && (bestValue == 0 || value < bestValue)) {
			bestValue = value
			bestCount = counts[value]
		}
	}
	return bestValue
}

func gcdPair(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
