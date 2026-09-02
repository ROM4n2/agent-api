// Package api 提供任务提交与查询的 HTTP 接口。
//
// 这一层只做「解析请求 → 落库 → 入队 → 立刻应答」，不执行任何任务逻辑；
// 真正的执行在 worker 包中异步进行。
package api

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// requestIDHeader 是回传给客户端的请求标识头，用于把一次请求的各处日志串起来。
const requestIDHeader = "X-Request-ID"

// requestCounter 给每个请求分配进程内唯一 ID；只需唯一、不需随机，
// 因此用原子自增而非 crypto/rand，零分配、无系统调用。
var requestCounter atomic.Uint64

// Recover 兜底 handler 内的 panic，避免单个请求把整个进程拖崩（GO-STANDARDS 7.5 禁止 log.Fatal，但允许 recover）。
// 返回 500 而非让连接半开，调用方才能正确感知失败并重试。
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "path", r.URL.Path, "panic", rec)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestID 为每个请求生成唯一 ID 并写回响应头，方便日志关联与问题排查。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestCounter.Add(1)
		w.Header().Set(requestIDHeader, formatRequestID(id))
		next.ServeHTTP(w, r)
	})
}

// formatRequestID 把自增计数转成可读的十六进制串，长度固定便于日志对齐。
func formatRequestID(n uint64) string {
	const hex = "0123456789abcdef"
	var buf [16]byte
	for i := 15; i >= 0; i-- {
		buf[i] = hex[n&0xf]
		n >>= 4
	}
	return string(buf[:])
}

// Auth 做 Bearer Token 鉴权。expectedKey 由调用方注入（来自 config），
// 为空时进入开发模式直接放行；生产必须配置，否则任何人都可提交任务、消耗 LLM 配额。
func Auth(expectedKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if expectedKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if got == "" || got != expectedKey {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimiter 是每客户端 IP 的令牌桶限流器，防止单一来源刷爆 LLM 配额。
// 仅用标准库实现，零依赖；桶按 IP 惰性创建，演示规模下内存占用可忽略。
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // 每秒补充令牌数
	burst   int     // 桶容量（突发上限）
}

type tokenBucket struct {
	tokens    float64
	last      time.Time
}

// newRateLimiter 构造限流器：rate 为每秒放行请求数，burst 为瞬时突发上限。
func newRateLimiter(rate float64, burst int) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

// allow 原子地尝试取一个令牌；成功返回 true。
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{tokens: float64(rl.burst), last: now}
		rl.buckets[ip] = b
	}

	// 按经过的时间补充令牌，上限为 burst
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.last = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Limit 把限流包装进中间件；超限返回 429，并带 Retry-After 头提示客户端退避。
func (rl *rateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.allow(clientIP(r)) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP 优先取 X-Forwarded-For（过代理时），否则用 RemoteAddr 的 IP 部分。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// 可能是 "ip1, ip2"，取第一个
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
