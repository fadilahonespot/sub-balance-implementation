#!/bin/bash

# Setup database for sub-balance implementation
echo "Setting up PostgreSQL database for sub-balance implementation..."

# Check if PostgreSQL is running
if ! pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
    echo "PostgreSQL is not running. Please start PostgreSQL first."
    echo "On macOS with Homebrew: brew services start postgresql"
    echo "On Ubuntu/Debian: sudo systemctl start postgresql"
    exit 1
fi

# Create database and user
echo "Creating database and user..."

# Connect to PostgreSQL and create database/user
psql -h localhost -p 5432 -U $(whoami) -d postgres << EOF
-- Create user if not exists
DO \$\$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'sub_balance_user') THEN
        CREATE ROLE sub_balance_user WITH LOGIN PASSWORD 'sub_balance_password';
    END IF;
END
\$\$;

-- Create database if not exists
SELECT 'CREATE DATABASE sub_balance_db OWNER sub_balance_user'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'sub_balance_db')\gexec

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE sub_balance_db TO sub_balance_user;
EOF

echo "Database setup completed!"
echo "Database: sub_balance_db"
echo "User: sub_balance_user"
echo "Password: sub_balance_password"
echo ""
echo "Update your config.yaml with these credentials."
