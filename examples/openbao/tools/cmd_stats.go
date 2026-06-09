package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

var pods = []string{"openbao-0", "openbao-1", "openbao-2"}
var ports = []int{8200, 8201, 8202}

func runStats() {
	token := os.Getenv("BAO_TOKEN")

	exporter := "http://localhost:8080/metrics"
	cacert := os.Getenv("BAO_CACERT")
	if cacert == "" {
		cacert = "../certs/ca.pem"
	}

	raw, err := httpGet(exporter, "", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch exporter:", err)
		os.Exit(1)
	}
	families, err := parseMetrics(raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse metrics:", err)
		os.Exit(1)
	}

	tlsCfg, err := tlsConfig(cacert)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load CA:", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	section(w, "Memory RSS (smaps)")
	smapRows := []struct{ path, label string }{
		{"/bin/bao", "binary RSS"},
		{"[anon: Go: heap]", "Go heap RSS"},
		{"[anon: Go: immortal metadata]", "Go immortal metadata"},
		{"[anon: Go: gc bits]", "Go gc bits"},
		{"[anon: Go: profiler hash buckets]", "Go profiler buckets"},
		{"/openbao/data/raft/raft.db", "raft.db mmap"},
		{"/openbao/data/vault.db", "vault.db mmap"},
	}
	var totals [3]float64
	for _, r := range smapRows {
		vals := getByPod(families, "process_smaps_rss_bytes", pods, map[string]string{"container": "openbao", "namespace": "default", "path": r.path})
		for i, v := range vals {
			totals[i] += v
		}
		row(w, r.label, mbRow(vals))
	}
	row(w, "Total RSS", mbRow(totals[:]))
	heapPSS := getByPod(families, "process_smaps_pss_bytes", pods, map[string]string{"container": "openbao", "namespace": "default", "path": "[anon: Go: heap]"})
	row(w, "Go heap PSS", mbRow(heapPSS))

	section(w, "Disk (file size / actual usage)")
	for _, path := range []string{"/openbao/data/raft/raft.db", "/openbao/data/vault.db"} {
		label := filepath.Base(path)
		sizes := getByPod(families, "container_file_size_bytes", pods, map[string]string{"container": "openbao", "namespace": "default", "path": path})
		disks := getByPod(families, "container_file_disk_usage_bytes", pods, map[string]string{"container": "openbao", "namespace": "default", "path": path})
		row(w, label+" size", mbRow(sizes))
		row(w, label+" used", mbRow(disks))
	}

	section(w, "Mountpoint /openbao/data")
	mntLabels := map[string]string{"container": "openbao", "mountpoint": "/openbao/data", "namespace": "default"}
	for _, entry := range []struct {
		label string
		name  string
	}{
		{"capacity", "container_mountpoint_capacity_bytes"},
		{"used", "container_mountpoint_used_bytes"},
		{"available", "container_mountpoint_available_bytes"},
	} {
		row(w, entry.label, mbRow(getByPod(families, entry.name, pods, mntLabels)))
	}

	cgLabels := map[string]string{"container": "openbao", "namespace": "default"}
	section(w, "Cgroup memory")
	for _, entry := range []struct {
		label string
		name  string
	}{
		{"current", "cgroup_memory_current_bytes"},
		{"peak", "cgroup_memory_peak_bytes"},
		{"limit", "cgroup_memory_max_bytes"},
	} {
		row(w, entry.label, mbRow(getByPod(families, entry.name, pods, cgLabels)))
	}

	section(w, "CPU throttling")
	row(w, "throttled_seconds", secRow(getByPod(families, "cgroup_cpu_throttled_seconds_total", pods, cgLabels)))

	section(w, "OpenBao Go runtime")
	type gaugeMap map[string]float64
	gauges := make([]gaugeMap, len(ports))
	var authError bool
	for i, port := range ports {
		url := fmt.Sprintf("https://localhost:%d/v1/sys/metrics", port)
		body, err := httpGet(url, token, tlsCfg)
		if err != nil {
			if !authError && strings.Contains(err.Error(), "403") {
				authError = true
				slog.Warn("OpenBao returned 403 — set BAO_TOKEN to enable runtime metrics")
			} else if !authError {
				slog.Error("Failed to fetch metrics", "host", fmt.Sprintf("localhost:%d", port), "path", "/v1/sys/metrics", "err", err)
			}
			gauges[i] = gaugeMap{}
			continue
		}
		var resp struct {
			Gauges []struct {
				Name  string  `json:"Name"`
				Value float64 `json:"Value"`
			} `json:"Gauges"`
		}
		json.Unmarshal(body, &resp)
		gauges[i] = make(gaugeMap)
		for _, g := range resp.Gauges {
			gauges[i][g.Name] = g.Value
		}
	}
	for _, entry := range []struct{ key, label string }{
		{"vault.runtime.alloc_bytes", "alloc_bytes"},
		{"vault.runtime.sys_bytes", "sys_bytes"},
		{"vault.runtime.heap_objects", "heap_objects"},
		{"vault.runtime.num_goroutines", "num_goroutines"},
		{"vault.runtime.total_gc_runs", "gc_runs"},
	} {
		vals := make([]float64, len(gauges))
		for i, g := range gauges {
			vals[i] = g[entry.key]
		}
		var cols []string
		if strings.HasSuffix(entry.key, "_bytes") {
			cols = mbRow(vals)
		} else {
			for _, v := range vals {
				cols = append(cols, strconv.Itoa(int(v)))
			}
		}
		row(w, entry.label, cols)
	}

	// Show stored data counts from the leader only (gauge collection runs on active node)
	leaderPort := findLeader(ports, token, tlsCfg)
	leaderIdx := 0
	for i, p := range ports {
		if p == leaderPort {
			leaderIdx = i
			break
		}
	}
	section(w, "OpenBao stored data (leader)")
	if authError {
		row(w, "(skipped)", []string{"no valid token"})
	} else {
		promBody, err := httpGet(fmt.Sprintf("https://localhost:%d/v1/sys/metrics?format=prometheus", leaderPort), token, tlsCfg)
		if err != nil {
			row(w, "(unavailable)", []string{err.Error()})
		} else {
			promFamilies, _ := parseMetrics(promBody)
			if fam := promFamilies["vault_secret_kv_count"]; fam != nil {
				for _, m := range fam.GetMetric() {
					mount := getLabelValue(m, "mount_point")
					row(w, "kv secrets ("+mount+")", leaderRow(leaderIdx, strconv.Itoa(int(m.GetGauge().GetValue()))))
				}
			}
			for _, entry := range []struct{ metric, label string }{
				{"vault_token_count", "tokens"},
				{"vault_expire_num_leases", "leases"},
				{"vault_identity_entity_count", "entities"},
			} {
				if v := gaugeValue(promFamilies, entry.metric); v >= 0 {
					row(w, entry.label, leaderRow(leaderIdx, strconv.Itoa(int(v))))
				}
			}
		}
	}

	w.Flush()
}

