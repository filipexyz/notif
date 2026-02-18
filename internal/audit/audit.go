package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"

	"github.com/filipexyz/notif/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// Logger provides structured audit logging with dual-write to slog (sync) and Postgres (async).
type Logger struct {
	queries *db.Queries
	ch      chan entry
	mu      sync.Mutex // guards closed + ch send atomically (prevents TOCTOU race)
	closed  bool
	once    sync.Once
}

type entry struct {
	actor  string
	action string
	orgID  string
	target string
	detail map[string]any
	ip     string
}

// New creates a new audit Logger. The buffer parameter controls the async channel size.
func New(queries *db.Queries, buffer int) *Logger {
	if buffer <= 0 {
		buffer = 256
	}
	l := &Logger{
		queries: queries,
		ch:      make(chan entry, buffer),
	}
	go l.drain()
	return l
}

// Log records an audit event. It writes to slog synchronously and to Postgres asynchronously.
// actor: who performed the action (e.g. "api:key_abc", "notifd", "cli:admin")
// action: what was done (e.g. "event.emit", "webhook.create")
// orgID: organization scope (empty string for system-level actions)
// target: what was acted on (e.g. topic name, webhook ID)
// detail: additional metadata (nil is fine)
func (l *Logger) Log(ctx context.Context, actor, action, orgID, target string, detail map[string]any) {
	ip := ipFromContext(ctx)

	// Sync: always log to slog (never lose audit data)
	attrs := []any{
		slog.String("actor", actor),
		slog.String("action", action),
	}
	if orgID != "" {
		attrs = append(attrs, slog.String("org_id", orgID))
	}
	if target != "" {
		attrs = append(attrs, slog.String("target", target))
	}
	if ip != "" {
		attrs = append(attrs, slog.String("ip_address", ip))
	}
	if detail != nil {
		attrs = append(attrs, slog.Any("detail", detail))
	}
	slog.Info("audit", attrs...)

	// Async: best-effort DB insert.
	// Mutex ensures closed check + channel send are atomic,
	// preventing send-on-closed-channel panic (TOCTOU race with Close).
	e := entry{
		actor:  actor,
		action: action,
		orgID:  orgID,
		target: target,
		detail: detail,
		ip:     ip,
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	select {
	case l.ch <- e:
	default:
		slog.Warn("audit log channel full, dropping event", "action", action)
	}
	l.mu.Unlock()
}

// drain processes the async channel and inserts into Postgres.
func (l *Logger) drain() {
	for e := range l.ch {
		if l.queries == nil {
			continue
		}

		var detailJSON []byte
		if e.detail != nil {
			var err error
			detailJSON, err = json.Marshal(e.detail)
			if err != nil {
				slog.Warn("audit detail marshal failed", "error", err, "action", e.action)
			}
		}

		var ipAddr *netip.Addr
		if e.ip != "" {
			if parsed, err := netip.ParseAddr(e.ip); err == nil {
				ipAddr = &parsed
			}
		}

		params := db.InsertAuditLogParams{
			Actor:     e.actor,
			Action:    e.action,
			OrgID:     pgtype.Text{String: e.orgID, Valid: e.orgID != ""},
			Target:    pgtype.Text{String: e.target, Valid: e.target != ""},
			Detail:    detailJSON,
			IpAddress: ipAddr,
		}

		if err := l.queries.InsertAuditLog(context.Background(), params); err != nil {
			slog.Error("audit log DB insert failed", "error", err, "action", e.action)
		}
	}
}

// Close drains remaining events and closes the channel. Safe to call multiple times.
func (l *Logger) Close() {
	l.once.Do(func() {
		l.mu.Lock()
		l.closed = true
		close(l.ch)
		l.mu.Unlock()
	})
}

// context key for IP address
type ctxKey string

const ipKey ctxKey = "audit_ip"

// WithIP returns a context with the client IP address stored for audit logging.
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ipKey, ip)
}

// IPFromRequest extracts the client IP from an HTTP request.
// Uses X-Forwarded-For / X-Real-Ip headers if present, falls back to RemoteAddr.
// NOTE: These headers are informational only and can be spoofed by clients.
// Do not use for access control without validating against trusted proxy CIDRs.
func IPFromRequest(r *http.Request) string {
	// chi's RealIP middleware sets RemoteAddr, but we can also check headers
	ip := r.Header.Get("X-Real-Ip")
	if ip != "" {
		return ip
	}
	ip = r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// X-Forwarded-For can be comma-separated; take the first (client) IP
		if idx := strings.IndexByte(ip, ','); idx != -1 {
			ip = strings.TrimSpace(ip[:idx])
		}
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func ipFromContext(ctx context.Context) string {
	ip, _ := ctx.Value(ipKey).(string)
	return ip
}
