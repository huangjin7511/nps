package p2p

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/logs"
)

func (s *runtimeSession) startSpray(ctx context.Context) {
	targets := BuildPreferredDirectPunchTargets(s.summary.Self, s.summary.Peer)
	mode := "direct"
	if s.plan.UseTargetSpray {
		targets = BuildPreferredPunchTargets(s.summary.Self, s.summary.Peer, s.plan)
		mode = "target"
	}
	if len(targets) == 0 {
		return
	}
	_ = s.sendProgress("spray_started", "ok", mode, addrString(s.localConn.LocalAddr()), "", map[string]string{
		"target_count": strconv.Itoa(len(targets)),
		"base_port":    strconv.Itoa(s.summary.Peer.Nat.ObservedBasePort),
		"rounds":       strconv.Itoa(s.plan.SprayRounds),
		"burst":        strconv.Itoa(s.plan.SprayBurst),
		"intervals":    fmt.Sprint(s.plan.predictionIntervals()),
	})
	logs.Info("[P2P] spray mode=%s basePort=%d intervals=%v rounds=%d burst=%d",
		mode, s.summary.Peer.Nat.ObservedBasePort, s.plan.predictionIntervals(), s.plan.SprayRounds, s.plan.SprayBurst)
	s.runSprayRounds(ctx, s.localConn, targets)
	if s.cm.ConfirmedPair() != nil || s.cm.NominatedPair() != nil {
		return
	}
	if s.plan.UseBirthdayAttack || s.plan.AllowBirthdayFallback {
		if !s.plan.UseBirthdayAttack {
			s.enableBirthdayFallback()
			_ = s.sendProgress("birthday_fallback_enabled", "ok", "", addrString(s.localConn.LocalAddr()), "", nil)
			logs.Info("[P2P] spray fallback=target_to_birthday role=%s", s.start.Role)
		}
		s.runBirthdaySpray(ctx)
		return
	}
	s.runPeriodicSpray(ctx, s.snapshotSockets(), targets)
}

func (s *runtimeSession) runBirthdaySpray(ctx context.Context) {
	targets := BuildBirthdayPunchTargets(s.summary.Peer, s.plan)
	if len(targets) == 0 {
		return
	}
	baseBind := buildAdditionalBindAddr(s.localConn.LocalAddr())
	if baseBind == "" {
		s.runPeriodicSpray(ctx, s.snapshotSockets(), targets)
		return
	}
	logs.Info("[P2P] spray mode=birthday listenPorts=%d targetsPerPort=%d",
		s.plan.BirthdayListenPorts, s.plan.BirthdayTargetsPerPort)
	for round := 0; round < s.plan.SprayRounds; round++ {
		desired := 1 << round
		if desired < 1 {
			desired = 1
		}
		if desired > s.plan.BirthdayListenPorts {
			desired = s.plan.BirthdayListenPorts
		}
		activeSockets := s.ensureBirthdaySockets(ctx, baseBind, desired)
		_ = s.sendProgress("birthday_phase", "ok", fmt.Sprintf("phase=%d", round+1), addrString(s.localConn.LocalAddr()), "", map[string]string{
			"listen_ports":     strconv.Itoa(len(activeSockets)),
			"targets_per_port": strconv.Itoa(len(targets)),
		})
		s.runSprayMatrixOnce(ctx, activeSockets, targets)
		if s.cm.ConfirmedPair() != nil || s.cm.NominatedPair() != nil {
			return
		}
		if round < s.plan.SprayRounds-1 {
			time.Sleep(s.plan.SprayPhaseGap)
		}
	}
	s.runPeriodicSpray(ctx, s.ensureBirthdaySockets(ctx, baseBind, s.plan.BirthdayListenPorts), targets)
}

func (s *runtimeSession) runSprayRounds(ctx context.Context, sendConn net.PacketConn, targets []string) {
	for round := 0; round < s.plan.SprayRounds; round++ {
		s.runSprayPass(ctx, sendConn, targets)
		if s.cm.ConfirmedPair() != nil || s.cm.NominatedPair() != nil {
			return
		}
		if round < s.plan.SprayRounds-1 {
			time.Sleep(s.plan.SprayPhaseGap)
		}
	}
}

func (s *runtimeSession) runSprayMatrixOnce(ctx context.Context, sockets []net.PacketConn, targets []string) {
	for _, socket := range sockets {
		if socket == nil {
			continue
		}
		s.runSprayPass(ctx, socket, targets)
		if s.cm.ConfirmedPair() != nil || s.cm.NominatedPair() != nil {
			return
		}
	}
}

func (s *runtimeSession) runPeriodicSpray(ctx context.Context, sockets []net.PacketConn, targets []string) {
	for attempt := 0; ; attempt++ {
		delay := periodicSprayDelay(attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if s.cm.ConfirmedPair() != nil {
				return
			}
			_ = s.sendProgress("spray_retry", "ok", fmt.Sprintf("attempt=%d", attempt+1), addrString(s.localConn.LocalAddr()), s.cm.CandidateRemote(), map[string]string{
				"delay_ms":     strconv.FormatInt(delay.Milliseconds(), 10),
				"socket_count": strconv.Itoa(len(sockets)),
				"target_count": strconv.Itoa(len(targets)),
			})
			s.runSprayMatrixOnce(ctx, sockets, targets)
		}
	}
}

func (s *runtimeSession) runSprayPass(ctx context.Context, sendConn net.PacketConn, targets []string) {
	for i, target := range targets {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if s.cm.ConfirmedPair() != nil || s.cm.NominatedPair() != nil {
			return
		}
		udpAddr, err := net.ResolveUDPAddr("udp", target)
		if err != nil {
			continue
		}
		for burst := 0; burst < s.plan.SprayBurst; burst++ {
			if s.cm.ConfirmedPair() != nil || s.cm.NominatedPair() != nil {
				return
			}
			_ = s.writeUDPToConn(sendConn, packetTypePunch, udpAddr)
			time.Sleep(s.plan.SprayPerPacketSleep)
		}
		if i < len(targets)-1 {
			time.Sleep(s.plan.SprayBurstGap)
		}
	}
}

func buildAdditionalBindAddr(localAddr net.Addr) string {
	if localAddr == nil {
		return ""
	}
	host := common.GetIpByAddr(localAddr.String())
	if host == "" {
		return ""
	}
	return common.BuildAddress(host, "0")
}

func (s *runtimeSession) ensureBirthdaySockets(ctx context.Context, bindAddr string, desired int) []net.PacketConn {
	if desired <= 0 {
		return nil
	}
	sockets := s.snapshotSockets()
	for len(sockets) < desired {
		birthdayConn, err := conn.NewUdpConnByAddr(bindAddr)
		if err != nil {
			break
		}
		s.addSocket(birthdayConn)
		go s.readLoopOnConn(ctx, birthdayConn)
		sockets = append(sockets, birthdayConn)
	}
	if len(sockets) > desired {
		sockets = sockets[:desired]
	}
	return sockets
}

func periodicSprayDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := 1500*time.Millisecond + time.Duration(minInt(attempt, 5))*500*time.Millisecond
	switch attempt % 3 {
	case 1:
		delay += 100 * time.Millisecond
	case 2:
		delay -= 100 * time.Millisecond
	}
	if delay < 500*time.Millisecond {
		delay = 500 * time.Millisecond
	}
	if delay > 4*time.Second {
		delay = 4 * time.Second
	}
	return delay
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
