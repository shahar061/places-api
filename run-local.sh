#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Starting Places API in Local Development Mode${NC}"

# Check if .env.local exists
if [ ! -f ".env.local" ]; then
    echo -e "${RED}❌ .env.local file not found!${NC}"
    echo -e "${YELLOW}Please create .env.local with your environment variables${NC}"
    exit 1
fi

# Start Docker services
echo -e "${BLUE}📦 Starting Docker services (NATS)...${NC}"
docker-compose -f docker-compose.services.yml up -d

# Wait for services to be ready
echo -e "${BLUE}⏳ Waiting for services to be ready...${NC}"
sleep 5

# Check NATS health
if curl -s http://localhost:8222/healthz > /dev/null; then
    echo -e "${GREEN}✅ NATS is healthy${NC}"
else
    echo -e "${RED}❌ NATS is not responding${NC}"
    exit 1
fi

# Load environment variables
echo -e "${BLUE}🔧 Loading environment variables...${NC}"
export $(cat .env.local | grep -v '^#' | xargs)

# Check if Air is installed
if command -v air &> /dev/null; then
    echo -e "${GREEN}🔥 Starting API with Air (live reload)...${NC}"
    air -c .air.toml
else
    echo -e "${YELLOW}⚠️  Air not found, starting with go run...${NC}"
    echo -e "${BLUE}💡 Install Air for live reload: go install github.com/air-verse/air@v1.52.3${NC}"
    go run . server
fi
