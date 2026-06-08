package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

var pods = []string{"envoy", "contour", "echoserver"}
var containers = []string{"envoy", "contour", "echoserver"}

func runStats() {
	exporterRaw, err := httpGet("http://localhost:8080/metrics")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch exporter:", err)
		os.Exit(1)
	}
	exporter, err := parseMetrics(exporterRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse exporter:", err)
		os.Exit(1)
	}

	envoyRaw, err := httpGet("http://localhost:9001/stats/prometheus")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch envoy:", err)
		os.Exit(1)
	}
	envoy, err := parseMetrics(envoyRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse envoy:", err)
		os.Exit(1)
	}

	contourRaw, err := httpGet("http://localhost:9002/metrics")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch contour:", err)
		os.Exit(1)
	}
	contour, err := parseMetrics(contourRaw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse contour:", err)
		os.Exit(1)
	}

	serverInfoRaw, err := httpGet("http://localhost:9001/server_info")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fetch server_info:", err)
		os.Exit(1)
	}
	var serverInfo struct {
		Version            string `json:"version"`
		State              string `json:"state"`
		UptimeCurrentEpoch string `json:"uptime_current_epoch"`
		CommandLineOptions struct {
			Concurrency int `json:"concurrency"`
		} `json:"command_line_options"`
	}
	json.Unmarshal(serverInfoRaw, &serverInfo)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	section(w, "Envoy")
	fmt.Fprintf(w, "  %-26s\t%s  state=%s  uptime=%s  workers=%d\n", "server",
		serverInfo.Version, serverInfo.State, serverInfo.UptimeCurrentEpoch, serverInfo.CommandLineOptions.Concurrency)
	fmt.Fprintf(w, "  %-26s\talloc=%-10s\theap=%-10s\tphysical=%s\n", "tcmalloc",
		fmtMB(sumMetric(envoy, "envoy_server_memory_allocated")),
		fmtMB(sumMetric(envoy, "envoy_server_memory_heap_size")),
		fmtMB(sumMetric(envoy, "envoy_server_memory_physical_size")))
	fmt.Fprintf(w, "  %-26s\ttotal=%-10s\tactive=%s\n", "downstream_cx",
		formatNum(sumMetric(envoy, "envoy_http_downstream_cx_total")),
		formatNum(sumMetric(envoy, "envoy_http_downstream_cx_active")))
	fmt.Fprintf(w, "  %-26s\ttotal=%-10s\tactive=%s\n", "downstream_rq",
		formatNum(sumMetric(envoy, "envoy_http_downstream_rq_total")),
		formatNum(sumMetric(envoy, "envoy_http_downstream_rq_active")))
	fmt.Fprintf(w, "  %-26s\ttotal=%-10s\tactive=%s\n", "upstream_cx",
		formatNum(sumMetric(envoy, "envoy_cluster_upstream_cx_total")),
		formatNum(sumMetric(envoy, "envoy_cluster_upstream_cx_active")))
	fmt.Fprintf(w, "  %-26s\ttotal=%-10s\tactive=%s\n", "upstream_rq",
		formatNum(sumMetric(envoy, "envoy_cluster_upstream_rq_total")),
		formatNum(sumMetric(envoy, "envoy_cluster_upstream_rq_active")))
	// Response codes
	if fam := envoy["envoy_http_downstream_rq_xx"]; fam != nil {
		codeTotals := map[string]float64{}
		for _, m := range fam.GetMetric() {
			prefix := getLabelValue(m, "envoy_http_conn_manager_prefix")
			if prefix == "admin" || prefix == "stats" {
				continue
			}
			class := getLabelValue(m, "envoy_response_code_class")
			codeTotals[class] += metricValue(m)
		}
		fmt.Fprintf(w, "  %-26s\t2xx=%-10s\t4xx=%-10s\t5xx=%s\n", "response_codes",
			formatNum(codeTotals["2"]), formatNum(codeTotals["4"]), formatNum(codeTotals["5"]))
	}
	// TLS
	handshakes := sumMetric(envoy, "envoy_listener_ssl_handshake")
	sslErrors := sumMetric(envoy, "envoy_listener_ssl_connection_error")
	fmt.Fprintf(w, "  %-26s\thandshakes=%-6s\terrors=%s\n", "tls", formatNum(handshakes), formatNum(sslErrors))
	// Upstream latency
	if fam := envoy["envoy_cluster_upstream_rq_time"]; fam != nil {
		var totalCount uint64
		buckets := map[float64]uint64{}
		for _, m := range fam.GetMetric() {
			h := m.GetHistogram()
			if h == nil {
				continue
			}
			totalCount += h.GetSampleCount()
			for _, b := range h.GetBucket() {
				buckets[b.GetUpperBound()] += b.GetCumulativeCount()
			}
		}
		if totalCount > 0 {
			p50 := percentileFromBuckets(buckets, totalCount, 0.5)
			p95 := percentileFromBuckets(buckets, totalCount, 0.95)
			p99 := percentileFromBuckets(buckets, totalCount, 0.99)
			fmt.Fprintf(w, "  %-26s\tp50=%s  p95=%s  p99=%s\n", "upstream_latency",
				formatDurationMs(p50/1000), formatDurationMs(p95/1000), formatDurationMs(p99/1000))
		}
	}
	// Overload
	overloadActive := sumMetric(envoy, "envoy_server_overload_active_gauge")
	cxOverflow := sumMetric(envoy, "envoy_listener_downstream_cx_overflow")
	overloadReject := sumMetric(envoy, "envoy_server_total_connections_overload_reject")
	noFilterMatch := sumMetric(envoy, "envoy_listener_no_filter_chain_match")
	fmt.Fprintf(w, "  %-26s\tactive=%s  cx_overflow=%s  reject=%s  no_filter_match=%s\n", "overload",
		formatNum(overloadActive), formatNum(cxOverflow), formatNum(overloadReject), formatNum(noFilterMatch))
	printClusterStats(w, envoy)

	section(w, "Contour")
	for _, entry := range []struct{ metric, label string }{
		{"go_memstats_alloc_bytes", "alloc_bytes"},
		{"go_memstats_sys_bytes", "sys_bytes"},
		{"go_goroutines", "goroutines"},
	} {
		v := sumMetric(contour, entry.metric)
		if strings.HasSuffix(entry.metric, "_bytes") {
			fmt.Fprintf(w, "  %-26s\t%s\n", entry.label, fmtMB(v))
		} else {
			fmt.Fprintf(w, "  %-26s\t%s\n", entry.label, formatNum(v))
		}
	}
	dagRebuilds := sumMetric(contour, "contour_dagrebuild_total")
	httpproxies := sumMetric(contour, "contour_httpproxy_total")
	httpproxiesValid := sumMetric(contour, "contour_httpproxy_valid_total")
	httpproxiesInvalid := sumMetric(contour, "contour_httpproxy_invalid_total")
	fmt.Fprintf(w, "  %-26s\t%s\n", "dag_rebuilds", formatNum(dagRebuilds))
	fmt.Fprintf(w, "  %-26s\ttotal=%s  valid=%s  invalid=%s\n", "httpproxies",
		formatNum(httpproxies), formatNum(httpproxiesValid), formatNum(httpproxiesInvalid))
	if fam := contour["contour_dagrebuild_seconds"]; fam != nil {
		for _, m := range fam.GetMetric() {
			s := m.GetSummary()
			if s != nil && s.GetSampleCount() > 0 {
				fmt.Fprintf(w, "  %-26s\t%.3fs total over %d rebuilds\n", "dagrebuild_time",
					s.GetSampleSum(), s.GetSampleCount())
			}
		}
	}

	section(w, "Cgroup resources")
	fmt.Fprintf(w, "  %-26s\t%-12s\t%-12s\t%s\n", "", "envoy", "contour", "echoserver")
	cgMem := getByContainer(exporter, "cgroup_memory_current_bytes")
	cgPeak := getByContainer(exporter, "cgroup_memory_peak_bytes")
	cgLimit := getByContainer(exporter, "cgroup_memory_max_bytes")
	cgThrottle := getByContainer(exporter, "cgroup_cpu_throttled_seconds_total")
	fmt.Fprintf(w, "  %-26s\t%-12s\t%-12s\t%s\n", "memory_current", fmtMB(cgMem[0]), fmtMB(cgMem[1]), fmtMB(cgMem[2]))
	fmt.Fprintf(w, "  %-26s\t%-12s\t%-12s\t%s\n", "memory_peak", fmtMB(cgPeak[0]), fmtMB(cgPeak[1]), fmtMB(cgPeak[2]))
	fmt.Fprintf(w, "  %-26s\t%-12s\t%-12s\t%s\n", "memory_limit", fmtMB(cgLimit[0]), fmtMB(cgLimit[1]), fmtMB(cgLimit[2]))
	fmt.Fprintf(w, "  %-26s\t%-12s\t%-12s\t%s\n", "cpu_throttled",
		fmt.Sprintf("%.1fs", cgThrottle[0]), fmt.Sprintf("%.1fs", cgThrottle[1]), fmt.Sprintf("%.1fs", cgThrottle[2]))

	section(w, "Memory RSS (smaps)")
	fmt.Fprintf(w, "  %-26s\t%-12s\t%-12s\t%s\n", "", "envoy", "contour", "echoserver")
	smapsRows := []struct {
		label     string
		paths     [3]string // per container
	}{
		{"binary", [3]string{"/usr/local/bin/envoy", "/bin/contour", "/echoserver"}},
		{"heap", [3]string{"[anon:tcmalloc_region_NORMAL]", "[anon: Go: heap]", "[anon: Go: heap]"}},
		{"metadata", [3]string{"[anon:tcmalloc_region_METADATA]", "[anon: Go: immortal metadata]", "[anon: Go: immortal metadata]"}},
	}
	for _, sr := range smapsRows {
		vals := make([]float64, 3)
		for i, container := range containers {
			if sr.paths[i] == "" {
				continue
			}
			ns := "projectcontour"
			if container == "echoserver" {
				ns = "default"
			}
			if fam := exporter["process_smaps_rss_bytes"]; fam != nil {
				for _, m := range fam.GetMetric() {
					if getLabelValue(m, "container") == container &&
						getLabelValue(m, "namespace") == ns &&
						getLabelValue(m, "path") == sr.paths[i] {
						vals[i] += metricValue(m)
					}
				}
			}
		}
		fmt.Fprintf(w, "  %-26s\t%-12s\t%-12s\t%s\n", sr.label, fmtMB(vals[0]), fmtMB(vals[1]), fmtMB(vals[2]))
	}

	// Echoserver
	echoRaw, err := httpGet("http://echoserver.127.0.0.1.nip.io/metrics")
	if err == nil {
		echo, err := parseMetrics(echoRaw)
		if err == nil {
			section(w, "Echoserver")
			rqTotal := sumMetric(echo, "http_requests_total")
			concurrent := sumMetric(echo, "http_concurrent_requests")
			fmt.Fprintf(w, "  %-26s\trequests=%-10s\tconcurrent=%s\n", "traffic",
				formatNum(rqTotal), formatNum(concurrent))
			if fam := echo["http_connections_by_state"]; fam != nil {
				var active, idle float64
				for _, m := range fam.GetMetric() {
					switch getLabelValue(m, "state") {
					case "active":
						active = metricValue(m)
					case "idle":
						idle = metricValue(m)
					}
				}
				fmt.Fprintf(w, "  %-26s\tactive=%s  idle=%s\n", "connections",
					formatNum(active), formatNum(idle))
			}
		}
	}

	w.Flush()
}

