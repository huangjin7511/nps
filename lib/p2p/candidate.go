package p2p

import (
	"sync"
	"time"
)

type CandidateState string

const (
	CandidateDiscovered CandidateState = "discovered"
	CandidateSucceeded  CandidateState = "succeeded"
	CandidateNominated  CandidateState = "nominated"
	CandidateConfirmed  CandidateState = "confirmed"
	CandidateClosed     CandidateState = "closed"
)

type CandidatePair struct {
	LocalAddr   string         `json:"local_addr"`
	RemoteAddr  string         `json:"remote_addr"`
	State       CandidateState `json:"state"`
	FirstSeenAt time.Time      `json:"first_seen_at"`
	LastSeenAt  time.Time      `json:"last_seen_at"`
	Nominated   bool           `json:"nominated"`
	Confirmed   bool           `json:"confirmed"`
}

type CandidateManager struct {
	mu               sync.Mutex
	candidates       map[string]*CandidatePair
	rendezvousRemote string
	nominatedKey     string
	confirmedKey     string
	candidateRemote  string
	confirmedRemote  string
}

func NewCandidateManager(rendezvousRemote string) *CandidateManager {
	return &CandidateManager{
		candidates:       make(map[string]*CandidatePair),
		rendezvousRemote: rendezvousRemote,
	}
}

func (m *CandidateManager) Observe(localAddr, remoteAddr string) *CandidatePair {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := candidateKey(localAddr, remoteAddr)
	if pair, ok := m.candidates[key]; ok {
		if pair.State == CandidateClosed && m.confirmedKey == "" {
			pair.State = CandidateDiscovered
			pair.Nominated = false
			pair.Confirmed = false
		}
		pair.LastSeenAt = time.Now()
		return pair
	}
	now := time.Now()
	pair := &CandidatePair{
		LocalAddr:   localAddr,
		RemoteAddr:  remoteAddr,
		State:       CandidateDiscovered,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
	m.candidates[key] = pair
	if m.confirmedRemote == "" || m.confirmedRemote == remoteAddr {
		m.candidateRemote = remoteAddr
	}
	return pair
}

func (m *CandidateManager) MarkSucceeded(localAddr, remoteAddr string) *CandidatePair {
	m.mu.Lock()
	defer m.mu.Unlock()
	pair, ok := m.candidates[candidateKey(localAddr, remoteAddr)]
	if !ok {
		return nil
	}
	pair.LastSeenAt = time.Now()
	if pair.State == CandidateClosed {
		if m.confirmedKey == "" {
			pair.State = CandidateSucceeded
			if m.confirmedRemote == "" || m.confirmedRemote == remoteAddr {
				m.candidateRemote = remoteAddr
			}
		}
		return pair
	}
	pair.State = CandidateSucceeded
	if m.confirmedRemote == "" || m.confirmedRemote == remoteAddr {
		m.candidateRemote = remoteAddr
	}
	return pair
}

func (m *CandidateManager) TryNominate(localAddr, remoteAddr string) (*CandidatePair, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := candidateKey(localAddr, remoteAddr)
	pair, ok := m.candidates[key]
	if !ok {
		return nil, false
	}
	pair.LastSeenAt = time.Now()
	if m.nominatedKey != "" {
		return pair, false
	}
	m.nominatedKey = key
	pair.State = CandidateNominated
	pair.Nominated = true
	if m.confirmedRemote == "" || m.confirmedRemote == remoteAddr {
		m.candidateRemote = remoteAddr
	}
	return pair, true
}

func (m *CandidateManager) Confirm(localAddr, remoteAddr string) *CandidatePair {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := candidateKey(localAddr, remoteAddr)
	pair, ok := m.candidates[key]
	if !ok {
		return nil
	}
	if m.nominatedKey == "" || m.nominatedKey != key {
		return nil
	}
	if m.confirmedKey != "" && m.confirmedKey != key {
		return nil
	}
	pair.LastSeenAt = time.Now()
	pair.State = CandidateConfirmed
	pair.Nominated = true
	pair.Confirmed = true
	m.confirmedKey = key
	m.nominatedKey = key
	m.confirmedRemote = remoteAddr
	m.candidateRemote = remoteAddr
	for otherKey, otherPair := range m.candidates {
		if otherKey == key || otherPair == nil || otherPair.Confirmed {
			continue
		}
		otherPair.State = CandidateClosed
	}
	return pair
}

func (m *CandidateManager) PruneStale(maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	pruned := 0
	for _, pair := range m.candidates {
		if pair == nil || pair.Confirmed || pair.Nominated || pair.State == CandidateClosed {
			continue
		}
		lastSeen := pair.LastSeenAt
		if lastSeen.IsZero() {
			lastSeen = pair.FirstSeenAt
		}
		if lastSeen.IsZero() || now.Sub(lastSeen) <= maxAge {
			continue
		}
		pair.State = CandidateClosed
		pair.LastSeenAt = now
		pruned++
	}
	return pruned
}

func (m *CandidateManager) CleanupClosed(maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	for key, pair := range m.candidates {
		if pair == nil {
			delete(m.candidates, key)
			removed++
			continue
		}
		if pair.State != CandidateClosed || pair.Confirmed {
			continue
		}
		lastSeen := pair.LastSeenAt
		if lastSeen.IsZero() {
			lastSeen = pair.FirstSeenAt
		}
		if lastSeen.IsZero() || now.Sub(lastSeen) <= maxAge {
			continue
		}
		delete(m.candidates, key)
		removed++
	}
	return removed
}

func (m *CandidateManager) ConfirmedRemote() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.confirmedRemote
}

func (m *CandidateManager) CandidateRemote() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.candidateRemote
}

func (m *CandidateManager) NominatedPair() *CandidatePair {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nominatedKey == "" {
		return nil
	}
	return m.candidates[m.nominatedKey]
}

func (m *CandidateManager) ConfirmedPair() *CandidatePair {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.confirmedKey == "" {
		return nil
	}
	return m.candidates[m.confirmedKey]
}

func (m *CandidateManager) StateCounts() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()
	counts := map[string]int{
		"total":      0,
		"active":     0,
		"discovered": 0,
		"succeeded":  0,
		"nominated":  0,
		"confirmed":  0,
		"closed":     0,
	}
	for _, pair := range m.candidates {
		if pair == nil {
			continue
		}
		counts["total"]++
		switch pair.State {
		case CandidateDiscovered:
			counts["discovered"]++
			counts["active"]++
		case CandidateSucceeded:
			counts["succeeded"]++
			counts["active"]++
		case CandidateNominated:
			counts["nominated"]++
			counts["active"]++
		case CandidateConfirmed:
			counts["confirmed"]++
			counts["active"]++
		case CandidateClosed:
			counts["closed"]++
		}
	}
	return counts
}

func candidateKey(localAddr, remoteAddr string) string {
	return localAddr + "->" + remoteAddr
}
