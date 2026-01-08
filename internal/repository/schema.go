package repository

// Schema definitions for Osprey database.
// Compatible with both SQLite and PostgreSQL.

const schemaTransactions = `
CREATE TABLE IF NOT EXISTS transactions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    type TEXT NOT NULL,
    debtor_id TEXT NOT NULL,
    debtor_account_id TEXT NOT NULL,
    creditor_id TEXT NOT NULL,
    creditor_account_id TEXT NOT NULL,
    amount REAL NOT NULL,
    currency TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    metadata TEXT,
    original_message BLOB
);

CREATE INDEX IF NOT EXISTS idx_transactions_tenant ON transactions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_transactions_debtor ON transactions(tenant_id, debtor_id);
CREATE INDEX IF NOT EXISTS idx_transactions_creditor ON transactions(tenant_id, creditor_id);
CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(tenant_id, timestamp);
`

const schemaRuleConfigs = `
CREATE TABLE IF NOT EXISTS rule_configs (
    id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    version TEXT NOT NULL,
    expression TEXT NOT NULL,
    bands TEXT NOT NULL,
    weight REAL NOT NULL DEFAULT 1.0,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (id, tenant_id, version)
);

CREATE INDEX IF NOT EXISTS idx_rule_configs_tenant ON rule_configs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_rule_configs_enabled ON rule_configs(tenant_id, enabled);
`

const schemaEvaluations = `
CREATE TABLE IF NOT EXISTS evaluations (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    tx_id TEXT NOT NULL,
    status TEXT NOT NULL,
    score REAL NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    rule_results TEXT NOT NULL,
    typology_results TEXT,
    metadata TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_evaluations_tenant ON evaluations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_evaluations_tx ON evaluations(tenant_id, tx_id);
CREATE INDEX IF NOT EXISTS idx_evaluations_status ON evaluations(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_evaluations_timestamp ON evaluations(tenant_id, timestamp);
`

// schemaTypologies defines the typologies table.
// Typologies group multiple rules with weights to calculate composite risk scores.
// Compatible with both SQLite and PostgreSQL.
const schemaTypologies = `
CREATE TABLE IF NOT EXISTS typologies (
    id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    version TEXT NOT NULL,
    rules TEXT NOT NULL,
    alert_threshold REAL NOT NULL DEFAULT 0.6,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (id, tenant_id, version)
);

CREATE INDEX IF NOT EXISTS idx_typologies_tenant ON typologies(tenant_id);
CREATE INDEX IF NOT EXISTS idx_typologies_enabled ON typologies(tenant_id, enabled);
CREATE INDEX IF NOT EXISTS idx_typologies_name ON typologies(tenant_id, name);
`

// AllSchemas returns all schema statements in order.
func AllSchemas() []string {
	return []string{
		schemaTransactions,
		schemaRuleConfigs,
		schemaEvaluations,
		schemaTypologies,
	}
}