func header(w *tabwriter.Writer) {
	fmt.Fprintf(w, "  %-30s", "")
	for _, p := range pods {
		fmt.Fprintf(w, "\t%s", p)
	}
	fmt.Fprintln(w)
}

func section(w *tabwriter.Writer, title string) {
	w.Flush()
	fmt.Printf("\n--- %s ---\n", title)
	header(w)
}

func row(w *tabwriter.Writer, label string, vals []string) {
	fmt.Fprintf(w, "  %-30s", label)
	for _, v := range vals {
		fmt.Fprintf(w, "\t%s", v)
	}
	fmt.Fprintln(w)
}

func mbRow(vals []float64) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = humanize.IBytes(uint64(v))
	}
	return out
}

func secRow(vals []float64) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = fmt.Sprintf("%.1fs", v)
	}
	return out
}

func leaderRow(leaderIdx int, val string) []string {
	out := make([]string, len(pods))
	for i := range out {
		out[i] = "-"
	}
	out[leaderIdx] = val
	return out
}

func httpGet(url, token string, tlsCfg *tls.Config) ([]byte, error) {
	transport := http.DefaultTransport
	if tlsCfg != nil {
		transport = &http.Transport{TLSClientConfig: tlsCfg}
	}
	client := &http.Client{Transport: transport}
	req, _ := http.NewRequest("GET", url, nil)
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode == 403 {
			msg += " (check BAO_TOKEN)"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return body, nil
}

func findLeader(ports []int, token string, tlsCfg *tls.Config) int {
	for _, port := range ports {
		body, err := httpGet(fmt.Sprintf("https://localhost:%d/v1/sys/health", port), token, tlsCfg)
		if err != nil {
			continue
		}
		var health struct{ Standby bool `json:"standby"` }
		if json.Unmarshal(body, &health) == nil && !health.Standby {
			return port
		}
	}
	return ports[0]
}

func tlsConfig(caFile string) (*tls.Config, error) {
	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca)
	return &tls.Config{RootCAs: pool}, nil
}

// parseMetrics parses Prometheus text exposition format into metric families.
func parseMetrics(data []byte) (map[string]*dto.MetricFamily, error) {
	parser := expfmt.NewTextParser(model.UTF8Validation)
	return parser.TextToMetricFamilies(bytes.NewReader(data))
}

// getByPod returns one value per pod for the named metric matching the given labels.
func getByPod(families map[string]*dto.MetricFamily, name string, pods []string, fixed map[string]string) []float64 {
	out := make([]float64, len(pods))
	fam := families[name]
	if fam == nil {
		return out
	}
	for i, pod := range pods {
		match := make(map[string]string, len(fixed)+1)
		for k, v := range fixed {
			match[k] = v
		}
		match["pod"] = pod
		for _, m := range fam.GetMetric() {
			if labelsMatch(m, match) {
				out[i] += metricValue(m)
			}
		}
	}
	return out
}

func labelsMatch(m *dto.Metric, match map[string]string) bool {
	for k, v := range match {
		if getLabelValue(m, k) != v {
			return false
		}
	}
	return true
}

func getLabelValue(m *dto.Metric, name string) string {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}

func metricValue(m *dto.Metric) float64 {
	if g := m.GetGauge(); g != nil {
		return g.GetValue()
	}
	if c := m.GetCounter(); c != nil {
		return c.GetValue()
	}
	if u := m.GetUntyped(); u != nil {
		return u.GetValue()
	}
	return 0
}

func gaugeValue(families map[string]*dto.MetricFamily, name string) float64 {
	fam := families[name]
	if fam == nil || len(fam.GetMetric()) == 0 {
		return -1
	}
	return fam.GetMetric()[0].GetGauge().GetValue()
}
