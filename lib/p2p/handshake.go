package p2p

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/logs"
)

func (s *runtimeSession) readLoopOnConn(ctx context.Context, readConn net.PacketConn) {
	buf := common.BufPoolUdp.Get()
	defer common.BufPoolUdp.Put(buf)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = readConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, addr, err := readConn.ReadFrom(buf)
		_ = readConn.SetReadDeadline(time.Time{})
		if err != nil {
			if conn.IsTimeout(err) || isIgnorableUDPIcmpError(err) {
				continue
			}
			return
		}
		packet, err := DecodeUDPPacket(buf[:n], s.start.Token)
		if err != nil {
			if errors.Is(err, ErrP2PTokenMismatch) {
				s.recordTokenMismatch()
			}
			continue
		}
		if packet.SessionID != s.start.SessionID || packet.Token != s.start.Token {
			s.recordTokenMismatch()
			continue
		}
		if !s.acceptReplay(packet) {
			continue
		}
		s.recordTokenVerified()
		remoteAddr := addr.String()
		localAddr := readConn.LocalAddr().String()
		switch packet.Type {
		case packetTypeProbe, packetTypePunch:
			s.cm.Observe(localAddr, remoteAddr)
			s.updateCandidateRemote(remoteAddr)
			_ = s.writeUDPToConn(readConn, packetTypeSucc, addr)
		case packetTypeSucc:
			logs.Info("[P2P] SUCCESS received local=%s remote=%s", localAddr, remoteAddr)
			s.cm.Observe(localAddr, remoteAddr)
			s.cm.MarkSucceeded(localAddr, remoteAddr)
			s.updateCandidateRemote(remoteAddr)
			logs.Info("[P2P] candidate hit local=%s remote=%s", localAddr, remoteAddr)
			_ = s.sendProgress("success_received", "ok", remoteAddr, localAddr, remoteAddr, nil)
			if s.start.Role == common.WORK_P2P_VISITOR {
				if pair, nominated := s.cm.TryNominate(localAddr, remoteAddr); nominated {
					logs.Info("[P2P] candidate nominated local=%s remote=%s", pair.LocalAddr, pair.RemoteAddr)
					_ = s.writeUDPToConn(readConn, packetTypeEnd, addr)
					logs.Info("[P2P] END sent local=%s remote=%s", localAddr, remoteAddr)
					_ = s.sendProgress("candidate_nominated", "ok", remoteAddr, localAddr, remoteAddr, nil)
					s.startNominationLoop(ctx, readConn, addr)
				}
			} else {
				_ = s.writeUDPToConn(readConn, packetTypeSucc, addr)
			}
		case packetTypeEnd:
			logs.Info("[P2P] END received local=%s remote=%s", localAddr, remoteAddr)
			s.cm.Observe(localAddr, remoteAddr)
			if s.start.Role == common.WORK_P2P_PROVIDER {
				if pair, nominated := s.cm.TryNominate(localAddr, remoteAddr); nominated || (pair != nil && pair.Nominated) {
					if confirmed := s.cm.Confirm(localAddr, remoteAddr); confirmed != nil {
						logs.Info("[P2P] candidate confirmed local=%s remote=%s", localAddr, remoteAddr)
						s.updateConfirmedRemote(remoteAddr)
						_ = s.writeUDPToConn(readConn, packetTypeAccept, addr)
						_ = s.sendProgress("candidate_confirmed", "ok", remoteAddr, localAddr, remoteAddr, nil)
						s.signalConfirmed(readConn, localAddr, remoteAddr)
					}
				}
			}
		case packetTypeAccept:
			logs.Info("[P2P] ACCEPT received local=%s remote=%s", localAddr, remoteAddr)
			s.cm.Observe(localAddr, remoteAddr)
			if s.start.Role == common.WORK_P2P_VISITOR {
				if confirmed := s.cm.Confirm(localAddr, remoteAddr); confirmed != nil {
					logs.Info("[P2P] candidate confirmed local=%s remote=%s", localAddr, remoteAddr)
					s.updateConfirmedRemote(remoteAddr)
					_ = s.sendProgress("accept_received", "ok", remoteAddr, localAddr, remoteAddr, nil)
					s.signalConfirmed(readConn, localAddr, remoteAddr)
				}
			}
		}
	}
}

func (s *runtimeSession) writeUDPTo(packetType string, addr net.Addr) error {
	return s.writeUDPToConn(s.localConn, packetType, addr)
}

func (s *runtimeSession) writeUDPToConn(writeConn net.PacketConn, packetType string, addr net.Addr) error {
	packet := newUDPPacket(s.start.SessionID, s.start.Token, s.start.Role, packetType)
	raw, err := EncodeUDPPacket(packet)
	if err != nil {
		return err
	}
	_, err = writeConn.WriteTo(raw, addr)
	return err
}

