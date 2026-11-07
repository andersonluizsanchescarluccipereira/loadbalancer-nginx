#!/usr/bin/env bash
# Laboratório NGINX + Go para Docker Compose V2 em macOS Intel.
# Compatível com o Bash 3.2 que acompanha o macOS Big Sur.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() {
  cat <<'HELP'
Uso: bash scripts/lab.sh COMANDO [argumentos]

  doctor                           Diagnóstico do Mac e do Docker
  up                               Constrói e sobe os 5 containers
  down                             Para e remove os containers do laboratório
  ps                               Lista serviços e portas
  logs [serviço]                   Mostra últimas 80 linhas de logs
  check                            Verifica HTTP e dashboard
  test [rr|weighted|least|iphash] [qtd]
                                   Teste HTTP; padrão rr e 36 requisições
  smoke                            Testes completos de HTTP, TCP e UDP
  reset                            Zera as métricas do painel
  help                             Exibe esta ajuda

Painel: http://localhost:8080
Servidores diretos: 8081, 8082, 8083
HELP
}

# A maioria das instalações antigas de Docker Desktop no Big Sur usa Compose V2.
# Não utilizamos docker-compose v1: ele não é compatível com este compose.yaml.
resolve_compose() {
  if ! command -v docker >/dev/null 2>&1; then
    echo 'ERRO: Docker CLI não encontrado no PATH.' >&2
    echo 'O Big Sur 11 não suporta o Docker Desktop atual; veja README_MAC.md.' >&2
    exit 1
  fi
  if docker compose version >/dev/null 2>&1; then
    compose=(docker compose -f compose.yaml)
  elif command -v docker-compose >/dev/null 2>&1 && \
       docker-compose version --short 2>/dev/null | grep -Eq '^v?2\.'; then
    compose=(docker-compose -f compose.yaml)
  else
    echo 'ERRO: Docker Compose V2 não encontrado.' >&2
    echo 'Verifique "docker compose version" e o README_MAC.md.' >&2
    exit 1
  fi
}

check_daemon() {
  if ! docker info >/dev/null 2>&1; then
    echo 'ERRO: Docker Engine não está acessível.' >&2
    echo 'Se usar Docker Desktop antigo, abra o aplicativo Docker e tente novamente.' >&2
    echo 'Veja também: docker context ls' >&2
    exit 1
  fi
}

doctor() {
  echo '== Informações do Mac =='
  if [ "$(uname -s)" = Darwin ]; then
    echo "macOS: $(sw_vers -productVersion)"
    echo "Arquitetura: $(uname -m)"
    case "$(sw_vers -productVersion)" in
      11.*) echo 'AVISO: macOS Big Sur 11 não é suportado pelo Docker Desktop atual.' ;;
    esac
  else
    echo "Sistema: $(uname -s) ($(uname -m))"
  fi
  echo
  echo '== Docker CLI =='
  if command -v docker >/dev/null 2>&1; then
    docker --version || true
  else
    echo 'Não instalado ou fora do PATH.'
  fi
  echo '== Docker Compose V2 =='
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    docker compose version
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose version || true
  else
    echo 'Não encontrado.'
  fi
  echo '== Docker Engine =='
  if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
    docker info --format 'Engine: {{.ServerVersion}} | OS: {{.OperatingSystem}}' || true
  else
    echo 'Inacessível (Docker Desktop desligado ou engine não instalado).'
  fi
  echo '== Ferramentas de teste =='
  command -v curl >/dev/null 2>&1 && echo 'curl: OK' || echo 'curl: ausente'
  command -v python3 >/dev/null 2>&1 && python3 --version || echo 'Python 3: ausente'
}

check_http() {
  if ! command -v curl >/dev/null 2>&1; then
    echo 'ERRO: curl não encontrado.' >&2
    exit 1
  fi
  echo '== Requisição via NGINX (Round Robin) =='
  curl --fail --silent --show-error --max-time 10 \
    'http://127.0.0.1:8080/rr/api/visit'
  echo
  echo '== Estatísticas do monitor =='
  curl --fail --silent --show-error --max-time 10 \
    'http://127.0.0.1:8080/dashboard/api/stats'
  echo
}

case "${1:-help}" in
  help|-h|--help) usage ;;
  doctor) doctor ;;
  up)
    resolve_compose
    check_daemon
    "${compose[@]}" up -d --build
    echo
    echo 'Containers iniciados. Verifique o estado com: bash scripts/lab.sh ps'
    echo 'Painel: http://localhost:8080'
    echo 'Execute: bash scripts/lab.sh check'
    ;;
  down)
    resolve_compose
    check_daemon
    "${compose[@]}" down
    ;;
  ps)
    resolve_compose
    check_daemon
    "${compose[@]}" ps
    ;;
  logs)
    resolve_compose
    check_daemon
    shift
    "${compose[@]}" logs --tail=80 "$@"
    ;;
  check)
    check_http
    ;;
  test)
    if ! command -v python3 >/dev/null 2>&1; then
      echo 'ERRO: Python 3 não instalado. Instale-o para testar carga.' >&2
      exit 1
    fi
    mode="${2:-rr}"
    count="${3:-36}"
    case "$mode" in rr|weighted|least|iphash) ;; *) echo "Modo inválido: $mode" >&2; exit 2 ;; esac
    case "$count" in ''|*[!0-9]*) echo "Quantidade inválida: $count" >&2; exit 2 ;; esac
    if [ "$count" -le 0 ]; then echo 'Quantidade deve ser positiva.' >&2; exit 2; fi
    python3 scripts/http_load.py --mode "$mode" --count "$count"
    ;;
  smoke)
    if ! command -v python3 >/dev/null 2>&1; then
      echo 'ERRO: Python 3 é necessário para testes TCP/UDP.' >&2
      exit 1
    fi
    bash scripts/smoke_test.sh
    ;;
  reset)
    curl --fail --silent --show-error --max-time 10 \
      -X POST 'http://127.0.0.1:8080/dashboard/api/reset'
    echo 'Métricas reiniciadas.'
    ;;
  *)
    usage
    echo "Comando desconhecido: $1" >&2
    exit 2
    ;;
esac