type clusterStats struct {
	name        string
	total       float64
	rq2xx       float64
	rq4xx       float64
	rq5xx       float64
	retries     float64
	timeouts    float64
	connectFail float64
}

// printClusterStats shows per-cluster request breakdown.
func printClusterStats(w *tabwriter.Writer, envoy map[string]*dto.MetricFamily) {
	clusters := map[string]*clusterStats{}
	collectClusterMetric(envoy, "envoy_cluster_upstream_rq_total", clusters, func(cs *clusterStats, v float64) { cs.total = v })
	collectClusterResponseCodes(envoy, clusters)
	collectClusterMetric(envoy, "envoy_cluster_upstream_rq_retry", clusters, func(cs *clusterStats, v float64) { cs.retries = v })
	collectClusterMetric(envoy, "envoy_cluster_upstream_rq_timeout", clusters, func(cs *clusterStats, v float64) { cs.timeouts = v })
	collectClusterMetric(envoy, "envoy_cluster_upstream_cx_connect_fail", clusters, func(cs *clusterStats, v float64) { cs.connectFail = v })

	sorted := make([]*clusterStats, 0, len(clusters))
	for _, cs := range clusters {
		if cs.total > 0 {
			sorted = append(sorted, cs)
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].total > sorted[j].total })

	if len(sorted) > 0 {
		w.Flush()
		fmt.Println()
		for _, cs := range sorted {
			fmt.Fprintf(w, "  %-26s\ttotal=%s  2xx=%s  4xx=%s  5xx=%s  retry=%s  timeout=%s  cx_fail=%s\n",
				cs.name,
				formatNum(cs.total), formatNum(cs.rq2xx), formatNum(cs.rq4xx), formatNum(cs.rq5xx),
				formatNum(cs.retries), formatNum(cs.timeouts), formatNum(cs.connectFail))
		}
	}
}

