# Red Team Test Runner
# Usage: make test-all TARGET=http://localhost:8080

TARGET ?= http://localhost:8080
API_KEY ?= $(REDTEAM_API_KEY)

.PHONY: help test-all test-websocket test-ssrf test-auth test-dos report clean

help:
	@echo "Red Team Test Suite for notif.sh"
	@echo ""
	@echo "Usage:"
	@echo "  make test-all TARGET=http://localhost:8080 API_KEY=nsh_xxx"
	@echo ""
	@echo "Available targets:"
	@echo "  test-all       Run all security tests"
	@echo "  test-websocket Run WebSocket hijacking tests (NOTIF-001)"
	@echo "  test-ssrf      Run SSRF tests (NOTIF-002)"
	@echo "  test-auth      Run authentication bypass tests"
	@echo "  test-dos       Run DoS/rate limit tests (NOTIF-004)"
	@echo "  report         Generate HTML report"
	@echo "  clean          Remove test artifacts"
	@echo ""
	@echo "Environment:"
	@echo "  TARGET=$(TARGET)"
	@echo "  API_KEY=$(if $(API_KEY),[SET],[NOT SET])"

test-all: deps
	@echo "Running all red team tests against $(TARGET)..."
	REDTEAM_TARGET=$(TARGET) REDTEAM_API_KEY=$(API_KEY) \
		go test ./... -tags=redteam -v -timeout 10m 2>&1 | tee reports/latest.log
	@echo "Results saved to reports/latest.log"

test-websocket: deps
	@echo "Testing WebSocket vulnerabilities (NOTIF-001)..."
	REDTEAM_TARGET=$(TARGET) REDTEAM_API_KEY=$(API_KEY) \
		go test ./exploits/websocket/... -tags=redteam -v

test-ssrf: deps
	@echo "Testing SSRF vulnerabilities (NOTIF-002)..."
	REDTEAM_TARGET=$(TARGET) REDTEAM_API_KEY=$(API_KEY) \
		go test ./exploits/ssrf/... -tags=redteam -v

test-auth: deps
	@echo "Testing authentication vulnerabilities..."
	REDTEAM_TARGET=$(TARGET) REDTEAM_API_KEY=$(API_KEY) \
		go test ./exploits/auth/... -tags=redteam -v

test-dos: deps
	@echo "Testing DoS/rate limiting (NOTIF-004)..."
	@echo "WARNING: This test may impact server performance!"
	REDTEAM_TARGET=$(TARGET) REDTEAM_API_KEY=$(API_KEY) \
		go test ./exploits/dos/... -tags=redteam -v

test-quick: deps
	@echo "Running quick tests (no DoS/flood tests)..."
	REDTEAM_TARGET=$(TARGET) REDTEAM_API_KEY=$(API_KEY) \
		go test ./... -tags=redteam -v -short

deps:
	@go mod tidy

report: test-all
	@echo "Generating report..."
	@mkdir -p reports
	@echo "<!DOCTYPE html><html><head><title>Red Team Report - $$(date)</title>" > reports/report.html
	@echo "<style>body{font-family:monospace;margin:20px}pre{background:#f5f5f5;padding:10px;overflow-x:auto}.PASS{color:green}.FAIL{color:red}.SKIP{color:orange}</style></head><body>" >> reports/report.html
	@echo "<h1>Red Team Report</h1>" >> reports/report.html
	@echo "<p>Target: $(TARGET)</p>" >> reports/report.html
	@echo "<p>Date: $$(date)</p>" >> reports/report.html
	@echo "<h2>Results</h2><pre>" >> reports/report.html
	@cat reports/latest.log | sed 's/--- PASS/<span class="PASS">--- PASS<\/span>/g' | sed 's/--- FAIL/<span class="FAIL">--- FAIL<\/span>/g' | sed 's/--- SKIP/<span class="SKIP">--- SKIP<\/span>/g' >> reports/report.html
	@echo "</pre></body></html>" >> reports/report.html
	@echo "Report generated: reports/report.html"

clean:
	rm -rf reports/*.log reports/*.html
	go clean -testcache

# Quick vulnerability check (read-only, no destructive tests)
scan: deps
	@echo "Running passive security scan..."
	REDTEAM_TARGET=$(TARGET) \
		go test ./exploits/auth/... -tags=redteam -v -run "TestNoAuthAccess|TestBootstrapEndpoint"
	REDTEAM_TARGET=$(TARGET) \
		go test ./exploits/websocket/... -tags=redteam -v -run "TestCSWSH"
