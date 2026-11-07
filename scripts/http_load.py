#!/usr/bin/env python3
"""Carga HTTP sem dependências extras. Usa a porta 8080 e resume a distribuição."""
import argparse
import collections
import concurrent.futures
import json
import statistics
import time
import urllib.error
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser(description='Dispara e contabiliza requests HTTP via NGINX')
    parser.add_argument('--mode', choices=['rr', 'weighted', 'least', 'iphash'], default='rr')
    parser.add_argument('--count', type=int, default=36)
    parser.add_argument('--concurrency', type=int, default=6)
    parser.add_argument('--delay', type=int, default=100)
    parser.add_argument('--slow-server1', action='store_true')
    parser.add_argument('--host', default='127.0.0.1')
    args = parser.parse_args()
    if not 1 <= args.count <= 10000 or not 1 <= args.concurrency <= 100:
        parser.error('count entre 1 e 10000; concurrency entre 1 e 100')
    if not 0 <= args.delay <= 10000:
        parser.error('delay entre 0 e 10000')
    query = 'slow=server1&slow_ms=3500&fast_ms=70' if args.slow_server1 else f'delay={args.delay}'
    url = f'http://{args.host}:8080/{args.mode}/api/visit?{query}'

    def send(_):
        started = time.perf_counter()
        req = urllib.request.Request(url, headers={'X-Request-ID': 'python-' + uuid.uuid4().hex})
        try:
            with urllib.request.urlopen(req, timeout=20) as response:
                payload = json.load(response)
            return payload['server'], (time.perf_counter() - started) * 1000, None
        except (OSError, ValueError, KeyError) as err:
            return None, 0, str(err)

    started = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as pool:
        rows = list(pool.map(send, range(args.count)))
    count = collections.Counter(server for server, _, _ in rows if server)
    durations = sorted(t for server, t, _ in rows if server)
    print(f'\nHTTP {args.mode.upper()} | total={args.count} | concorrência={args.concurrency}')
    for server in ('server1', 'server2', 'server3'):
        print(f'{server}: {count[server]:>5} ({100 * count[server] / args.count:5.1f}%)')
    if durations:
        p95 = durations[max(0, int(0.95 * len(durations)) - 1)]
        print(f'latência média={statistics.mean(durations):.1f}ms | P95 amostral={p95:.1f}ms')
    print(f'falhas={sum(server is None for server, _, _ in rows)} | tempo de carga={time.perf_counter() - started:.2f}s')
    print('Observe o gráfico em http://localhost:8080')


if __name__ == '__main__':
    main()
