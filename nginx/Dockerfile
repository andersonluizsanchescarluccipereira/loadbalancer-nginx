# Pacotes nginx e nginx-mod-stream da MESMA versão do Alpine
FROM docker.io/library/alpine:3.22
RUN apk add --no-cache nginx nginx-mod-stream && mkdir -p /run/nginx
COPY nginx.conf /etc/nginx/nginx.conf
COPY proxy-common.conf /etc/nginx/proxy-common.conf
EXPOSE 8080 9090 9091/udp 9092 9093
CMD ["nginx", "-g", "daemon off;"]
