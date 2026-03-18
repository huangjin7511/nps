package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/config"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/crypt"
	"github.com/djylb/nps/lib/file"
	"github.com/djylb/nps/lib/logs"
	"github.com/djylb/nps/lib/mux"
	"github.com/djylb/nps/lib/p2p"
	"github.com/djylb/nps/server/proxy"
	"github.com/quic-go/quic-go"
)

// ------------------------------
// P2PManager
// ------------------------------

type Closer interface{ Close() error }

type P2PManager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	pCancel      context.CancelFunc
	mu           sync.Mutex
	wg           sync.WaitGroup
	cfg          *config.CommonConfig
	monitor      bool
	udpConn      net.Conn
	muxSession   *mux.Mux
	quicConn     *quic.Conn
	uuid         string
	secretConn   any
	statusOK     bool
	statusCh     chan struct{}
	proxyServers []Closer
	lastActive   time.Time
}

type P2pBridge struct {
	mgr     *P2PManager
	local   *config.LocalServer
	p2p     bool
	secret  bool
	timeout time.Duration
}

func NewP2pBridge(mgr *P2PManager, l *config.LocalServer) *P2pBridge {
	var useP2P, secret bool
	timeout := time.Second * 5
	if l.Type != "secret" && !DisableP2P {
		useP2P = true
		secret = l.Fallback
	} else {
		secret = true
	}
	if secret && useP2P {
		timeout = 3 * time.Second
	}
	return &P2pBridge{
		mgr:     mgr,
		local:   l,
		p2p:     useP2P,
		secret:  secret,
		timeout: timeout,
	}
}

func NewP2PManager(pCtx context.Context, pCancel context.CancelFunc, cfg *config.CommonConfig) *P2PManager {
	ctx, cancel := context.WithCancel(pCtx)
	mgr := &P2PManager{
		ctx:          ctx,
		cancel:       cancel,
		pCancel:      pCancel,
		cfg:          cfg,
		monitor:      false,
		statusCh:     make(chan struct{}, 1),
		proxyServers: make([]Closer, 0),
	}
	go func() {
		<-pCtx.Done()
		mgr.Close()
		if !AutoReconnect {
			os.Exit(1)
		}
	}()
	return mgr
}

func (b *P2pBridge) SendLinkInfo(_ int, link *conn.Link, _ *file.Tunnel) (net.Conn, error) {
	if link == nil {
		return nil, errors.New("link is nil")
	}
	mgr := b.mgr
	var lastErr error
	ctx, cancel := context.WithTimeout(mgr.ctx, 1000*time.Millisecond)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	first := true
	for {
		var tick <-chan time.Time
		if first {
			first = false
			ch := make(chan time.Time, 1)
			ch <- time.Time{}
			tick = ch
		} else {
			tick = ticker.C
		}
		select {
		case <-ctx.Done():
			mgr.mu.Lock()
			mgr.statusOK = false
			mgr.mu.Unlock()
			if lastErr != nil {
				return nil, fmt.Errorf("timeout waiting P2P tunnel; last error: %w", lastErr)
			}
			return nil, errors.New("timeout waiting P2P tunnel")
		case <-tick:
			if b.p2p {
				mgr.mu.Lock()
				qConn := mgr.quicConn
				session := mgr.muxSession
				idle := time.Since(mgr.lastActive)
				mgr.mu.Unlock()
				// ---------- QUIC ----------
				if qConn != nil {
					logs.Trace("using P2P[QUIC] for connection")
					viaQUIC, err := b.sendViaQUIC(link, qConn, idle)
					if err == nil {
						return viaQUIC, nil
					}
					lastErr = err
				}
				// ---------- KCP ----------
				if session != nil {
					logs.Trace("using P2P[KCP] for connection")
					viaKCP, err := b.sendViaKCP(link, session)
					if err == nil {
						return viaKCP, nil
					}
					lastErr = err
				}
			}
			if b.secret {
				if b.p2p {
					logs.Warn("P2P not ready, fallback to secret")
				} else {
					logs.Trace("using Secret for connection")
				}
				viaSecret, err := b.sendViaSecret(link)
				if err == nil {
					return viaSecret, nil
				}
				lastErr = err
			}
		}
	}
}

