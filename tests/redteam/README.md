# Red Team - notif.sh

Área dedicada a testes de segurança ofensivos para identificar e validar vulnerabilidades.

## Estrutura

```
tests/redteam/
├── README.md           # Este arquivo
├── VULNERABILITIES.md  # Catálogo de vulns conhecidas
├── exploits/           # Scripts de exploração
│   ├── websocket/      # WebSocket hijacking, CSWSH
│   ├── ssrf/           # SSRF via webhooks
│   ├── auth/           # Auth bypass, token attacks
│   └── dos/            # Rate limiting, resource exhaustion
├── payloads/           # Payloads reutilizáveis
└── reports/            # Relatórios de testes
```

## Quick Start

```bash
# Instalar dependências
cd tests/redteam
go mod tidy

# Rodar todos os testes de segurança
go test ./... -tags=redteam -v

# Testar vulnerabilidade específica
go test ./exploits/ssrf -v -run TestSSRF

# Gerar relatório
go test ./... -tags=redteam -json > reports/$(date +%Y%m%d).json
```

## Ambiente

```bash
# Target (NÃO usar em produção!)
export REDTEAM_TARGET=http://localhost:8080
export REDTEAM_WS_TARGET=ws://localhost:8080/ws

# API key de teste (opcional - alguns testes são unauth)
export REDTEAM_API_KEY=nsh_testkey1234567890abcdefghijk
```

## Regras

1. **NUNCA** executar contra produção sem autorização explícita
2. Documentar todas as vulnerabilidades encontradas em `VULNERABILITIES.md`
3. Criar PoC reproduzível para cada vulnerabilidade
4. Reportar findings críticos imediatamente

## Severidade

| Nível | Descrição | SLA |
|-------|-----------|-----|
| CRITICAL | RCE, Auth bypass total, Data breach | 24h |
| HIGH | SSRF, Info disclosure significativo | 72h |
| MEDIUM | Rate limit bypass, Limited auth issues | 1 semana |
| LOW | Info leakage menor, Best practices | 2 semanas |

## Responsáveis

- Security Lead: @luis
- Última auditoria: 2026-01-27
