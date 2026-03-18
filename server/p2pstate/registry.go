package p2pstate

import (
	"sort"
	"sync"
	"time"

	"github.com/djylb/nps/lib/p2p"
)

type probeSession struct {
	mu           sync.Mutex
	token        string
	expiresAt    time.Time
	observations map[string]map[int]p2p.ProbeSample
	replay       *p2p.ReplayWindow
}

var (
	probeSessions sync.Map
	cleanerOnce   sync.Once
)

func Register(sessionID, token string, ttl time.Duration) {
	if sessionID == "" || token == "" {
		return
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	startCleaner()
	probeSessions.Store(sessionID, &probeSession{
		token:        token,
		expiresAt:    time.Now().Add(ttl),
		observations: make(map[string]map[int]p2p.ProbeSample),
		replay:       p2p.NewReplayWindow(30 * time.Second),
	})
}

func Unregister(sessionID string) {
	if sessionID == "" {
		return
	}
	probeSessions.Delete(sessionID)
}

func ValidateToken(sessionID, token string) bool {
	session := get(sessionID)
	if session == nil {
		return false
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if time.Now().After(session.expiresAt) {
		probeSessions.Delete(sessionID)
		return false
	}
	return session.token == token
}

func LookupToken(sessionID string) (string, bool) {
	session := get(sessionID)
	if session == nil {
		return "", false
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if time.Now().After(session.expiresAt) {
		probeSessions.Delete(sessionID)
		return "", false
	}
	if session.token == "" {
		return "", false
	}
	return session.token, true
}

func RecordObservation(sessionID, role string, sample p2p.ProbeSample) {
	if sessionID == "" || role == "" || sample.ProbePort == 0 || sample.ObservedAddr == "" {
		return
	}
	session := get(sessionID)
	if session == nil {
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if time.Now().After(session.expiresAt) {
		probeSessions.Delete(sessionID)
		return
	}
	if _, ok := session.observations[role]; !ok {
		session.observations[role] = make(map[int]p2p.ProbeSample)
	}
	session.observations[role][sample.ProbePort] = sample
}

func AcceptPacket(sessionID string, timestampMs int64, nonce string) bool {
	session := get(sessionID)
	if session == nil {
		return false
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if time.Now().After(session.expiresAt) {
		probeSessions.Delete(sessionID)
		return false
	}
	if session.replay == nil {
		return true
	}
	return session.replay.Accept(timestampMs, nonce)
}

func GetObservations(sessionID, role string) []p2p.ProbeSample {
	if sessionID == "" || role == "" {
		return nil
	}
	session := get(sessionID)
	if session == nil {
		return nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if time.Now().After(session.expiresAt) {
		probeSessions.Delete(sessionID)
		return nil
	}
	byPort, ok := session.observations[role]
	if !ok {
		return nil
	}
	samples := make([]p2p.ProbeSample, 0, len(byPort))
	for _, sample := range byPort {
		samples = append(samples, sample)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].ProbePort < samples[j].ProbePort })
	return samples
}

func get(sessionID string) *probeSession {
	if sessionID == "" {
		return nil
	}
	value, ok := probeSessions.Load(sessionID)
	if !ok {
		return nil
	}
	session, ok := value.(*probeSession)
	if !ok || session == nil {
		return nil
	}
	return session
}

func startCleaner() {
	cleanerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				now := time.Now()
				probeSessions.Range(func(key, value any) bool {
					session, ok := value.(*probeSession)
					if !ok || session == nil {
						probeSessions.Delete(key)
						return true
					}
					session.mu.Lock()
					expired := now.After(session.expiresAt)
					session.mu.Unlock()
					if expired {
						probeSessions.Delete(key)
					}
					return true
				})
			}
		}()
	})
}
