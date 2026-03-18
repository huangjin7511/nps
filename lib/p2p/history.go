package p2p

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/djylb/nps/lib/common"
)

const (
	predictionHistoryTTL        = 30 * time.Minute
	predictionHistoryMaxEntries = 256
	predictionHistoryMaxOffsets = 6
)

type predictionHistoryEntry struct {
	updatedAt time.Time
	offsets   map[int]int
}

type predictionHistoryStore struct {
	mu      sync.Mutex
	entries map[string]*predictionHistoryEntry
}

var globalPredictionHistory = &predictionHistoryStore{
	entries: make(map[string]*predictionHistoryEntry),
}

func recordPredictionSuccess(obs NatObservation, remoteAddr string) {
	basePort := obs.ObservedBasePort
	remotePort := common.GetPortByAddr(remoteAddr)
	if basePort <= 0 || remotePort <= 0 {
		return
	}
	key := predictionHistoryKey(obs)
	if key == "" {
		return
	}
	globalPredictionHistory.record(key, remotePort-basePort)
}

func predictionHistoryOffsets(obs NatObservation) []int {
	return globalPredictionHistory.offsets(predictionHistoryKey(obs))
}

func predictionHistoryKey(obs NatObservation) string {
	if obs.PublicIP == "" {
		return ""
	}
	return obs.PublicIP + "|" + obs.NATType + "|" + obs.MappingBehavior + "|" + strconv.Itoa(obs.ObservedInterval)
}

func (s *predictionHistoryStore) record(key string, offset int) {
	if key == "" {
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	entry, ok := s.entries[key]
	if !ok {
		entry = &predictionHistoryEntry{offsets: make(map[int]int)}
		s.entries[key] = entry
	}
	entry.updatedAt = now
	entry.offsets[offset]++
}

func (s *predictionHistoryStore) offsets(key string) []int {
	if key == "" {
		return nil
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	entry, ok := s.entries[key]
	if !ok || entry == nil || len(entry.offsets) == 0 {
		return nil
	}
	type scoredOffset struct {
		offset int
		count  int
	}
	items := make([]scoredOffset, 0, len(entry.offsets))
	for offset, count := range entry.offsets {
		items = append(items, scoredOffset{offset: offset, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		if absInt(items[i].offset) != absInt(items[j].offset) {
			return absInt(items[i].offset) < absInt(items[j].offset)
		}
		return items[i].offset < items[j].offset
	})
	limit := predictionHistoryMaxOffsets
	if len(items) < limit {
		limit = len(items)
	}
	out := make([]int, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, items[i].offset)
	}
	return out
}

func (s *predictionHistoryStore) pruneLocked(now time.Time) {
	for key, entry := range s.entries {
		if entry == nil || now.Sub(entry.updatedAt) > predictionHistoryTTL {
			delete(s.entries, key)
		}
	}
	if len(s.entries) <= predictionHistoryMaxEntries {
		return
	}
	type entryAge struct {
		key       string
		updatedAt time.Time
	}
	ages := make([]entryAge, 0, len(s.entries))
	for key, entry := range s.entries {
		ages = append(ages, entryAge{key: key, updatedAt: entry.updatedAt})
	}
	sort.Slice(ages, func(i, j int) bool { return ages[i].updatedAt.Before(ages[j].updatedAt) })
	for len(s.entries) > predictionHistoryMaxEntries && len(ages) > 0 {
		delete(s.entries, ages[0].key)
		ages = ages[1:]
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
