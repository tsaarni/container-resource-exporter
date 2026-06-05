package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
)

var pods = []string{"openbao-0", "openbao-1", "openbao-2"}
var ports = []int{8200, 8201, 8202}

func runStats() {
	token := os.Getenv("BAO_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "BAO_TOKEN environment variable is required")
		os.Exit(1)
	}

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
	smaps    := parseMetric(raw, "process_smaps_rss_bytes")
	smapsPSS := parseMetric(raw, "process_smaps_pss_bytes")
	fsize    := parseMetric(raw, "container_file_size_bytes")
	fdisk    := parseMetric(raw, "container_file_disk_usage_bytes")
	mntUsed  := parseMetric(raw, "container_mountpoint_used_bytes")
	mntCap   := parseMetric(raw, "container_mountpoint_capacity_bytes")
	mntAvail := parseMetric(raw, "container_mountpoint_available_bytes")
	cgMemCur  := parseMetric(raw, "cgroup_memory_current_bytes")
	cgMemPeak := parseMetric(raw, "cgroup_memory_peak_bytes")
	cgMemMax  := parseMetric(raw, "cgroup_memory_max_bytes")
	cgCPUThrottle := parseMetric(raw, "cgroup_cpu_throttled_seconds_total")

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
		vals := getByPod(smaps, pods, map[string]string{"container": "openbao", "namespace": "default", "path": r.path})
		for i, v := range vals {
			totals[i] += v
		}
		row(w, r.label, mbRow(vals))
	}
	row(w, "Total RSS", mbRow(totals[:]))
	heapPSS := getByPod(smapsPSS, pods, map[string]string{"container": "openbao", "namespace": "default", "path": "[anon: Go: heap]"})
	row(w, "Go heap PSS", mbRow(heapPSS))

	section(w, "Disk (file size / actual usage)")
	for _, path := range []string{"/openbao/data/raft/raft.db", "/openbao/data/vault.db"} {
		label := filepath.Base(path)
		sizes := getByPod(fsize, pods, map[string]string{"container": "openbao", "namespace": "default", "path": path})
		disks := getByPod(fdisk, pods, map[string]string{"container": "openbao", "namespace": "default", "path": path})
		row(w, label+" size", mbRow(sizes))
		row(w, label+" used", mbRow(disks))
	}

	section(w, "Mountpoint /openbao/data")
	mntLabels := map[string]string{"container": "openbao", "device": "tmpfs", "fstype": "tmpfs", "mountpoint": "/openbao/data", "namespace": "default"}
	for _, entry := range []struct {
		label  string
		metric map[string][]float64
	}{
		{"capacity", mntCap},
		{"used", mntUsed},
		{"available", mntAvail},
	} {
		row(w, entry.label, mbRow(getByPod(entry.metric, pods, mntLabels)))
	}

	cgLabels := map[string]string{"container": "openbao", "namespace": "default"}
	section(w, "Cgroup memory")
	for _, entry := range []struct {
		label  string
		metric map[string][]float64
	}{
		{"current", cgMemCur},
		{"peak", cgMemPeak},
		{"limit", cgMemMax},
	} {
		row(w, entry.label, mbRow(getByPod(entry.metric, pods, cgLabels)))
	}

	section(w, "CPU throttling")
	row(w, "throttled_seconds", secRow(getByPod(cgCPUThrottle, pods, cgLabels)))

	section(w, "OpenBao Go runtime")
	type gaugeMap map[string]float64
	gauges := make([]gaugeMap, len(ports))
	for i, port := range ports {
		body, err := httpGet(fmt.Sprintf("https://localhost:%d/v1/sys/metrics", port), token, tlsCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: port %d: %v\n", port, err)
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
	section(w, "OpenBao stored data (leader)")
	promBody, err := httpGet(fmt.Sprintf("https://localhost:%d/v1/sys/metrics?format=prometheus", ports[0]), token, tlsCfg)
	if err == nil {
		promMetrics := parseMetric(promBody, "vault_secret_kv_count")
		for labels, vals := range promMetrics {
			mount := extractLabel(labels, "mount_point")
			row(w, "kv secrets ("+mount+")", []string{strconv.Itoa(int(vals[0])), "-", "-"})
		}
		if v := promGaugeValue(promBody, "vault_token_count"); v >= 0 {
			row(w, "tokens", []string{strconv.Itoa(int(v)), "-", "-"})
		}
		if v := promGaugeValue(promBody, "vault_expire_num_leases"); v >= 0 {
			row(w, "leases", []string{strconv.Itoa(int(v)), "-", "-"})
		}
		if v := promGaugeValue(promBody, "vault_identity_entity_count"); v >= 0 {
			row(w, "entities", []string{strconv.Itoa(int(v)), "-", "-"})
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
		out[i] = fmt.Sprintf("%.1f MB", v/1024/1024)
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
	return io.ReadAll(resp.Body)
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

// parseMetric parses Prometheus text format lines for a given metric name.
// Returns map of label-set (as sorted "k=v,..." string) -> value.
func parseMetric(data []byte, name string) map[string][]float64 {
	result := map[string][]float64{}
	prefix := name + "{"
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		end := strings.LastIndex(line, "}")
		if end < 0 {
			continue
		}
		labelStr := line[len(prefix):end]
		valStr := strings.TrimSpace(line[end+1:])
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		result[labelStr] = append(result[labelStr], val)
	}
	return result
}

// getByPod looks up values for each pod, matching the given fixed labels.
func getByPod(metric map[string][]float64, pods []string, fixed map[string]string) []float64 {
	out := make([]float64, len(pods))
	for i, pod := range pods {
		match := map[string]string{"pod": pod}
		for k, v := range fixed {
			match[k] = v
		}
		for labelStr, vals := range metric {
			if labelsMatch(labelStr, match) {
				for _, v := range vals {
					out[i] += v
				}
				break
			}
		}
	}
	return out
}

func labelsMatch(labelStr string, match map[string]string) bool {
	for k, v := range match {
		needle := k + `="` + v + `"`
		if !strings.Contains(labelStr, needle) {
			return false
		}
	}
	return true
}

func extractLabel(labelStr, key string) string {
	needle := key + `="`
	idx := strings.Index(labelStr, needle)
	if idx < 0 {
		return ""
	}
	start := idx + len(needle)
	end := strings.Index(labelStr[start:], `"`)
	if end < 0 {
		return ""
	}
	return labelStr[start : start+end]
}

func promGaugeValue(data []byte, name string) float64 {
	// Match lines like: metric_name{labels} value  OR  metric_name value
	prefix := name + "{"
	bare := name + " "
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			end := strings.LastIndex(line, "}")
			if end < 0 {
				continue
			}
			valStr := strings.TrimSpace(line[end+1:])
			val, err := strconv.ParseFloat(valStr, 64)
			if err == nil {
				return val
			}
		} else if strings.HasPrefix(line, bare) {
			valStr := strings.TrimSpace(line[len(name):])
			val, err := strconv.ParseFloat(valStr, 64)
			if err == nil {
				return val
			}
		}
	}
	return -1
}
