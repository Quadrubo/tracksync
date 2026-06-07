# Client
run-client *args:
    cd tracksync && go run . {{ args }}

run-client-nix *args:
    nix run . -- {{ args }}

# Server
run-server:
    cd server && go run . -env-file .env

run-server-docker:
    cd server && docker compose up --build

clear-server-data:
    rm -f server/data/state.db server/data/state.db-shm server/data/state.db-wal

# Maintenance
update-vendor-hash:
    bash scripts/update-vendor-hash.sh


dangerously-clear-system-client-data:
    rm -f ~/.local/share/tracksync/state.db ~/.local/share/tracksync/state.db-shm ~/.local/share/tracksync/state.db-wal
