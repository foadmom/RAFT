#!/bin/bash

echo "Rebuilding raft docker image"
rm ./bin/raft

echo "Removing old raft image if it exists"
sudo docker rmi -f $(sudo docker images | grep 'raft')

echo "Removing old raft containers if they exist"
sudo docker rm -f $(sudo docker ps -a | grep 'raft')

echo removing old nats containers if they exist
sudo docker rm $(sudo docker ps -a | grep 'nats')

echo "Removing network if it exists"
sudo docker network rm -fnats

# echo "Listing all docker images to confirm raft image is built"
# sudo docker images

echo "Building raft binary"
go build -o ./bin/ ./...

echo "Building raft docker image"
sudo docker build -t raft .

# echo "Create network"
# sudo docker network create --driver bridge nats

echo "run with compose"
sudo sudo docker compose up

# echo "Create nats://nats:4222 DNS entry within nats network and also reachablel via localhost:4222 (-p 4222:4222)"
# sudo docker run --hostname nats_server -p 4222:4222 -p 8222:8222 -p 6222:6222 -dit --name nats --network nats nats:latest

# echo "Running raft container in nats network with awareness of nats://nats:4222 endpoint"
# sudo docker run -it --name raft_container --network nats raft

# echo "Attaching to raft container logs"
# sudo docker attach raft


# sudo docker run --rm -it --add-host=host.docker.internal:host-gateway raft