func (s *runtimeSession) updateCandidateRemote(remote string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.endpoints.ConfirmedRemote != "" && s.endpoints.ConfirmedRemote != remote {
		return
	}
	if s.endpoints.CandidateRemote != "" && s.endpoints.CandidateRemote != remote {
		logs.Info("[P2P] remote corrected old=%s new=%s", s.endpoints.CandidateRemote, remote)
	}
	s.endpoints.CandidateRemote = remote
}

func (s *runtimeSession) updateConfirmedRemote(remote string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.endpoints.ConfirmedRemote != "" && s.endpoints.ConfirmedRemote != remote {
		return
	}
	s.endpoints.CandidateRemote = remote
	s.endpoints.ConfirmedRemote = remote
}

func (s *runtimeSession) signalConfirmed(readConn net.PacketConn, localAddr, remote string) {
	select {
	case s.confirmed <- confirmedPair{conn: readConn, localAddr: localAddr, remoteAddr: remote}:
	default:
	}
}

func trySendProgress(c *conn.Conn, sessionID, role, stage, detail string) error {
	return trySendProgressWithStatus(c, sessionID, role, stage, "ok", detail, "", "", nil, nil)
}

func trySendProgressWithStatus(c *conn.Conn, sessionID, role, stage, status, detail, localAddr, remoteAddr string, meta map[string]string, counters map[string]int) error {
	if c == nil {
		return nil
	}
	return WritePunchProgress(c, P2PPunchProgress{
		SessionID:  sessionID,
		Role:       role,
		Stage:      stage,
		Status:     status,
		Detail:     detail,
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Meta:       meta,
		Counters:   counters,
	})
}

func (s *runtimeSession) sendProgress(stage, status, detail, localAddr, remoteAddr string, meta map[string]string) error {
	if s == nil {
		return nil
	}
	return trySendProgressWithStatus(s.control, s.start.SessionID, s.start.Role, stage, status, detail, localAddr, remoteAddr, meta, s.progressCounters())
}

func trySendAbort(c *conn.Conn, sessionID, role, reason string) error {
	if c == nil {
		return nil
	}
	return WriteBridgeMessage(c, common.P2P_PUNCH_ABORT, P2PPunchAbort{
		SessionID: sessionID,
		Role:      role,
		Reason:    reason,
	})
}

func (s *runtimeSession) startNominationLoop(ctx context.Context, writeConn net.PacketConn, remoteAddr net.Addr) {
	if writeConn == nil || remoteAddr == nil {
		return
	}
	key := candidateKey(writeConn.LocalAddr().String(), remoteAddr.String())
	s.mu.Lock()
	if _, ok := s.nominate[key]; ok {
		s.mu.Unlock()
		return
	}
	s.nominate[key] = struct{}{}
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.nominate, key)
			s.mu.Unlock()
		}()
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if s.cm.ConfirmedPair() != nil {
					return
				}
				pair := s.cm.NominatedPair()
				if pair == nil || candidateKey(pair.LocalAddr, pair.RemoteAddr) != key {
					return
				}
				_ = s.writeUDPToConn(writeConn, packetTypeEnd, remoteAddr)
			}
		}
	}()
}

func (s *runtimeSession) recordTokenMismatch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.tokenMismatchDropped++
}

func (s *runtimeSession) recordReplayDrop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.replayDropped++
}

func (s *runtimeSession) recordTokenVerified() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stats.tokenVerified {
		return
	}
	s.stats.tokenVerified = true
	logs.Info("[P2P] token verified role=%s session=%s", s.start.Role, s.start.SessionID)
}

func (s *runtimeSession) logTokenStats() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stats.tokenMismatchDropped > 0 {
		logs.Info("[P2P] token mismatch silently dropped role=%s count=%d", s.start.Role, s.stats.tokenMismatchDropped)
	}
	if s.stats.replayDropped > 0 {
		logs.Info("[P2P] replay silently dropped role=%s count=%d", s.start.Role, s.stats.replayDropped)
	}
}

func (s *runtimeSession) progressCounters() map[string]int {
	if s == nil {
		return nil
	}
	counts := s.cm.StateCounts()
	s.mu.Lock()
	defer s.mu.Unlock()
	if counts == nil {
		counts = make(map[string]int)
	}
	counts["token_mismatch_dropped"] = s.stats.tokenMismatchDropped
	counts["replay_dropped"] = s.stats.replayDropped
	if s.stats.tokenVerified {
		counts["token_verified"] = 1
	}
	return counts
}

func effectiveExtraReplySeen(extraReplySeen, expectExtraReply bool) bool {
	return extraReplySeen || !expectExtraReply
}

func (s *runtimeSession) acceptReplay(packet *UDPPacket) bool {
	if packet == nil {
		return false
	}
	if s.replay == nil {
		return true
	}
	if s.replay.Accept(packet.Timestamp, packet.Nonce) {
		return true
	}
	s.recordReplayDrop()
	return false
}
