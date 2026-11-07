# Guia de execução no MacBook Pro Intel — macOS 11.7.11

Este é o mesmo laboratório de **NGINX L7/L4 + 3 backends Go + monitor SSE** usado no Linux Mint, adaptado para o **Docker Compose V2**. Nada exige compilar Go localmente.

## Compatibilidade do Docker Desktop com Big Sur

**Importante (novembro/2026):** versões atuais do Docker Desktop exigem macOS mais recente e **não são compatíveis com Big Sur 11**. A geração **Docker Desktop 4.24.x** ainda atendia esse sistema em 2023; no lançamento 4.25 o requisito mínimo passou a ser Monterey 12. Não há distribuição oficial atual dos instaladores antigos (fora da janela de seis meses), nem correções de segurança para eles. Não indicamos fontes de terceiros para instaladores desatualizados.

- **Se já houver Docker Desktop antigo funcionando:** use-o, sem atualizar para versões incompatíveis. Com acesso à internet, faça `docker info` e `docker compose version` para verificar se a CLI e o Engine respondem.
- **Se não houver Docker funcional:** o script **não instala o Engine**. O caminho mais seguro para o laboratório é rodar a versão Docker/Podman no seu Linux Mint, ou usar um host Linux com Docker Engine remoto via SSH (nesse caso as portas 8080–9093 são publicadas no host remoto, não no Mac, e exigem acesso ou redirecionamento por túnel).
- **Colima atual não resolve** diretamente: exige macOS 13+ segundo a documentação oficial.

Fontes oficiais: https://docs.docker.com/desktop/release-notes/ , https://docs.docker.com/desktop/troubleshoot-and-support/faqs/releases/ , https://github.com/abiosoft/colima/blob/main/docs/FAQ.md .

## Passo 1 — diagnóstico (Bash 3.2 nativo do macOS é suficiente)

```bash
cd laboratorio-balanceamento-nginx-go-docker-mac
bash scripts/lab.sh doctor
# Você precisa conseguir executar:
docker info
docker compose version
```

Se o Engine estiver parado e o Docker Desktop já estiver instalado, abra-o pelo Dock/Applications (`open -a Docker`) e só prossiga quando `docker info` responder.

Para um Mac Intel de **8 GB**, sugere-se configurar em Docker Desktop > Settings > Resources aproximadamente **2 CPUs e 2 GB de RAM**, com Kubernetes desativado. Você pode precisar aumentar memória caso algum build não complete. Prefira baixo nível de concorrência ao experimentar.

## Passo 2 — subir serviços

```bash
bash scripts/lab.sh up
bash scripts/lab.sh ps
bash scripts/lab.sh check
```

Dashboard: **http://localhost:8080**.

Serviços: `server1 :8081`, `server2 :8082`, `server3 :8083`, `nginx :8080`, `monitor :7070` apenas na rede interna (o dashboard é servido pela porta 8080). São cinco containers.

## Passo 3 — testes HTTP com dashboard aberto

Abra `http://localhost:8080` e use os controles gráficos; ou gere tráfego pelo terminal:

```bash
bash scripts/lab.sh test rr 36          # Round Robin aproximadamente 12/12/12
bash scripts/lab.sh reset               # limpar dados antes de trocar algoritmo
bash scripts/lab.sh test weighted 90    # pesos 5:3:1, aproximadamente 50/30/10
bash scripts/lab.sh reset
bash scripts/lab.sh test least 48       # não garante desequilíbrio sem carga concorrente
bash scripts/lab.sh reset
bash scripts/lab.sh test iphash 18      # chamadas mesmo IP tendem ao mesmo servidor
```

Para evidenciar Least Connections, use a simulação de `server1` lento no painel, ou:

```bash
python3 scripts/http_load.py --mode least --count 48 --concurrency 12 --slow-server1
```

O `python3` deve estar instalado no Mac para os testes de carga. Se `python3 --version` falhar, instale uma versão compatível com seu macOS por um fornecedor confiável. A interface web pode gerar tráfego sem Python.

## Passo 4 — camada 4 (TCP e UDP)

```bash
python3 scripts/l4_client.py --protocol tcp --port 9090 --count 18
python3 scripts/l4_client.py --protocol tcp --port 9092 --count 24 --concurrency 8 --delay 1800
python3 scripts/l4_client.py --protocol tcp --port 9093 --count 90
python3 scripts/l4_client.py --protocol udp --port 9091 --count 18
```

Os eventos também aparecem na timeline do dashboard. A execução e o encaminhamento UDP podem variar conforme a versão antiga do Docker Desktop; monitore os logs se houver timeout.

## Operação, logs e encerramento

```bash
bash scripts/lab.sh logs nginx
bash scripts/lab.sh logs server1
bash scripts/lab.sh smoke
bash scripts/lab.sh down
```

### Dicas de diagnóstico no macOS

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
lsof -nP -iTCP:8081 -sTCP:LISTEN
docker context ls
docker compose -f compose.yaml ps
```

Se surgir `Cannot connect to the Docker daemon`, verifique que o Docker Engine está realmente acessível. Se houver `port is already allocated`, libere a porta ou altere `compose.yaml`. Se ocorrer `502 Bad Gateway`, veja logs de `nginx` e dos `serverN`; o processo Go pode ainda estar inicializando.

**Segurança:** laboratório local para aprendizado, com portas publicadas apenas em `127.0.0.1`, sem autenticação no dashboard ou métricas. Não exponha a internet ou LAN.

Consulte o [README.md](README.md) para documentação completa sobre Round Robin, Weighted, Least Connections, IP Hash, protocolos e System Design.
