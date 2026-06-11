ui = false
telemetry {
  prometheus_retention_time = "60s"
  disable_hostname = true
  # How often to collect high-cardinality gauges like secret count, token count, entity count (default: 10m).
  # (Causes CPU spikes every interval, throttles bad if low CPU limit is set).
  usage_gauge_period = "5s"
}
cluster_name = "openbao"
listener "tcp" {
  address = "[::]:8200"
  cluster_address = "[::]:8201"
  tls_cert_file = "/host/certs/bao-server.pem"
  tls_key_file = "/host/certs/bao-server-key.pem"
}
storage "raft" {
  path = "/openbao/data"
  # After this many writes, clean up the raft log file to reclaim disk space (default: 8192)
  snapshot_threshold = 1000
  # How often to check if cleanup is needed (default: 120s)
  snapshot_interval = "15s"
  # Keep this many recent writes in the log so slow followers can catch up (default: 10000)
  trailing_logs = 500
  retry_join {
    leader_api_addr = "https://openbao-0.openbao-internal:8200"
    leader_ca_cert_file = "/host/certs/ca.pem"
    leader_client_cert_file = "/host/certs/bao-server.pem"
    leader_client_key_file = "/host/certs/bao-server-key.pem"
  }
  retry_join {
    leader_api_addr = "https://openbao-1.openbao-internal:8200"
    leader_ca_cert_file = "/host/certs/ca.pem"
    leader_client_cert_file = "/host/certs/bao-server.pem"
    leader_client_key_file = "/host/certs/bao-server-key.pem"
  }
  retry_join {
    leader_api_addr = "https://openbao-2.openbao-internal:8200"
    leader_ca_cert_file = "/host/certs/ca.pem"
    leader_client_cert_file = "/host/certs/bao-server.pem"
    leader_client_key_file = "/host/certs/bao-server-key.pem"
  }
}
