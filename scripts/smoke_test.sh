#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
echo "== HTTP (L7), 3 servidores =="
for server in server1 server2 server3; do
  case "$server" in server1) port=8081;; server2) port=8082;; server3) port=8083;; esac
  curl --fail --silent --show-error "http://127.0.0.1:$port/healthz"
done
echo "== Round Robin, Weighted, Least, IP Hash =="
for mode in rr weighted least iphash; do
  echo "Testando $mode:"
  curl --fail --silent --show-error "http://127.0.0.1:8080/$mode/api/visit?delay=10"
  echo
done
echo "== TCP/UDP (L4) =="
python3 scripts/l4_client.py --protocol tcp --port 9090 --count 3
python3 scripts/l4_client.py --protocol tcp --port 9092 --count 3 --concurrency 3
python3 scripts/l4_client.py --protocol tcp --port 9093 --count 3
python3 scripts/l4_client.py --protocol udp --port 9091 --count 3
echo "== Métricas via NGINX =="
curl --fail --silent --show-error http://127.0.0.1:8080/dashboard/api/stats
echo;echo "Smoke test completo (revise as métricas no painel)."
