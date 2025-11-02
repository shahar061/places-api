#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🛑 Stopping Places API Local Development${NC}"

# Stop Docker services
echo -e "${BLUE}📦 Stopping Docker services...${NC}"
docker-compose -f docker-compose.services.yml down

echo -e "${GREEN}✅ All services stopped${NC}"
