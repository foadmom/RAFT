FROM ubuntu:24.04
# FROM alpine:latest

ENV NATS_SERVER 2.12.5

WORKDIR /app

COPY ./bin/RAFT .
COPY ./config.json .

# EXTRA_HOSTS: "nats-server:host-gateway"

ENTRYPOINT ["/app/RAFT"]
# CMD ["/app/RAFT"]

