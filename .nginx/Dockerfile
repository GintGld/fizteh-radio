FROM alpine:latest

RUN apk add --update nginx nginx-mod-rtmp

EXPOSE 80
EXPOSE 443
EXPOSE 1935

CMD ["nginx", "-g", "daemon off;"]