func (b *P2pBridge) sendViaQUIC(link *conn.Link, qConn *quic.Conn, idle time.Duration) (net.Conn, error) {
	mgr := b.mgr
	if idle > b.timeout {
		logs.Trace("sent ACK before proceeding")
		link.Option.NeedAck = true
	}
	stream, err := qConn.OpenStreamSync(mgr.ctx)
	if err != nil {
		logs.Trace("QUIC OpenStreamSync failed, retrying: %v", err)
		mgr.resetStatus(false)
		return nil, err
	}
	nc := conn.NewQuicStreamConn(stream, qConn)
	sendOK := false
	defer func() {
		if !sendOK {
			_ = nc.Close()
		}
	}()
	if _, err := conn.NewConn(nc).SendInfo(link, ""); err != nil {
		logs.Trace("QUIC SendInfo failed, retrying: %v", err)
		mgr.resetStatus(false)
		return nil, err
	}
	if link.Option.NeedAck {
		if err := conn.ReadACK(nc, b.timeout); err != nil {
			logs.Trace("QUIC ReadACK failed, retrying: %v", err)
			mgr.resetStatus(false)
			return nil, err
		}
		mgr.mu.Lock()
		mgr.lastActive = time.Now()
		mgr.mu.Unlock()
	}
	mgr.resetStatus(true)
	sendOK = true
	return nc, nil
}

func (b *P2pBridge) sendViaKCP(link *conn.Link, session *mux.Mux) (net.Conn, error) {
	mgr := b.mgr
	nowConn, err := session.NewConn()
	if err != nil {
		logs.Trace("KCP NewConn failed, retrying: %v", err)
		mgr.resetStatus(false)
		return nil, err
	}
	link.Option.NeedAck = false
	sendOK := false
	defer func() {
		if !sendOK {
			_ = nowConn.Close()
		}
	}()
	if _, err := conn.NewConn(nowConn).SendInfo(link, ""); err != nil {
		logs.Trace("KCP SendInfo failed, retrying: %v", err)
		mgr.resetStatus(false)
		return nil, err
	}
	mgr.resetStatus(true)
	sendOK = true
	return nowConn, nil
}

func (b *P2pBridge) sendViaSecret(link *conn.Link) (net.Conn, error) {
	mgr := b.mgr
	sc, err := mgr.getSecretConn()
	if err != nil {
		if AutoReconnect {
			logs.Trace("getSecretConn failed, retrying: %v", err)
		} else {
			logs.Trace("getSecretConn failed: %v", err)
			mgr.pCancel()
		}
		return nil, err
	}
	sendOK := false
	defer func() {
		if !sendOK {
			_ = sc.Close()
		}
	}()
	if _, err := sc.Write([]byte(crypt.Md5(b.local.Password))); err != nil {
		logs.Error("secret write password failed: %v", err)
		return nil, err
	}
	if _, err := conn.NewConn(sc).SendInfo(link, ""); err != nil {
		logs.Trace("Secret SendInfo failed, retrying: %v", err)
		return nil, err
	}
	if link.Option.NeedAck {
		if err := conn.ReadACK(sc, b.timeout); err != nil {
			logs.Trace("Secret ReadACK failed, retrying: %v", err)
			return nil, err
		}
	}
	sendOK = true
	return sc, nil
}

func (b *P2pBridge) IsServer() bool {
	return false
}

func (b *P2pBridge) CliProcess(*conn.Conn, string) {
	// no-op
}

