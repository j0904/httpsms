#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TARGET_VM="${TARGET_VM:-s1001.bigt.ai}"
TARGET_IP="${TARGET_IP:-85.214.37.95}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/oraclevpc.key}"
SSH_USER="${SSH_USER:-root}"
DOMAIN="${DOMAIN:-httpsms.bigt.ai}"
REGISTRY="${REGISTRY:-ghcr.io/j0904/httpsms}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
API_IMAGE="${REGISTRY}-api:${IMAGE_TAG}"
WEB_IMAGE="${REGISTRY}-web:${IMAGE_TAG}"

unset SSH_AUTH_SOCK
unset SSH_ASKPASS_REQUIRE
export DISPLAY=none
export GIT_TERMINAL_PROMPT=0
export GCM_INTERACTIVE=never

SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=10 -o BatchMode=yes -o IdentitiesOnly=yes"
REMOTE="${SSH_USER}@${TARGET_IP}"
REMOTE_DIR="${REMOTE_DIR:-/root/httpsms}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

if [ ! -f "${SSH_KEY}" ]; then
  echo "Error: SSH key not found at ${SSH_KEY}"
  exit 1
fi

test_ssh() {
  echo "Testing SSH to ${REMOTE}..."
  ssh ${SSH_OPTS} -i ${SSH_KEY} ${REMOTE} "echo OK" 2>/dev/null && echo "SSH OK" || { echo "SSH failed"; exit 1; }
}

ensure_networks() {
  echo -e "${YELLOW}Ensuring Docker networks...${NC}"
  ssh -i ${SSH_KEY} ${SSH_OPTS} ${REMOTE} \
    "sudo docker network inspect caddy_net >/dev/null 2>&1 || sudo docker network create caddy_net --driver bridge --subnet 172.20.6.0/24" 2>&1
}

sync_env() {
  echo -e "${YELLOW}Syncing .env files...${NC}"
  ssh -i ${SSH_KEY} ${SSH_OPTS} ${REMOTE} "mkdir -p ${REMOTE_DIR}"
  scp -i ${SSH_KEY} ${SSH_OPTS} "${PROJECT_DIR}/api/.env" "${REMOTE}:${REMOTE_DIR}/api.env"
  scp -i ${SSH_KEY} ${SSH_OPTS} "${PROJECT_DIR}/web/.env" "${REMOTE}:${REMOTE_DIR}/web.env"
}

write_compose() {
  echo -e "${YELLOW}Writing docker-compose.yml...${NC}"
  ssh -i ${SSH_KEY} ${SSH_OPTS} ${REMOTE} "cat > ${REMOTE_DIR}/docker-compose.yml" << COMPOSE
services:
  postgres:
    image: postgres:alpine
    container_name: httpsms-db
    environment:
      POSTGRES_DB: httpsms
      POSTGRES_PASSWORD: dbpassword
      POSTGRES_USER: dbusername
    volumes:
      - httpsms_postgres:/var/lib/postgresql
    restart: on-failure
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U dbusername -d httpsms"]
      interval: 30s
      timeout: 60s
      retries: 5
      start_period: 10s
    networks:
      - httpsms_net

  redis:
    image: redis:latest
    container_name: httpsms-redis
    command: redis-server
    volumes:
      - httpsms_redis:/var/lib/redis
    restart: on-failure
    networks:
      - httpsms_net

  api:
    image: ${API_IMAGE}
    container_name: httpsms-api
    env_file:
      - /root/httpsms/api.env
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
    restart: unless-stopped
    networks:
      - httpsms_net
      - caddy_net

  web:
    image: ${WEB_IMAGE}
    container_name: httpsms-web
    restart: unless-stopped
    networks:
      - httpsms_net
      - caddy_net

volumes:
  httpsms_postgres:
  httpsms_redis:

networks:
  httpsms_net:
    driver: bridge
  caddy_net:
    external: true
COMPOSE
}

start() {
  echo -e "${YELLOW}Pulling and starting containers on server...${NC}"
  ssh -i ${SSH_KEY} ${SSH_OPTS} ${REMOTE} << ENDSSH
    cd ${REMOTE_DIR}
    sudo docker compose pull
    sudo docker compose up -d --remove-orphans
    sleep 5
    sudo docker ps --filter "name=httpsms-"
ENDSSH
}