// collectClusterMetric aggregates a metric by cluster name (envoy_cluster_name label).
func collectClusterMetric(envoy map[string]*dto.MetricFamily, name string, clusters map[string]*clusterStats, set func(*clusterStats, float64)) {
	fam := envoy[name]
	if fam == nil {
		return
	}
	for _, m := range fam.GetMetric() {
		clusterName := getLabelValue(m, "envoy_cluster_name")
		if clusterName == "" {
			continue
		}
		cs, ok := clusters[clusterName]
		if !ok {
			cs = &clusterStats{name: clusterName}
			clusters[clusterName] = cs
		}
		set(cs, metricValue(m))
	}
}

// collectClusterResponseCodes collects 2xx/4xx/5xx per cluster from envoy_cluster_upstream_rq_xx.
func collectClusterResponseCodes(envoy map[string]*dto.MetricFamily, clusters map[string]*clusterStats) {
	fam := envoy["envoy_cluster_upstream_rq_xx"]
	if fam == nil {
		return
	}
	for _, m := range fam.GetMetric() {
		clusterName := getLabelValue(m, "envoy_cluster_name")
		if clusterName == "" {
			continue
		}
		cs, ok := clusters[clusterName]
		if !ok {
			cs = &clusterStats{name: clusterName}
			clusters[clusterName] = cs
		}
		class := getLabelValue(m, "envoy_response_code_class")
		v := metricValue(m)
		switch class {
		case "2":
			cs.rq2xx = v
		case "4":
			cs.rq4xx = v
		case "5":
			cs.rq5xx = v
		}
	}
}

