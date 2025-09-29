#!/bin/bash

# Docker Database Sharding Setup Script
# This script sets up database sharding using Docker

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🐳 Setting up Database Sharding with Docker${NC}"
echo "=================================================="

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker is not installed. Please install Docker first.${NC}"
    exit 1
fi

# Check if Docker Compose is installed
if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}❌ Docker Compose is not installed. Please install Docker Compose first.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Docker and Docker Compose are available${NC}"

# Create necessary directories
echo -e "${YELLOW}📁 Creating necessary directories...${NC}"
mkdir -p scripts
mkdir -p data/shard1
mkdir -p data/shard2
mkdir -p data/shard3

# Create initialization scripts for each shard
echo -e "${YELLOW}📝 Creating database initialization scripts...${NC}"

# Shard 1 initialization
cat > scripts/init-shard-1.sql << 'EOF'
-- Shard 1: Account ID range 0-33%
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_no VARCHAR(50) UNIQUE NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    use_sub_balance BOOLEAN DEFAULT false,
    shard_count INTEGER DEFAULT 8,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Account balance shard table
CREATE TABLE IF NOT EXISTS account_balance_shard (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_account_id UUID NOT NULL REFERENCES accounts(id),
    shard_index INTEGER NOT NULL,
    shard_hash VARCHAR(100) NOT NULL,
    credit_amount DECIMAL(20,2) DEFAULT 0,
    debit_amount DECIMAL(20,2) DEFAULT 0,
    total_balance DECIMAL(20,2) DEFAULT 0,
    reserve_balance DECIMAL(20,2) DEFAULT 0,
    unsettled_amount DECIMAL(20,2) DEFAULT 0,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    UNIQUE(parent_account_id, shard_index)
);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id VARCHAR(100) UNIQUE NOT NULL,
    parent_account_id UUID NOT NULL REFERENCES accounts(id),
    shard_id UUID REFERENCES account_balance_shard(id),
    shard_index INTEGER,
    transaction_type VARCHAR(20) NOT NULL,
    amount DECIMAL(20,2) NOT NULL,
    previous_balance DECIMAL(20,2),
    new_balance DECIMAL(20,2),
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    metadata JSONB
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_accounts_wallet_no ON accounts(wallet_no);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_parent ON account_balance_shard(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_index ON account_balance_shard(parent_account_id, shard_index);
CREATE INDEX IF NOT EXISTS idx_transactions_parent ON transactions(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_shard ON transactions(shard_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);

-- Insert shard metadata
INSERT INTO accounts (id, wallet_no, account_name, use_sub_balance, shard_count) 
VALUES ('00000000-0000-0000-0000-000000000001', 'shard_1_metadata', 'Shard 1 Metadata', false, 0)
ON CONFLICT (wallet_no) DO NOTHING;
EOF

# Shard 2 initialization
cat > scripts/init-shard-2.sql << 'EOF'
-- Shard 2: Account ID range 34-66%
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_no VARCHAR(50) UNIQUE NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    use_sub_balance BOOLEAN DEFAULT false,
    shard_count INTEGER DEFAULT 8,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Account balance shard table
CREATE TABLE IF NOT EXISTS account_balance_shard (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_account_id UUID NOT NULL REFERENCES accounts(id),
    shard_index INTEGER NOT NULL,
    shard_hash VARCHAR(100) NOT NULL,
    credit_amount DECIMAL(20,2) DEFAULT 0,
    debit_amount DECIMAL(20,2) DEFAULT 0,
    total_balance DECIMAL(20,2) DEFAULT 0,
    reserve_balance DECIMAL(20,2) DEFAULT 0,
    unsettled_amount DECIMAL(20,2) DEFAULT 0,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    UNIQUE(parent_account_id, shard_index)
);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id VARCHAR(100) UNIQUE NOT NULL,
    parent_account_id UUID NOT NULL REFERENCES accounts(id),
    shard_id UUID REFERENCES account_balance_shard(id),
    shard_index INTEGER,
    transaction_type VARCHAR(20) NOT NULL,
    amount DECIMAL(20,2) NOT NULL,
    previous_balance DECIMAL(20,2),
    new_balance DECIMAL(20,2),
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    metadata JSONB
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_accounts_wallet_no ON accounts(wallet_no);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_parent ON account_balance_shard(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_index ON account_balance_shard(parent_account_id, shard_index);
CREATE INDEX IF NOT EXISTS idx_transactions_parent ON transactions(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_shard ON transactions(shard_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);

-- Insert shard metadata
INSERT INTO accounts (id, wallet_no, account_name, use_sub_balance, shard_count) 
VALUES ('00000000-0000-0000-0000-000000000002', 'shard_2_metadata', 'Shard 2 Metadata', false, 0)
ON CONFLICT (wallet_no) DO NOTHING;
EOF

# Shard 3 initialization
cat > scripts/init-shard-3.sql << 'EOF'
-- Shard 3: Account ID range 67-100%
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    wallet_no VARCHAR(50) UNIQUE NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    use_sub_balance BOOLEAN DEFAULT false,
    shard_count INTEGER DEFAULT 8,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Account balance shard table
CREATE TABLE IF NOT EXISTS account_balance_shard (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    parent_account_id UUID NOT NULL REFERENCES accounts(id),
    shard_index INTEGER NOT NULL,
    shard_hash VARCHAR(100) NOT NULL,
    credit_amount DECIMAL(20,2) DEFAULT 0,
    debit_amount DECIMAL(20,2) DEFAULT 0,
    total_balance DECIMAL(20,2) DEFAULT 0,
    reserve_balance DECIMAL(20,2) DEFAULT 0,
    unsettled_amount DECIMAL(20,2) DEFAULT 0,
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    UNIQUE(parent_account_id, shard_index)
);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id VARCHAR(100) UNIQUE NOT NULL,
    parent_account_id UUID NOT NULL REFERENCES accounts(id),
    shard_id UUID REFERENCES account_balance_shard(id),
    shard_index INTEGER,
    transaction_type VARCHAR(20) NOT NULL,
    amount DECIMAL(20,2) NOT NULL,
    previous_balance DECIMAL(20,2),
    new_balance DECIMAL(20,2),
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    metadata JSONB
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_accounts_wallet_no ON accounts(wallet_no);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_parent ON account_balance_shard(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_account_balance_shard_index ON account_balance_shard(parent_account_id, shard_index);
CREATE INDEX IF NOT EXISTS idx_transactions_parent ON transactions(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_shard ON transactions(shard_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);

-- Insert shard metadata
INSERT INTO accounts (id, wallet_no, account_name, use_sub_balance, shard_count) 
VALUES ('00000000-0000-0000-0000-000000000003', 'shard_3_metadata', 'Shard 3 Metadata', false, 0)
ON CONFLICT (wallet_no) DO NOTHING;
EOF

echo -e "${GREEN}✅ Database initialization scripts created${NC}"

# Start Docker services
echo -e "${YELLOW}🚀 Starting Docker services...${NC}"
docker-compose -f docker-compose.sharding.yml up -d

# Wait for services to be ready
echo -e "${YELLOW}⏳ Waiting for services to be ready...${NC}"
sleep 10

# Check service health
echo -e "${YELLOW}🔍 Checking service health...${NC}"

# Check PostgreSQL shards
for port in 5432 5433 5434; do
    if docker exec postgres-shard-1 pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PostgreSQL Shard 1 (port $port) is ready${NC}"
    else
        echo -e "${RED}❌ PostgreSQL Shard 1 (port $port) is not ready${NC}"
    fi
done

# Check Redis
if docker exec redis-advisory-locks redis-cli ping > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Redis is ready${NC}"
else
    echo -e "${RED}❌ Redis is not ready${NC}"
fi

echo -e "${GREEN}🎉 Database Sharding setup completed!${NC}"
echo ""
echo -e "${BLUE}📊 Service Information:${NC}"
echo "  - PostgreSQL Shard 1: localhost:5432"
echo "  - PostgreSQL Shard 2: localhost:5433"
echo "  - PostgreSQL Shard 3: localhost:5434"
echo "  - Redis: localhost:6379"
echo ""
echo -e "${BLUE}🔧 Management Commands:${NC}"
echo "  - Start services: docker-compose -f docker-compose.sharding.yml up -d"
echo "  - Stop services: docker-compose -f docker-compose.sharding.yml down"
echo "  - View logs: docker-compose -f docker-compose.sharding.yml logs"
echo "  - Scale shards: docker-compose -f docker-compose.sharding.yml up -d --scale postgres-shard-1=2"
echo ""
echo -e "${YELLOW}💡 Next Steps:${NC}"
echo "  1. Update your application to use the shard router"
echo "  2. Configure environment variables"
echo "  3. Run your application with sharding enabled"
echo "  4. Test the sharding functionality"
