FROM golang:1.18.9-alpine3.17 as builder
WORKDIR /app
COPY . /app
RUN apk update
RUN apk add make git
RUN make build

FROM debian:latest as production
WORKDIR /app
RUN apt update -y && apt install ca-certificates -y
COPY --from=builder /app/dnsfun.bin /app
COPY --from=builder /app/cities15000.txt /app
COPY ./entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENTRYPOINT /app/dnsfun.bin