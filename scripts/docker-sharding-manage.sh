#!/bin/bash

# Docker Database Sharding Management Script
# This script manages the database sharding Docker services

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to display usage
usage() {
    echo -e "${BLUE}🐳 Docker Database Sharding Management${NC}"
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  start     - Start all sharding services"
    echo "  stop      - Stop all sharding services"
    echo "  restart   - Restart all sharding services"
    echo "  status    - Show status of all services"
    echo "  logs      - Show logs from all services"
    echo "  clean     - Clean up all containers and volumes"
    echo "  scale     - Scale specific shard (usage: scale <shard> <count>)"
    echo "  health    - Check health of all shards"
    echo "  stats     - Show statistics of all shards"
    echo ""
    echo "Examples:"
    echo "  $0 start"
    echo "  $0 status"
    echo "  $0 scale postgres-shard-1 2"
    echo "  $0 health"
}

# Function to start services
start_services() {
    echo -e "${YELLOW}🚀 Starting database sharding services...${NC}"
    docker-compose -f docker-compose.sharding.yml up -d
    echo -e "${GREEN}✅ Services started successfully${NC}"
}

# Function to stop services
stop_services() {
    echo -e "${YELLOW}🛑 Stopping database sharding services...${NC}"
    docker-compose -f docker-compose.sharding.yml down
    echo -e "${GREEN}✅ Services stopped successfully${NC}"
}

# Function to restart services
restart_services() {
    echo -e "${YELLOW}🔄 Restarting database sharding services...${NC}"
    docker-compose -f docker-compose.sharding.yml restart
    echo -e "${GREEN}✅ Services restarted successfully${NC}"
}

# Function to show status
show_status() {
    echo -e "${BLUE}📊 Service Status:${NC}"
    echo "=================================="
    docker-compose -f docker-compose.sharding.yml ps
    echo ""
    
    echo -e "${BLUE}🔍 Detailed Status:${NC}"
    echo "=================================="
    
    # Check each service
    services=("postgres-shard-1" "postgres-shard-2" "postgres-shard-3" "redis-advisory-locks")
    
    for service in "${services[@]}"; do
        if docker ps --format "table {{.Names}}\t{{.Status}}" | grep -q "$service"; then
            status=$(docker ps --format "table {{.Names}}\t{{.Status}}" | grep "$service" | awk '{print $2}')
            echo -e "${GREEN}✅ $service: $status${NC}"
        else
            echo -e "${RED}❌ $service: Not running${NC}"
        fi
    done
}

# Function to show logs
show_logs() {
    echo -e "${BLUE}📋 Service Logs:${NC}"
    echo "=================================="
    docker-compose -f docker-compose.sharding.yml logs --tail=50
}

# Function to clean up
clean_up() {
    echo -e "${YELLOW}🧹 Cleaning up Docker resources...${NC}"
    echo -e "${RED}⚠️  This will remove all containers, volumes, and networks!${NC}"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        docker-compose -f docker-compose.sharding.yml down -v --remove-orphans
        docker system prune -f
        echo -e "${GREEN}✅ Cleanup completed${NC}"
    else
        echo -e "${YELLOW}❌ Cleanup cancelled${NC}"
    fi
}

# Function to scale services
scale_service() {
    if [ $# -lt 2 ]; then
        echo -e "${RED}❌ Usage: $0 scale <service> <count>${NC}"
        echo "Available services: postgres-shard-1, postgres-shard-2, postgres-shard-3"
        exit 1
    fi
    
    service=$1
    count=$2
    
    echo -e "${YELLOW}📈 Scaling $service to $count instances...${NC}"
    docker-compose -f docker-compose.sharding.yml up -d --scale $service=$count
    echo -e "${GREEN}✅ Scaling completed${NC}"
}

# Function to check health
check_health() {
    echo -e "${BLUE}🏥 Health Check:${NC}"
    echo "=================================="
    
    # Check PostgreSQL shards
    for i in 1 2 3; do
        container_name="postgres-shard-$i"
        if docker ps | grep -q "$container_name"; then
            if docker exec "$container_name" pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
                echo -e "${GREEN}✅ PostgreSQL Shard $i: Healthy${NC}"
            else
                echo -e "${RED}❌ PostgreSQL Shard $i: Unhealthy${NC}"
            fi
        else
            echo -e "${RED}❌ PostgreSQL Shard $i: Not running${NC}"
        fi
    done
    
    # Check Redis
    if docker ps | grep -q "redis-advisory-locks"; then
        if docker exec redis-advisory-locks redis-cli ping > /dev/null 2>&1; then
            echo -e "${GREEN}✅ Redis: Healthy${NC}"
        else
            echo -e "${RED}❌ Redis: Unhealthy${NC}"
        fi
    else
        echo -e "${RED}❌ Redis: Not running${NC}"
    fi
}

# Function to show statistics
show_stats() {
    echo -e "${BLUE}📊 Shard Statistics:${NC}"
    echo "=================================="
    
    # Show container stats
    echo -e "${YELLOW}Container Statistics:${NC}"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"
    
    echo ""
    echo -e "${YELLOW}Database Statistics:${NC}"
    
    # Check each shard
    for i in 1 2 3; do
        container_name="postgres-shard-$i"
        if docker ps | grep -q "$container_name"; then
            echo -e "${BLUE}Shard $i:${NC}"
            docker exec "$container_name" psql -U postgres -d "sub_balance_shard_$i" -c "
                SELECT 
                    'Accounts' as table_name, 
                    COUNT(*) as count 
                FROM accounts 
                UNION ALL
                SELECT 
                    'Balance Shards' as table_name, 
                    COUNT(*) as count 
                FROM account_balance_shard 
                UNION ALL
                SELECT 
                    'Transactions' as table_name, 
                    COUNT(*) as count 
                FROM transactions;
            " 2>/dev/null || echo "  Unable to connect to shard $i"
        fi
    done
}

# Main script logic
case "${1:-}" in
    start)
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        restart_services
        ;;
    status)
        show_status
        ;;
    logs)
        show_logs
        ;;
    clean)
        clean_up
        ;;
    scale)
        shift
        scale_service "$@"
        ;;
    health)
        check_health
        ;;
    stats)
        show_stats
        ;;
    *)
        usage
        exit 1
        ;;
esac
