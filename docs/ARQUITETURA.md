# Análise arquitetural e trade-offs do laboratório

## Objetivos de aprendizagem cobertos

| Item apresentado no material de referência | Evidência neste laboratório |
|---|---|
| Escalabilidade vertical e horizontal | 3 réplicas Go; comparação conceitual e testes de carga |
| Load balancer na prática | Arquivos `nginx/nginx.conf` e `nginx/proxy-common.conf` |
| Muitos usuários simultâneos | `http_load.py --count 300 --concurrency 30` e gráfico SSE |
| Diferença entre L4 e L7 | `http {}` versus `stream {}`; HTTP :8080, TCP/UDP :909x |
| Modelo OSI e balanceamento | L4 = Transporte; L7 = Aplicação |
| Quando usar L4 e L7 | Matriz de decisão no README, seção 8 |
| Round Robin | `upstream http_rr`, `upstream tcp_rr` e `upstream udp_rr` |
| Weighted Round Robin | `http_weighted` e `tcp_weighted`, pesos 5:3:1 |
| Least Connections | `http_least` e `tcp_least`, testes com delays e concorrência |
| HTTP, TCP e UDP | API HTTP, socket TCP e datagramas UDP em todos os 3 backends |
| Balanceadores de aplicações web | NGINX reverse proxy, SSE, REST API de monitoramento |
| Configuração NGINX prática | Dockerfile, upstreams, proxy headers, timeouts e access logs |
| Distribuir requests em múltiplas instâncias | Respostas reais `server1:8081`, `server2:8082`, `server3:8083` |

## Caminho exato de uma requisição HTTP

1. Navegador ou cliente envia `GET :8080/weighted/api/visit?delay=100`.
2. NGINX encontra o `location /weighted/api/`, reescreve URI para `/api/visit` e encaminha ao `upstream http_weighted`.
3. NGINX seleciona um backend segundo os pesos (5:3:1).
4. O backend Go recebe `X-Lab-Mode: weighted`, registra `start` no monitor e simula tempo de processamento.
5. O backend responde JSON `{server, port, mode, delayMs, ...}` e registra `finish`.
6. O monitor contabiliza **um request**, remove o in-flight no evento `finish` e notifica os assinantes SSE.
7. O navegador recebe o SSE na mesma porta `:8080` por meio de `GET /dashboard/api/events`, atualiza cartões, percentual, timeline e tabela.

**Observabilidade não confunde seleção com visualização:** o método de escolha está dentro do NGINX. O Go não escolhe seu próprio servidor; apenas relata que recebeu o request. O monitor não influencia o balanceamento e está fora do caminho crítico de resposta (a telemetria é enfileirada assíncronamente).

## Caminho L4 TCP

1. Cliente abre **nova conexão TCP** em `localhost:9092`.
2. `stream { server { listen 9092; proxy_pass tcp_least; } }` seleciona um backend por conexões ativas.
3. Backend escuta em `:9000` **dentro do próprio container**, lê a linha enviada e retorna `serverN port=808N`.
4. Backend registra eventos de início/fim, que alimentam o mesmo painel HTTP.

Em L4 o conceito principal é **conexão TCP**, não request HTTP. Uma conexão com muitas mensagens é tratada como uma conexão. O cliente de demonstração abre uma conexão por mensagem deliberadamente.

## Caminho L4 UDP

1. Cliente cria um socket UDP, envia **um datagrama** a `:9091/udp`.
2. NGINX `stream` encaminha ao backend selecionado via `udp_rr` (`serverN:9001`).
3. Backend retorna um datagrama com sua identidade.
4. `proxy_responses 1` indica uma resposta esperada na sessão UDP do NGINX.

UDP não cria uma conexão TCP, mas o NGINX mantém estado de fluxo/pseudossessão enquanto processa datagramas. Isso explica por que testes UDP com o mesmo socket e testes com novos sockets podem produzir padrões diferentes.

## Cuidados de medição

- **Contadores** são registrados a partir de eventos `start` recebidos. A conclusão (`finish`) não soma novamente.
- **In-flight** representa requests em andamento no backend relatados ao monitor; não é o total de conexões internas do NGINX. O monitor e a rede introduzem atraso. Não confundir com métrica nativa de conexões do NGINX.
- **Métricas de latência** no cliente Python são de ponta a ponta, incluindo overhead de rede; as respostas Go exibem duração do processamento do backend, sem o tempo total de fila e proxy.
- **RR/Weighted** compartilham as três instâncias mas têm `upstream` separados, então os algoritmos possuem estado próprio.
- **Least Connections** toma decisões sobre conexões ativas de upstream, não CPU, filas, fila de requests, uso de memória, latência média ou tamanho da resposta.
- **Falsos resultados de IP Hash:** os testes partem de um único IP. NAT ou proxy podem agrupar milhares de usuários sob o mesmo IP.
- **Não há persistência:** ao reiniciar o monitor, as métricas desaparecem. O propósito é educacional, sem dependência de Redis, Kafka ou banco de dados.
- **Não há autoprovisionamento:** mudar de 3 para 4 réplicas requer atualizar Compose e NGINX. Containers não equivalem a disponibilidade independente se todos rodam no mesmo notebook.

## Trade-offs para discussão em entrevista de System Design

| Decisão | Benefício | Limitação |
|---|---|---|
| 3 réplicas stateless Go | Fácil observar expansão horizontal e tolerância de software | Um host único ainda é ponto único de falha físico |
| NGINX como proxy | Algoritmos reais e configuração bem documentada | Introduz hop de rede e pode ser gargalo ou SPOF |
| SSE para telemetria | Atualizações unidirecionais simples no navegador | Não é ferramenta completa de métricas, não persiste e pode perder eventos se atingir limites |
| Monitor fora do caminho crítico | Falha do monitor não impede requests Go | Métricas podem ficar incompletas ou atrasadas |
| Round Robin | Simples e previsível | Não mede trabalho por request |
| Weighted Round Robin | Pode representar capacidade desigual | Pesos precisam ser ajustados/operados |
| Least Connections | Melhor quando a duração da conexão varia | Conexões não representam diretamente CPU/latência |
| IP Hash | Afinidade por IP | NAT e proxy podem causar desequilíbrio |
| L4 TCP/UDP | Balanceia protocolos não HTTP | Não enxerga paths ou headers HTTP |
| L7 HTTP | Decisão por URL/host/cabeçalhos, fácil observação | Trabalho de parsing/proxy da camada de aplicação |

## Próximos incrementos sugeridos

1. NGINX `stub_status`, Prometheus/Grafana e OpenTelemetry para complementar a telemetria didática (RPS, latência, erros, percentis).
2. Simular perda de instâncias, timeout e `502`, documentando RTO, readiness e failover passivo.
3. Implementar sessões externas em Redis/Postgres e demonstrar por que sticky sessions nem sempre bastam.
4. Comparar L4/L7 sob diferentes tamanhos de payload e durações de conexão usando medição controlada.
5. Quando dominar este lab, avaliar HAProxy/Envoy e Kubernetes (Service, Ingress/Gateway API, HPA).