func (mgr *P2PManager) StartLocalServer(l *config.LocalServer) error {
	if mgr.ctx.Err() != nil {
		return errors.New("parent context canceled")
	}
	pb := NewP2pBridge(mgr, l)
	if pb.p2p {
		mgr.mu.Lock()
		needStart := !mgr.monitor
		if needStart {
			mgr.monitor = true
		}
		mgr.mu.Unlock()
		if needStart {
			mgr.wg.Add(1)
			go func() {
				defer mgr.wg.Done()
				mgr.handleUdpMonitor(mgr.cfg, l)
			}()
		}
	}

	task := &file.Tunnel{
		Port:     l.Port,
		ServerIp: "0.0.0.0",
		Status:   true,
		Client: &file.Client{
			Cnf: &file.Config{
				U:        "",
				P:        "",
				Compress: mgr.cfg.Client.Cnf.Compress,
			},
			Status:    true,
			IsConnect: true,
			RateLimit: 0,
			Flow:      &file.Flow{},
		},
		HttpProxy:   true,
		Socks5Proxy: true,
		Flow:        &file.Flow{},
		Target: &file.Target{
			TargetStr:  l.Target,
			LocalProxy: l.LocalProxy,
		},
	}

	switch l.Type {
	case "p2ps":
		logs.Info("start http/socks5 monitor port %d", l.Port)
		srv := proxy.NewTunnelModeServer(proxy.ProcessMix, pb, task, true)
		mgr.mu.Lock()
		mgr.proxyServers = append(mgr.proxyServers, srv)
		mgr.mu.Unlock()
		mgr.wg.Add(1)
		go func() {
			defer mgr.wg.Done()
			_ = srv.Start()
		}()
		return nil
	case "p2pt":
		logs.Info("start tcp trans monitor port %d", l.Port)
		srv := proxy.NewTunnelModeServer(proxy.HandleTrans, pb, task, true)
		mgr.mu.Lock()
		mgr.proxyServers = append(mgr.proxyServers, srv)
		mgr.mu.Unlock()
		mgr.wg.Add(1)
		go func() {
			defer mgr.wg.Done()
			_ = srv.Start()
		}()
		return nil
	}

	if l.TargetType == common.CONN_ALL || l.TargetType == common.CONN_TCP {
		logs.Info("local tcp monitoring started on port %d", l.Port)
		srv := proxy.NewTunnelModeServer(proxy.ProcessTunnel, pb, task, true)
		mgr.mu.Lock()
		mgr.proxyServers = append(mgr.proxyServers, srv)
		mgr.mu.Unlock()
		mgr.wg.Add(1)
		go func() {
			defer mgr.wg.Done()
			_ = srv.Start()
		}()
	}
	if l.TargetType == common.CONN_ALL || l.TargetType == common.CONN_UDP {
		logs.Info("local udp monitoring started on port %d", l.Port)
		srv := proxy.NewUdpModeServer(pb, task, true)
		mgr.mu.Lock()
		mgr.proxyServers = append(mgr.proxyServers, srv)
		mgr.mu.Unlock()
		mgr.wg.Add(1)
		go func() {
			defer mgr.wg.Done()
			_ = srv.Start()
		}()
	}

	return nil
}

