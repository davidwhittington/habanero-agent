package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PingResult holds the result of a ping probe.
type PingResult struct {
	Host       string        `json:"host"`
	Sent       int           `json:"sent"`
	Received   int           `json:"received"`
	Loss       float64       `json:"loss_percent"`
	MinRTT     time.Duration `json:"min_rtt_ms"`
	AvgRTT     time.Duration `json:"avg_rtt_ms"`
	MaxRTT     time.Duration `json:"max_rtt_ms"`
	Error      string        `json:"error,omitempty"`
}

// HTTPTimingResult holds detailed HTTP timing breakdown.
type HTTPTimingResult struct {
	URL             string        `json:"url"`
	StatusCode      int           `json:"status_code"`
	DNSLookup       time.Duration `json:"dns_lookup_ms"`
	TCPConnect      time.Duration `json:"tcp_connect_ms"`
	TLSHandshake    time.Duration `json:"tls_handshake_ms"`
	TimeToFirstByte time.Duration `json:"ttfb_ms"`
	TotalTime       time.Duration `json:"total_ms"`
	Error           string        `json:"error,omitempty"`
}

// Ping executes a ping to the given host and parses the results.
// Uses the system ping command for ICMP (requires no special privileges).
func Ping(host string, count int, timeout time.Duration) PingResult {
	result := PingResult{Host: host, Sent: count}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"-c", strconv.Itoa(count), "-W", strconv.Itoa(int(timeout.Seconds())), host}
	cmd := exec.CommandContext(ctx, "ping", args...)
	out, err := cmd.CombinedOutput()
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		result.Error = "timeout"
		result.Loss = 100
		return result
	}

	output := string(out)

	// Parse packet loss line: "X packets transmitted, Y received, Z% packet loss"
	lossRe := regexp.MustCompile(`(\d+) packets transmitted, (\d+) (?:packets )?received.*?(\d+(?:\.\d+)?)% packet loss`)
	if matches := lossRe.FindStringSubmatch(output); len(matches) >= 4 {
		result.Sent, _ = strconv.Atoi(matches[1])
		result.Received, _ = strconv.Atoi(matches[2])
		result.Loss, _ = strconv.ParseFloat(matches[3], 64)
	}

	// Parse RTT line: "rtt min/avg/max/mdev = X/Y/Z/W ms"
	// Also handles "round-trip min/avg/max/stddev" on macOS.
	rttRe := regexp.MustCompile(`(?:rtt|round-trip)\s+min/avg/max/(?:mdev|stddev)\s*=\s*([\d.]+)/([\d.]+)/([\d.]+)`)
	if matches := rttRe.FindStringSubmatch(output); len(matches) >= 4 {
		minMs, _ := strconv.ParseFloat(matches[1], 64)
		avgMs, _ := strconv.ParseFloat(matches[2], 64)
		maxMs, _ := strconv.ParseFloat(matches[3], 64)
		result.MinRTT = time.Duration(minMs * float64(time.Millisecond))
		result.AvgRTT = time.Duration(avgMs * float64(time.Millisecond))
		result.MaxRTT = time.Duration(maxMs * float64(time.Millisecond))
	}

	if err != nil && result.Received == 0 {
		result.Error = "host unreachable"
		result.Loss = 100
	}

	return result
}

// DNSResolve performs a DNS lookup for the given domain using the specified
// DNS server. Returns the resolved address, lookup duration, and any error.
func DNSResolve(domain, server string) (string, time.Duration, error) {
	resolver := &net.Resolver{
		PreferGo: true,
	}

	if server != "" {
		if !strings.Contains(server, ":") {
			server = server + ":53"
		}
		resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", server)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	addrs, err := resolver.LookupHost(ctx, domain)
	elapsed := time.Since(start)

	if err != nil {
		return "", elapsed, fmt.Errorf("dns lookup failed for %s: %w", domain, err)
	}

	if len(addrs) == 0 {
		return "", elapsed, fmt.Errorf("dns lookup returned no addresses for %s", domain)
	}

	return addrs[0], elapsed, nil
}

// TCPConnect attempts a TCP connection to the given host:port and returns
// whether the connection succeeded and how long it took.
func TCPConnect(host string, port int, timeout time.Duration) (bool, time.Duration) {
	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)

	if err != nil {
		return false, elapsed
	}
	conn.Close()
	return true, elapsed
}

// HTTPTiming performs an HTTP GET request and returns detailed timing breakdown.
func HTTPTiming(url string) HTTPTimingResult {
	result := HTTPTimingResult{URL: url}

	var dnsStart, dnsEnd, connectStart, connectEnd, tlsStart, tlsEnd, gotFirstByte time.Time
	requestStart := time.Now()

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			dnsEnd = time.Now()
		},
		ConnectStart: func(_, _ string) {
			connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			connectEnd = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ interface{ ConnectionState() interface{} }, _ error) {
			tlsEnd = time.Now()
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
		},
	}

	// Use a custom trace-aware TLS handshake hook.
	trace = &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			dnsEnd = time.Now()
		},
		ConnectStart: func(_, _ string) {
			connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			connectEnd = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ interface{}, _ error) {
			tlsEnd = time.Now()
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	totalTime := time.Since(requestStart)

	if err != nil {
		result.Error = err.Error()
		result.TotalTime = totalTime
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.TotalTime = totalTime

	if !dnsStart.IsZero() && !dnsEnd.IsZero() {
		result.DNSLookup = dnsEnd.Sub(dnsStart)
	}
	if !connectStart.IsZero() && !connectEnd.IsZero() {
		result.TCPConnect = connectEnd.Sub(connectStart)
	}
	if !tlsStart.IsZero() && !tlsEnd.IsZero() {
		result.TLSHandshake = tlsEnd.Sub(tlsStart)
	}
	if !gotFirstByte.IsZero() {
		result.TimeToFirstByte = gotFirstByte.Sub(requestStart)
	}

	return result
}