configure_caddy() {
  echo -e "${YELLOW}Configuring Caddy for ${DOMAIN}...${NC}"
  ssh -i ${SSH_KEY} ${SSH_OPTS} ${REMOTE} << ENDSSH
    API_IP=\$(sudo docker inspect httpsms-api --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{" "}}{{end}}' 2>/dev/null | grep -oE '172\.20\.6\.[0-9]+' | head -1)
    WEB_IP=\$(sudo docker inspect httpsms-web --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{" "}}{{end}}' 2>/dev/null | grep -oE '172\.20\.6\.[0-9]+' | head -1)

    if [ -z "\$API_IP" ] || [ -z "\$WEB_IP" ]; then
      echo "Error: Cannot get container IPs. Are they running?"
      exit 1
    fi

    echo "API container IP: \$API_IP"
    echo "Web container IP: \$WEB_IP"

    sudo tee /etc/caddy/Caddyfile.d/httpsms.caddy > /dev/null << CADDYEOF
httpsms.bigt.ai {
    handle /health {
        reverse_proxy \${API_IP}:8000
    }
    handle_path /api/* {
        reverse_proxy \${API_IP}:8000
    }
    handle /v1/* {
        reverse_proxy \${API_IP}:8000
    }
    handle {
        reverse_proxy \${WEB_IP}:3000
    }
}
CADDYEOF

    # Reboot-safe: re-sync static-IP backends from live Docker IPs
    sudo tee /usr/local/sbin/httpsms-sync-caddy-ips > /dev/null << 'SYNCEOF'
#!/bin/bash
set -euo pipefail
VHOST=/etc/caddy/Caddyfile.d/httpsms.caddy
API_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' httpsms-api 2>/dev/null | tr -s ' \\t' '\\n' | grep -E '^172\\.20\\.6\\.' | head -1 || true)
WEB_IP=\$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' httpsms-web 2>/dev/null | tr -s ' \\t' '\\n' | grep -E '^172\\.20\\.6\\.' | head -1 || true)
[ -n "\${API_IP:-}" ] && [ -n "\${WEB_IP:-}" ] || exit 0
NEED=0
if [ -f "\$VHOST" ]; then
  grep -q "reverse_proxy \${API_IP}:8000" "\$VHOST" || NEED=1
  grep -q "reverse_proxy \${WEB_IP}:3000" "\$VHOST" || NEED=1
else
  NEED=1
fi
[ "\$NEED" = 1 ] || exit 0
cat > "\$VHOST" << VHOSTEOF
httpsms.bigt.ai {
    handle /health {
        reverse_proxy \${API_IP}:8000
    }
    handle_path /api/* {
        reverse_proxy \${API_IP}:8000
    }
    handle /v1/* {
        reverse_proxy \${API_IP}:8000
    }
    handle {
        reverse_proxy \${WEB_IP}:3000
    }
}
VHOSTEOF
systemctl reload caddy
logger -t httpsms-sync-caddy "updated vhost api=\${API_IP} web=\${WEB_IP}"
SYNCEOF
    sudo chmod 755 /usr/local/sbin/httpsms-sync-caddy-ips

    sudo tee /etc/systemd/system/httpsms-caddy-sync.service > /dev/null << 'UNITEOF'
[Unit]
Description=Sync httpsms Caddy backend IPs from Docker
After=docker.service caddy.service
Requires=docker.service
Wants=caddy.service

[Service]
Type=oneshot
ExecStart=/usr/local/sbin/httpsms-sync-caddy-ips
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
UNITEOF

    sudo tee /etc/systemd/system/httpsms-caddy-sync.timer > /dev/null << 'TIMEREOF'
[Unit]
Description=Periodically sync httpsms Caddy backend IPs

[Timer]
OnBootSec=15s
OnUnitActiveSec=30s
AccuracySec=5s

[Install]
WantedBy=timers.target
TIMEREOF

    sudo systemctl daemon-reload
    sudo systemctl enable --now httpsms-caddy-sync.service
    sudo systemctl enable --now httpsms-caddy-sync.timer

    sudo systemctl reload caddy
ENDSSH
}

build_and_push() {
  echo -e "${YELLOW}Building and pushing images to ${REGISTRY}...${NC}"

  echo "Building API image..."
  cd "${PROJECT_DIR}/api"
  docker build -t "${API_IMAGE}" .
  docker push "${API_IMAGE}"

  echo "Building Web image..."
  cd "${PROJECT_DIR}/web"
  docker build -t "${WEB_IMAGE}" .
  docker push "${WEB_IMAGE}"

  echo -e "${GREEN}Done: ${API_IMAGE} / ${WEB_IMAGE}${NC}"
}

init_db() {
  echo -e "${YELLOW}=== PostgreSQL Setup ===${NC}"
  echo "The database will be created automatically on first start."
  echo ""
  echo "After first deploy, create the system user:"
  echo "  docker exec -i httpsms-db psql -U dbusername -d httpsms << SQL"
  echo "  INSERT INTO users (id, api_key, email)"
  echo "  VALUES ('system-user-id', 'system-user-api-key', 'system@httpsms.bigt.ai');"
  echo "  SQL"
  echo ""
  echo "Then set these in api.env:"
  echo "  EVENTS_QUEUE_USER_ID=system-user-id"
  echo "  EVENTS_QUEUE_USER_API_KEY=system-user-api-key"
  echo ""
  echo "Run '$0 deploy' again after updating api.env to restart with new config."
}

status() {
  echo -e "${GREEN}=== httpsms Status ===${NC}"
  ssh -i ${SSH_KEY} ${SSH_OPTS} ${REMOTE} << 'ENDSSH'
    echo "--- Containers ---"
    sudo docker ps --filter "name=httpsms-" --format "table {{.Names}}\t{{.Status}}"
    echo ""
    echo "--- Caddy ---"
    cat /etc/caddy/Caddyfile.d/httpsms.caddy 2>/dev/null || echo "No Caddy config"
    echo ""
    sudo systemctl is-active caddy && echo "Caddy: active" || echo "Caddy: inactive"
ENDSSH
}

cleanup() {
  echo -e "${YELLOW}Stopping and removing httpsms containers...${NC}"
  ssh -i ${SSH_KEY} ${SSH_OPTS} ${REMOTE} << ENDSSH
    cd ${REMOTE_DIR} 2>/dev/null
    sudo docker compose down -v 2>/dev/null || true
    sudo docker rm -f httpsms-api httpsms-web httpsms-db httpsms-redis 2>/dev/null || true
    sudo rm -f /etc/caddy/Caddyfile.d/httpsms.caddy
    sudo systemctl reload caddy 2>/dev/null || true
ENDSSH
  echo -e "${GREEN}Cleanup done.${NC}"
}

main() {
  local ACTION="${1:-deploy}"

  echo -e "${GREEN}=== httpsms Deployment ===${NC}"
  echo -e "Target: ${YELLOW}${TARGET_VM} (${TARGET_IP})${NC}"
  echo -e "Domain: ${YELLOW}${DOMAIN}${NC}"
  echo -e "Images: ${YELLOW}${API_IMAGE} / ${WEB_IMAGE}${NC}"

  case "${ACTION}" in
    build)
      build_and_push
      ;;
    deploy)
      test_ssh
      ensure_networks
      sync_env
      write_compose
      start
      configure_caddy
      init_db
      echo -e "\n${GREEN}Deployed! Frontend: https://${DOMAIN}, API: https://${DOMAIN}/v1/${NC}"
      ;;
    redeploy)
      test_ssh
      start
      echo -e "${GREEN}Containers updated.${NC}"
      ;;
    status)
      status
      ;;
    caddy)
      configure_caddy
      echo -e "${GREEN}Caddy config updated.${NC}"
      ;;
    cleanup)
      cleanup
      ;;
    *)
      echo "Usage: $0 {build|deploy|redeploy|status|caddy|cleanup}"
      exit 1
      ;;
  esac
}

main "$@"
