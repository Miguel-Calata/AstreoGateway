package metrics

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Attempt struct {
	ProviderSlug string `json:"provider_slug"`
	ModelName    string `json:"model_name"`
	KeyID        string `json:"key_id"`
	Status       int    `json:"status"`
	FailClass    string `json:"fail_class"`
	DurationMs   int64  `json:"duration_ms"`
}

type RequestLog struct {
	ID               string    `json:"id"`
	RequestID        string    `json:"request_id"`
	Ts               int64     `json:"ts"`
	GatewayKeyID     string    `json:"gateway_key_id"`
	Method           string    `json:"method"`
	Path             string    `json:"path"`
	Directive        string    `json:"directive"`
	ResolvedProvider string    `json:"resolved_provider_slug"`
	ResolvedModel    string    `json:"resolved_model"`
	AliasName        string    `json:"alias_name"`
	Status           int       `json:"status"`
	Attempts         int       `json:"attempts"`
	DurationMs       int64     `json:"duration_ms"`
	TokensPrompt     int       `json:"tokens_prompt"`
	TokensCompletion int       `json:"tokens_completion"`
	Stream           bool      `json:"stream"`
	ErrorClass       string    `json:"error_class"`
	ClientIP         string    `json:"client_ip"`
	AttemptsDetail   []Attempt `json:"attempts_detail,omitempty"`
}

type ListQuery struct {
	From           int64
	To             int64
	GatewayKeyID   string
	ProviderSlug   string
	StatusClass    string // ok | client_err | server_err | ""
	Directive      string
	Limit          int
	Offset         int
	Order          string // ts_desc | ts_asc | dur_desc
}

type ListResult struct {
	Items     []RequestLog `json:"items"`
	Total     int          `json:"total"`
	OldestTs  int64        `json:"oldest_ts"`
	Capacity  int          `json:"capacity"`
	Size      int          `json:"size"`
	Truncated bool         `json:"truncated"`
}

type StatsQuery struct {
	Window  string // 1h | 24h | 7d
	GroupBy string // hour | day
}

type ProviderStat struct {
	Slug     string `json:"slug"`
	Requests int    `json:"requests"`
	Tokens   int    `json:"tokens"`
	Errors   int    `json:"errors"`
}

type GatewayKeyStat struct {
	ID       string `json:"id"`
	Requests int    `json:"requests"`
	Tokens   int    `json:"tokens"`
}

type StatusClassStat struct {
	Class string `json:"class"`
	Count int    `json:"count"`
}

type TimeBucket struct {
	Ts                int64 `json:"ts"`
	RequestsOK        int   `json:"requests_ok"`
	RequestsClientErr int   `json:"requests_client_err"`
	RequestsServerErr int   `json:"requests_server_err"`
	Tokens            int   `json:"tokens"`
	TokensPrompt      int   `json:"tokens_prompt"`
	TokensCompletion  int   `json:"tokens_completion"`
}

type StatsResult struct {
	Window                string            `json:"window"`
	From                  int64             `json:"from"`
	To                    int64             `json:"to"`
	OldestTs              int64             `json:"oldest_ts"`
	Truncated             bool              `json:"truncated"`
	TotalRequests         int               `json:"total_requests"`
	TotalTokens           int               `json:"total_tokens"`
	TotalTokensPrompt     int               `json:"total_tokens_prompt"`
	TotalTokensCompletion int               `json:"total_tokens_completion"`
	ErrorRate             float64           `json:"error_rate"`
	P95DurationMs         int64             `json:"p95_duration_ms"`
	ByProvider            []ProviderStat    `json:"by_provider"`
	ByGatewayKey          []GatewayKeyStat  `json:"by_gateway_key"`
	ByStatusClass         []StatusClassStat `json:"by_status_class"`
	TsBuckets             []TimeBucket      `json:"ts_buckets"`
}

type LogStore struct {
	mu   sync.RWMutex
	buf  []RequestLog
	cap  int
	head int
	size int
}

