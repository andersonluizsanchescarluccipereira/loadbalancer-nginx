# Evidências de validação — 07/11/2026

## O que foi realmente executado

- `go test ./...`: **passou**.
- `go test -race ./...`: **passou** (sem relato de race em testes unitários).
- `go build`: **passou**.
- `node --check service/web/app.js`: **passou**.
- Arquivo `compose.yaml` analisado com parser YAML: **cinco serviços** e publicações de portas corretas.
- NGINX HTTP com as configurações L7 do projeto, Go e monitor executados localmente no mesmo namespace de rede: **passou**. Para este teste, os hosts de container foram substituídos por `127.0.0.1` e a porta frontal passou temporariamente a `18080`; isso foi necessário porque o ambiente de testes não inclui Docker Engine/Compose.
- TCP e UDP: instâncias Go respondendo em portas locais separadas; **passou**. Para esta validação, as portas TCP/UDP internas foram remapeadas usando `TCP_PORT`/`UDP_PORT` em processos locais independentes, sem NGINX L4.
- SSE/contagem do monitor: HTTP monitor recebeu eventos de início e fim, acumulando 195 requisições no teste completo; **passou**. Também foi verificada separadamente uma conexão `text/event-stream` que recebeu em tempo real o evento `sse-unique` após `POST /internal/event` (`202 Accepted`).

## Distribuição realmente observada na validação HTTP NGINX

| Algoritmo | Número | server1 | server2 | server3 |
|---|---:|---:|---:|---:|
| Round Robin | 36 | 12 | 12 | 12 |
| Weighted Round Robin (5:3:1) | 90 | 50 | 30 | 10 |
| Least Connections, server1 lento | 48 | 4 | 22 | 22 |
| IP Hash com uma origem | 15 | 0 | 0 | 15 |

O resultado IP Hash não é garantia de que sempre será o `server3`; é apenas a instância selecionada nesta execução. Os números Least Connections mudam com a concorrência, o tempo de processamento e a ordem dos requests.

## O que depende da execução no Mac com Docker funcionando

**Não foi possível executar `docker compose up` nem testar a proxy L4 NGINX no runtime de elaboração, porque o Docker Engine não está instalado ali.** A sintaxe HTTP do NGINX passou no binário local; a sintaxe do bloco `stream` segue a documentação do NGINX Open Source e o caminho do módulo `ngx_stream_module.so` foi verificado na lista de arquivos do pacote Alpine v3.22.

Quando iniciar o ambiente com Docker, valide o projeto integralmente:

```bash
bash scripts/lab.sh up
bash scripts/lab.sh ps
bash scripts/smoke_test.sh
```

Dessa forma, a validação L4 ocorrerá pela implementação efetiva do NGINX e não apenas diretamente nos servidores Go.
