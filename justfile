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

# Maintenance
update-vendor-hash:
    bash scripts/update-vendor-hash.sh

clear-local-data:
    rm ~/.local/share/tracksync/state.db