func NewLogStore(capacity int) *LogStore {
	if capacity <= 0 {
		capacity = 10000
	}
	return &LogStore{
		buf: make([]RequestLog, capacity),
		cap: capacity,
	}
}

func NewRequestID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}

func (s *LogStore) Cap() int {
	return s.cap
}

func (s *LogStore) Append(log RequestLog) {
	if log.ID == "" {
		log.ID = NewRequestID()
	}
	if log.Ts == 0 {
		log.Ts = time.Now().UnixMilli()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf[s.head] = log
	s.head = (s.head + 1) % s.cap
	if s.size < s.cap {
		s.size++
	}
}

func (s *LogStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.head = 0
	s.size = 0
	s.buf = make([]RequestLog, s.cap)
}

func (s *LogStore) snapshot() ([]RequestLog, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.size == 0 {
		return nil, 0
	}
	out := make([]RequestLog, s.size)
	start := 0
	if s.size == s.cap {
		start = s.head
	} else {
		start = (s.head - s.size + s.cap) % s.cap
	}
	for i := 0; i < s.size; i++ {
		out[i] = s.buf[(start+i)%s.cap]
	}
	return out, out[0].Ts
}

func (s *LogStore) List(q ListQuery) ListResult {
	all, oldest := s.snapshot()
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 100
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	filtered := make([]RequestLog, 0, len(all))
	for _, e := range all {
		if q.From > 0 && e.Ts < q.From {
			continue
		}
		if q.To > 0 && e.Ts > q.To {
			continue
		}
		if q.GatewayKeyID != "" && e.GatewayKeyID != q.GatewayKeyID {
			continue
		}
		if q.ProviderSlug != "" && e.ResolvedProvider != q.ProviderSlug {
			continue
		}
		if q.Directive != "" && !strings.Contains(strings.ToLower(e.Directive), strings.ToLower(q.Directive)) {
			continue
		}
		if q.StatusClass != "" && statusClass(e.Status) != q.StatusClass {
			continue
		}
		filtered = append(filtered, e)
	}

	switch q.Order {
	case "ts_asc":
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Ts < filtered[j].Ts })
	case "dur_desc":
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].DurationMs > filtered[j].DurationMs })
	default:
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Ts > filtered[j].Ts })
	}

	total := len(filtered)
	truncated := oldest > 0 && q.From > 0 && oldest > q.From
	if q.Offset >= total {
		return ListResult{
			Items:     []RequestLog{},
			Total:     total,
			OldestTs:  oldest,
			Capacity:  s.cap,
			Size:      len(all),
			Truncated: truncated,
		}
	}
	end := q.Offset + q.Limit
	if end > total {
		end = total
	}
	return ListResult{
		Items:     filtered[q.Offset:end],
		Total:     total,
		OldestTs:  oldest,
		Capacity:  s.cap,
		Size:      len(all),
		Truncated: truncated,
	}
}

