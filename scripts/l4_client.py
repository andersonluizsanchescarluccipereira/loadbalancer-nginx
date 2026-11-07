#!/usr/bin/env python3
"""Cliente didático de transporte L4: TCP (conexões) ou UDP (datagramas).

A porta informada escolhe o upstream do NGINX, não o texto 'mode'. O campo
'mode' no payload serve exclusivamente para rotular a telemetria do painel.
"""
import argparse
import collections
import concurrent.futures
import re
import socket
import time
import uuid

MODES = {9090: 'tcp-rr', 9092: 'tcp-least', 9093: 'tcp-weighted', 9091: 'udp-rr'}


def send_one(args, index):
    request_id = 'cli-' + uuid.uuid4().hex[:16]
    mode = MODES.get(args.port, args.protocol + '-rr')
    msg = f'id={request_id} mode={mode} delay={args.delay}\n'.encode()
    start = time.perf_counter()
    try:
        if args.protocol == 'tcp':
            with socket.create_connection((args.host, args.port), timeout=args.timeout) as sock:
                sock.settimeout(args.timeout)
                sock.sendall(msg)
                response = sock.recv(1024).decode(errors='replace').strip()
        else:
            with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
                sock.settimeout(args.timeout)
                sock.sendto(msg, (args.host, args.port))
                response, _ = sock.recvfrom(1024)
                response = response.decode(errors='replace').strip()
        found = re.search(r'\bserver=(server[123])\b', response)
        if not found:
            raise RuntimeError('resposta sem identificação válida: ' + response)
        server = found.group(1)
        latency = (time.perf_counter() - start) * 1000
        return index, server, latency, response, None
    except (OSError, RuntimeError) as exc:
        return index, None, 0, '', str(exc)


def main():
    parser = argparse.ArgumentParser(description='Gera conexões TCP ou datagramas UDP no NGINX')
    parser.add_argument('--protocol', choices=['tcp', 'udp'], default='tcp')
    parser.add_argument('--host', default='127.0.0.1')
    parser.add_argument('--port', type=int, default=9090)
    parser.add_argument('--count', type=int, default=18)
    parser.add_argument('--concurrency', type=int, default=1)
    parser.add_argument('--delay', type=int, default=0, help='delay em ms; somente TCP')
    parser.add_argument('--timeout', type=float, default=15)
    args = parser.parse_args()
    if not 1 <= args.count <= 10000 or not 1 <= args.concurrency <= 100:
        parser.error('count deve estar entre 1 e 10000, concurrency entre 1 e 100')
    if not 0 <= args.delay <= 10000:
        parser.error('delay deve estar entre 0 e 10000')
    if args.protocol == 'udp' and args.delay:
        parser.error('delay é suportado apenas no exemplo TCP')
    if args.protocol == 'udp' and args.port == 9090:
        parser.error('porta 9090 é TCP; use --port 9091 para UDP')
    results = []
    started = time.perf_counter()
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as pool:
        futures = [pool.submit(send_one, args, n) for n in range(1, args.count + 1)]
        for future in concurrent.futures.as_completed(futures):
            result = future.result()
            results.append(result)
            seq, server, latency, response, error = result
            if error:
                print(f'[{seq:03d}] ERRO: {error}')
            elif args.count <= 40:
                print(f'[{seq:03d}] {server} | {latency:.1f} ms | {response}')
    summary = collections.Counter(row[1] for row in results if row[1])
    print('\n--- DISTRIBUIÇÃO REAL ---')
    for server in ('server1', 'server2', 'server3'):
        print(f'{server}: {summary[server]:>5} ({100 * summary[server] / args.count:5.1f}%)')
    print(f'falhas: {sum(row[1] is None for row in results)} | tempo total: {time.perf_counter() - started:.2f} s')
    print('Veja também o painel http://localhost:8080 e selecione o filtro L4 correspondente.')


if __name__ == '__main__':
    main()