func (mgr *P2PManager) getSecretConn() (c net.Conn, err error) {
	mgr.mu.Lock()
	secretConn := mgr.secretConn
	mgr.mu.Unlock()
	if secretConn != nil {
		switch tun := secretConn.(type) {
		case *mux.Mux:
			c, err = tun.NewConn()
			if err != nil {
				_ = tun.Close()
			}
		case *quic.Conn:
			var stream *quic.Stream
			stream, err = tun.OpenStreamSync(mgr.ctx)
			if err == nil {
				c = conn.NewQuicStreamConn(stream, tun)
			} else {
				_ = tun.CloseWithError(0, err.Error())
			}
		default:
			err = errors.New("the tunnel type error")
			logs.Error("the tunnel type error")
		}
		if err != nil {
			mgr.mu.Lock()
			mgr.secretConn = nil
			mgr.mu.Unlock()
			secretConn = nil
		}
	}
	if secretConn == nil {
		pc, uuid, err := NewConn(mgr.cfg.Tp, mgr.cfg.VKey, mgr.cfg.Server, mgr.cfg.ProxyUrl, mgr.cfg.LocalIP)
		if err != nil {
			logs.Error("secret NewConn failed: %v", err)
			return nil, err
		}
		mgr.mu.Lock()
		if mgr.uuid == "" {
			mgr.uuid = uuid
		} else {
			uuid = mgr.uuid
		}
		mgr.mu.Unlock()
		if Ver > 5 {
			err = SendType(pc, common.WORK_VISITOR, uuid)
			if err != nil {
				logs.Error("secret SendType failed: %v", err)
				_ = pc.Close()
				return nil, err
			}
			if mgr.cfg.Tp == common.CONN_QUIC {
				qc, ok := pc.Conn.(*conn.QuicAutoCloseConn)
				if !ok {
					logs.Error("failed to get quic session")
					_ = pc.Close()
					return nil, errors.New("failed to get quic session")
				}
				sess := qc.GetSession()
				var stream *quic.Stream
				stream, err := sess.OpenStreamSync(mgr.ctx)
				if err != nil {
					logs.Error("secret OpenStreamSync failed: %v", err)
					_ = pc.Close()
					return nil, err
				}
				c = conn.NewQuicStreamConn(stream, sess)
				secretConn = sess
			} else {
				muxConn := mux.NewMux(pc.Conn, mgr.cfg.Tp, mgr.cfg.DisconnectTime, true)
				c, err = muxConn.NewConn()
				if err != nil {
					logs.Error("secret muxConn failed: %v", err)
					_ = muxConn.Close()
					_ = pc.Close()
					return nil, err
				}
				secretConn = muxConn
			}
			mgr.mu.Lock()
			mgr.secretConn = secretConn
			mgr.mu.Unlock()
		} else {
			c = pc
		}
	}
	if c == nil {
		logs.Error("secret GetConn failed: %v", err)
		return nil, errors.New("secret conn nil")
	}
	mgr.mu.Lock()
	uuid := mgr.uuid
	mgr.mu.Unlock()
	err = SendType(conn.NewConn(c), common.WORK_SECRET, uuid)
	if err != nil {
		logs.Error("secret SendType failed: %v", err)
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (mgr *P2PManager) handleUdpMonitor(cfg *config.CommonConfig, l *config.LocalServer) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	const maxRetry = 300
	const natHardFailLimit = 6
	var addrRetry int
	var notReadyRetry int
	var natHardFailCount int
	var hardNATPunchDisabled bool

	for {
		select {
		case <-mgr.ctx.Done():
			return
		case <-ticker.C:
		case <-mgr.statusCh:
		}

		mgr.mu.Lock()
		ok := mgr.statusOK && (mgr.udpConn != nil || (mgr.quicConn != nil && mgr.quicConn.Context().Err() == nil))
		if ok {
			addrRetry = 0
			notReadyRetry = 0
			natHardFailCount = 0
			hardNATPunchDisabled = false
			mgr.mu.Unlock()
			continue
		}
		if mgr.udpConn != nil {
			_ = mgr.udpConn.Close()
			mgr.udpConn = nil
		}
		if mgr.quicConn != nil {
			if mgr.quicConn.Context().Err() != nil {
				logs.Debug("quic connection context error: %v", mgr.quicConn.Context().Err())
			}
			_ = mgr.quicConn.CloseWithError(0, "monitor close")
			mgr.quicConn = nil
		}
		mgr.mu.Unlock()

		if hardNATPunchDisabled {
			continue
		}

		addrV4, errV4 := common.GetLocalUdp4Addr()
		if errV4 != nil {
			logs.Warn("Failed to get local IPv4 address: %v", errV4)
		} else {
			logs.Debug("IPv4 address: %v", addrV4)
		}

		addrV6, errV6 := common.GetLocalUdp6Addr()
		if errV6 != nil {
			logs.Warn("Failed to get local IPv6 address: %v", errV6)
		} else {
			logs.Debug("IPv6 address: %v", addrV6)
		}

		if errV4 != nil && errV6 != nil {
			addrRetry++
			if addrRetry >= maxRetry {
				logs.Error("Both IPv4 and IPv6 address retrieval failed %d times, exiting.", addrRetry)
				mgr.resetStatus(false)
				return
			}
			logs.Warn("No local IP available yet (retry %d/%d).", addrRetry, maxRetry)
			continue
		}
		addrRetry = 0

		for i := 0; i < 3; i++ {
			logs.Debug("try P2P hole punch %d", i+1)
			select {
			case <-mgr.ctx.Done():
				return
			default:
			}
			if errV4 == nil {
				tryErr := mgr.newUdpConn(addrV4.String(), cfg, l)
				if errors.Is(tryErr, p2p.ErrNATNotSupportP2P) {
					natHardFailCount++
				}
				mgr.mu.Lock()
				if mgr.statusOK {
					mgr.mu.Unlock()
					break
				}
				mgr.mu.Unlock()
				notReadyRetry++
			}
			if errV6 == nil {
				tryErr := mgr.newUdpConn(addrV6.String(), cfg, l)
				if errors.Is(tryErr, p2p.ErrNATNotSupportP2P) {
					natHardFailCount++
				}
				mgr.mu.Lock()
				if mgr.statusOK {
					mgr.mu.Unlock()
					break
				}
				mgr.mu.Unlock()
				notReadyRetry++
			}
			time.Sleep(50 * time.Millisecond)
		}

		mgr.mu.Lock()
		stillBad := !mgr.statusOK
		mgr.mu.Unlock()
		if stillBad {
			if natHardFailCount >= natHardFailLimit {
				if !hardNATPunchDisabled {
					logs.Warn("P2P hard-NAT handshake failed %d times, disable punching and keep relay/secret available.", natHardFailCount)
				}
				hardNATPunchDisabled = true
				continue
			}
			if notReadyRetry >= maxRetry {
				logs.Error("P2P connection not established after %d retries (~%ds), exiting.", notReadyRetry, maxRetry)
				mgr.resetStatus(false)
				return
			}
			logs.Warn("P2P not established yet (retry %d/%d).", notReadyRetry, maxRetry)
		} else {
			notReadyRetry = 0
		}
	}
}

func (mgr *P2PManager) newUdpConn(localAddr string, cfg *config.CommonConfig, l *config.LocalServer) error {
	mgr.mu.Lock()
	secretConn := mgr.secretConn
	mgr.mu.Unlock()
	var err error
	var rawConn net.Conn
	var remoteConn *conn.Conn
	if secretConn != nil {
		switch tun := mgr.secretConn.(type) {
		case *mux.Mux:
			rawConn, err = tun.NewConn()
			if err != nil {
				_ = tun.Close()
			}
		case *quic.Conn:
			var stream *quic.Stream
			stream, err = tun.OpenStreamSync(mgr.ctx)
			if err == nil {
				rawConn = conn.NewQuicStreamConn(stream, tun)
			} else {
				_ = tun.CloseWithError(0, err.Error())
			}
		default:
			err = errors.New("the tunnel type error")
			logs.Error("the tunnel type error")
		}
		if err != nil {
			mgr.mu.Lock()
			mgr.secretConn = nil
			mgr.mu.Unlock()
			secretConn = nil
		}
	}
	if secretConn == nil {
		var uuid string
		remoteConn, uuid, err = NewConn(cfg.Tp, cfg.VKey, cfg.Server, cfg.ProxyUrl, cfg.LocalIP)
		if err != nil {
			logs.Error("Failed to connect to server: %v", err)
			if AutoReconnect {
				time.Sleep(5 * time.Second)
			} else {
				mgr.pCancel()
			}
			return nil
		}
		mgr.mu.Lock()
		if mgr.uuid == "" {
			mgr.uuid = uuid
		}
		mgr.mu.Unlock()
	}
	if remoteConn == nil && rawConn != nil {
		remoteConn = conn.NewConn(rawConn)
	}
	if remoteConn == nil {
		logs.Error("Get conn failed: %v", err)
		return nil
	}
	defer func() { _ = remoteConn.Close() }()
	mgr.mu.Lock()
	uuid := mgr.uuid
	mgr.mu.Unlock()
	err = SendType(remoteConn, common.WORK_P2P, uuid)
	if err != nil {
		logs.Error("Failed to send type to server: %v", err)
		if AutoReconnect {
			time.Sleep(5 * time.Second)
		} else {
			mgr.pCancel()
		}
		return nil
	}
	if _, err := remoteConn.Write([]byte(crypt.Md5(l.Password))); err != nil {
		logs.Error("Failed to send password to server: %v", err)
		if AutoReconnect {
			time.Sleep(5 * time.Second)
		} else {
			mgr.pCancel()
		}
		return nil
	}

	var remoteAddr, sessionID, role, mode, data string
	var transportTimeout time.Duration
	var localConn net.PacketConn
	localConn, remoteAddr, localAddr, sessionID, role, mode, data, transportTimeout, err = p2p.RunVisitorSession(mgr.ctx, remoteConn, localAddr, P2PMode, "", p2p.ParseSTUNServerList(cfg.P2PStunServers))
	if err != nil {
		logs.Error("Run visitor P2P session failed: %v", err)
		return err
	}
	if transportTimeout <= 0 {
		transportTimeout = 10 * time.Second
	}
	if mode == "" {
		mode = common.CONN_KCP
	}
	_ = p2p.WritePunchProgress(remoteConn, p2p.P2PPunchProgress{
		SessionID:  sessionID,
		Role:       role,
		Stage:      "transport_start",
		Status:     "ok",
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Detail:     mode,
		Meta: map[string]string{
			"transport_mode": mode,
		},
	})
	logs.Debug("visitor p2p result local=%s remote=%s role=%s mode=%s data=%s", localAddr, remoteAddr, role, mode, data)

	var udpTunnel net.Conn
	var sess *quic.Conn

	rUDPAddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		logs.Error("Failed to resolve remote UDP addr: %v", err)
		_ = p2p.WritePunchProgress(remoteConn, p2p.P2PPunchProgress{
			SessionID:  sessionID,
			Role:       role,
			Stage:      "transport_fail_stage",
			Status:     "fail",
			LocalAddr:  localAddr,
			RemoteAddr: remoteAddr,
			Detail:     "resolve_remote",
		})
		_ = localConn.Close()
		return nil
	}

	if mode == common.CONN_QUIC {
		dialCtx, cancelDial := context.WithTimeout(mgr.ctx, transportTimeout)
		sess, err = quic.Dial(dialCtx, localConn, rUDPAddr, TlsCfg, QuicConfig)
		cancelDial()
		if err != nil {
			logs.Error("QUIC dial error: %v", err)
			_ = p2p.WritePunchProgress(remoteConn, p2p.P2PPunchProgress{
				SessionID:  sessionID,
				Role:       role,
				Stage:      "transport_fail_stage",
				Status:     "fail",
				LocalAddr:  localAddr,
				RemoteAddr: remoteAddr,
				Detail:     "quic_dial",
			})
			_ = localConn.Close()
			return nil
		}
		state := sess.ConnectionState().TLS
		if len(state.PeerCertificates) == 0 {
			logs.Error("Failed to get QUIC certificate")
			_ = p2p.WritePunchProgress(remoteConn, p2p.P2PPunchProgress{
				SessionID:  sessionID,
				Role:       role,
				Stage:      "transport_fail_stage",
				Status:     "fail",
				LocalAddr:  localAddr,
				RemoteAddr: remoteAddr,
				Detail:     "quic_cert_missing",
			})
			_ = localConn.Close()
			return nil
		}
		leaf := state.PeerCertificates[0]
		if data != string(crypt.GetHMAC(cfg.VKey, leaf.Raw)) {
			logs.Error("Failed to verify QUIC certificate")
			_ = p2p.WritePunchProgress(remoteConn, p2p.P2PPunchProgress{
				SessionID:  sessionID,
				Role:       role,
				Stage:      "transport_fail_stage",
				Status:     "fail",
				LocalAddr:  localAddr,
				RemoteAddr: remoteAddr,
				Detail:     "quic_cert_verify",
			})
			_ = localConn.Close()
			return nil
		}
	} else {
		kcpTunnel, err := conn.NewKCPSessionWithConn(rUDPAddr, localConn)
		if err != nil {
			logs.Warn("KCP create failed: %v", err)
			_ = p2p.WritePunchProgress(remoteConn, p2p.P2PPunchProgress{
				SessionID:  sessionID,
				Role:       role,
				Stage:      "transport_fail_stage",
				Status:     "fail",
				LocalAddr:  localAddr,
				RemoteAddr: remoteAddr,
				Detail:     "kcp_create",
			})
			_ = localConn.Close()
			return nil
		}
		udpTunnel = kcpTunnel
	}
	_ = p2p.WritePunchProgress(remoteConn, p2p.P2PPunchProgress{
		SessionID:  sessionID,
		Role:       role,
		Stage:      "transport_established",
		Status:     "ok",
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		Detail:     mode,
	})

	logs.Info("P2P UDP[%s] tunnel established to %s, role[%s]", mode, remoteAddr, role)

	mgr.mu.Lock()
	mgr.lastActive = time.Now()
	if mgr.udpConn != nil {
		_ = mgr.udpConn.Close()
	}
	if mgr.muxSession != nil {
		_ = mgr.muxSession.Close()
	}
	if mgr.quicConn != nil {
		_ = mgr.quicConn.CloseWithError(0, "new connection")
	}
	if mode == common.CONN_QUIC {
		mgr.quicConn = sess
		mgr.udpConn = nil
		mgr.muxSession = nil
	} else {
		mgr.udpConn = udpTunnel
		mgr.muxSession = mux.NewMux(udpTunnel, "kcp", cfg.DisconnectTime, false)
	}
	mgr.statusOK = true
	mgr.mu.Unlock()
	return nil
}

func (mgr *P2PManager) resetStatus(ok bool) {
	mgr.mu.Lock()
	oldStatus := mgr.statusOK
	mgr.statusOK = ok
	mgr.mu.Unlock()
	if !ok && oldStatus {
		select {
		case mgr.statusCh <- struct{}{}:
		default:
		}
	}
}

func (mgr *P2PManager) Close() {
	mgr.cancel()
	mgr.mu.Lock()
	psList := mgr.proxyServers
	udp := mgr.udpConn
	muxSess := mgr.muxSession
	qConn := mgr.quicConn
	secretConn := mgr.secretConn
	mgr.mu.Unlock()

	for _, srv := range psList {
		_ = srv.Close()
	}
	if udp != nil {
		_ = udp.Close()
	}
	if muxSess != nil {
		_ = muxSess.Close()
	}
	if qConn != nil {
		_ = qConn.CloseWithError(0, "close quic")
	}
	if secretConn != nil {
		switch tun := secretConn.(type) {
		case *mux.Mux:
			_ = tun.Close()
		case *quic.Conn:
			_ = tun.CloseWithError(0, "p2p close")
		default:
			logs.Error("the tunnel type error")
		}
	}
	mgr.wg.Wait()
}