func (s *LogStore) Stats(q StatsQuery) StatsResult {
	now := time.Now()
	window := q.Window
	if window == "" {
		window = "24h"
	}
	dur := windowDuration(window)
	from := now.Add(-dur).UnixMilli()
	to := now.UnixMilli()

	all, oldest := s.snapshot()
	truncated := oldest > 0 && oldest > from

	var (
		totalReq, totalTok, totalPrompt, totalComp int
		errors                                     int
		durations                                  []int64
		byProv                                     = map[string]*ProviderStat{}
		byKey                                      = map[string]*GatewayKeyStat{}
		byClass                                    = map[string]int{}
		buckets                                    = map[int64]*TimeBucket{}
	)

	bucketSize := bucketDuration(q.GroupBy, window)

	for _, e := range all {
		if e.Ts < from || e.Ts > to {
			continue
		}
		totalReq++
		tok := e.TokensPrompt + e.TokensCompletion
		totalTok += tok
		totalPrompt += e.TokensPrompt
		totalComp += e.TokensCompletion
		durations = append(durations, e.DurationMs)

		sc := statusClass(e.Status)
		byClass[sc]++
		if sc != "ok" {
			errors++
		}

		ps := e.ResolvedProvider
		if ps == "" {
			ps = "(unresolved)"
		}
		if _, ok := byProv[ps]; !ok {
			byProv[ps] = &ProviderStat{Slug: ps}
		}
		byProv[ps].Requests++
		byProv[ps].Tokens += tok
		if sc != "ok" {
			byProv[ps].Errors++
		}

		gk := e.GatewayKeyID
		if gk == "" {
			gk = "(none)"
		}
		if _, ok := byKey[gk]; !ok {
			byKey[gk] = &GatewayKeyStat{ID: gk}
		}
		byKey[gk].Requests++
		byKey[gk].Tokens += tok

		bt := e.Ts - (e.Ts % bucketSize.Milliseconds())
		if _, ok := buckets[bt]; !ok {
			buckets[bt] = &TimeBucket{Ts: bt}
		}
		b := buckets[bt]
		b.Tokens += tok
		b.TokensPrompt += e.TokensPrompt
		b.TokensCompletion += e.TokensCompletion
		switch sc {
		case "ok":
			b.RequestsOK++
		case "client_err":
			b.RequestsClientErr++
		default:
			b.RequestsServerErr++
		}
	}

	provStats := make([]ProviderStat, 0, len(byProv))
	for _, v := range byProv {
		provStats = append(provStats, *v)
	}
	sort.Slice(provStats, func(i, j int) bool { return provStats[i].Requests > provStats[j].Requests })

	keyStats := make([]GatewayKeyStat, 0, len(byKey))
	for _, v := range byKey {
		keyStats = append(keyStats, *v)
	}
	sort.Slice(keyStats, func(i, j int) bool { return keyStats[i].Requests > keyStats[j].Requests })

	classStats := make([]StatusClassStat, 0, len(byClass))
	for k, v := range byClass {
		classStats = append(classStats, StatusClassStat{Class: k, Count: v})
	}
	sort.Slice(classStats, func(i, j int) bool { return classStats[i].Count > classStats[j].Count })

	tsBuckets := make([]TimeBucket, 0, len(buckets))
	for _, v := range buckets {
		tsBuckets = append(tsBuckets, *v)
	}
	sort.Slice(tsBuckets, func(i, j int) bool { return tsBuckets[i].Ts < tsBuckets[j].Ts })

	var errRate float64
	if totalReq > 0 {
		errRate = float64(errors) / float64(totalReq)
	}

	return StatsResult{
		Window:                window,
		From:                  from,
		To:                    to,
		OldestTs:              oldest,
		Truncated:             truncated,
		TotalRequests:         totalReq,
		TotalTokens:           totalTok,
		TotalTokensPrompt:     totalPrompt,
		TotalTokensCompletion: totalComp,
		ErrorRate:             errRate,
		P95DurationMs:         percentile(durations, 0.95),
		ByProvider:            provStats,
		ByGatewayKey:          keyStats,
		ByStatusClass:         classStats,
		TsBuckets:             tsBuckets,
	}
}

func statusClass(status int) string {
	switch {
	case status >= 200 && status < 400:
		return "ok"
	case status >= 400 && status < 500:
		return "client_err"
	default:
		return "server_err"
	}
}

func windowDuration(w string) time.Duration {
	switch w {
	case "1h":
		return time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func bucketDuration(groupBy, window string) time.Duration {
	if groupBy == "day" {
		return 24 * time.Hour
	}
	if groupBy == "hour" {
		return time.Hour
	}
	switch window {
	case "1h":
		return 5 * time.Minute
	case "7d":
		return 24 * time.Hour
	default:
		return time.Hour
	}
}

func percentile(vals []int64, p float64) int64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]int64(nil), vals...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