// percentileFromBuckets computes a percentile from aggregated histogram buckets.
func percentileFromBuckets(buckets map[float64]uint64, totalCount uint64, percentile float64) float64 {
	if totalCount == 0 {
		return 0
	}
	target := uint64(float64(totalCount) * percentile)

	// Sort bucket boundaries
	bounds := make([]float64, 0, len(buckets))
	for b := range buckets {
		bounds = append(bounds, b)
	}
	sort.Float64s(bounds)

	for _, b := range bounds {
		if buckets[b] >= target {
			return b
		}
	}
	if len(bounds) > 0 {
		return bounds[len(bounds)-1]
	}
	return 0
}

func formatDurationMs(seconds float64) string {
	if seconds == 0 || math.IsInf(seconds, 1) {
		return "N/A"
	}
	if seconds < 1 {
		return fmt.Sprintf("%.1fms", seconds*1000)
	}
	return fmt.Sprintf("%.2fs", seconds)
}

// histPercentile computes a percentile directly from a dto.Histogram.
func histPercentile(h *dto.Histogram, p float64) float64 {
	target := uint64(float64(h.GetSampleCount()) * p)
	for _, b := range h.GetBucket() {
		if b.GetCumulativeCount() >= target {
			return b.GetUpperBound()
		}
	}
	return 0
}

// --- Helpers ---

func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return body, nil
}

func parseMetrics(data []byte) (map[string]*dto.MetricFamily, error) {
	parser := expfmt.NewTextParser(model.UTF8Validation)
	return parser.TextToMetricFamilies(bytes.NewReader(data))
}

func sumMetric(families map[string]*dto.MetricFamily, name string) float64 {
	fam := families[name]
	if fam == nil {
		return 0
	}
	var total float64
	for _, m := range fam.GetMetric() {
		total += metricValue(m)
	}
	return total
}

func getByContainer(families map[string]*dto.MetricFamily, name string) []float64 {
	out := make([]float64, len(containers))
	fam := families[name]
	if fam == nil {
		return out
	}
	for i, container := range containers {
		ns := "projectcontour"
		if container == "echoserver" {
			ns = "default"
		}
		for _, m := range fam.GetMetric() {
			if getLabelValue(m, "container") == container && getLabelValue(m, "namespace") == ns {
				out[i] += metricValue(m)
			}
		}
	}
	return out
}

func getSmaps(families map[string]*dto.MetricFamily, name, targetContainer, path string) []float64 {
	out := make([]float64, len(containers))
	fam := families[name]
	if fam == nil {
		return out
	}
	for i, container := range containers {
		if container != targetContainer {
			continue
		}
		ns := "projectcontour"
		if container == "echoserver" {
			ns = "default"
		}
		for _, m := range fam.GetMetric() {
			if getLabelValue(m, "container") == container &&
				getLabelValue(m, "namespace") == ns &&
				getLabelValue(m, "path") == path {
				out[i] += metricValue(m)
			}
		}
	}
	return out
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

func section(w *tabwriter.Writer, title string) {
	w.Flush()
	fmt.Printf("\n--- %s ---\n", title)
}

func header(w *tabwriter.Writer) {
	fmt.Fprintf(w, "  %-30s", "")
	for _, p := range pods {
		fmt.Fprintf(w, "\t%s", p)
	}
	fmt.Fprintln(w)
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
		out[i] = fmtMB(v)
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

func fmtMB(v float64) string {
	return humanize.IBytes(uint64(v))
}

func formatNum(v float64) string {
	return humanize.Comma(int64(v))
}
