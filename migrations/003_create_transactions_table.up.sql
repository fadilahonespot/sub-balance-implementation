-- Create transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id VARCHAR(255) PRIMARY KEY,
    transaction_id VARCHAR(255) NOT NULL,
    parent_account_id VARCHAR(255) NOT NULL,
    shard_id VARCHAR(255) NOT NULL,
    shard_index INTEGER NOT NULL,
    transaction_type VARCHAR(20) NOT NULL CHECK (transaction_type IN ('debit', 'credit')),
    amount DECIMAL(20,2) NOT NULL,
    previous_balance DECIMAL(20,2) NOT NULL,
    new_balance DECIMAL(20,2) NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed', 'rolled_back')),
    created_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    modified_on TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    checksum VARCHAR(255),
    metadata JSONB
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_transactions_transaction_id ON transactions(transaction_id);
CREATE INDEX IF NOT EXISTS idx_transactions_parent_account_id ON transactions(parent_account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_shard_id ON transactions(shard_id);
CREATE INDEX IF NOT EXISTS idx_transactions_created_on ON transactions(created_on);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(transaction_type);

-- Create composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_transactions_account_created ON transactions(parent_account_id, created_on);
CREATE INDEX IF NOT EXISTS idx_transactions_shard_created ON transactions(shard_id, created_on);

-- Add foreign key constraints
ALTER TABLE transactions 
ADD CONSTRAINT fk_transactions_parent_account 
FOREIGN KEY (parent_account_id) REFERENCES accounts(id) ON DELETE CASCADE;

ALTER TABLE transactions 
ADD CONSTRAINT fk_transactions_shard 
FOREIGN KEY (shard_id) REFERENCES account_balance_shard(id) ON DELETE CASCADE;

-- Add comments
COMMENT ON TABLE transactions IS 'Transaction log for all balance changes';
COMMENT ON COLUMN transactions.transaction_id IS 'Business transaction ID (can span multiple shards)';
COMMENT ON COLUMN transactions.parent_account_id IS 'Reference to parent account';
COMMENT ON COLUMN transactions.shard_id IS 'Reference to specific shard that was modified';
COMMENT ON COLUMN transactions.shard_index IS 'Index of the shard for quick reference';
COMMENT ON COLUMN transactions.transaction_type IS 'Type of transaction: debit or credit';
COMMENT ON COLUMN transactions.amount IS 'Transaction amount';
COMMENT ON COLUMN transactions.previous_balance IS 'Balance before transaction';
COMMENT ON COLUMN transactions.new_balance IS 'Balance after transaction';
COMMENT ON COLUMN transactions.status IS 'Transaction status';
COMMENT ON COLUMN transactions.metadata IS 'Additional transaction metadata in JSON format';
