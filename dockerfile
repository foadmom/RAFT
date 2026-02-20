FROM ubuntu:24.04

WORKDIR /app

COPY ./bin/RAFT .

# EXTRA_HOSTS: "nats-server:host-gateway"

ENTRYPOINT ["/app/RAFT"]
# CMD ["/app/RAFT"]

