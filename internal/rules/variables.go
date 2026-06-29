package rules

import "github.com/google/cel-go/cel"

// VariableDoc describes one variable available to CEL rule expressions.
//
// Catalog is the single source of truth: the CEL environment (EnvOptions),
// the GET /rules/variables endpoint, and docs/RULE_TYPOLOGY_AUTHORING.md all
// derive from it, so they cannot drift. Add a variable here and it is declared
// in the engine and published by the API automatically.
type VariableDoc struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Group       string `json:"group"`
	Access      string `json:"access,omitempty"`
	Note        string `json:"note,omitempty"`
	Description string `json:"description,omitempty"`

	celType *cel.Type // not serialized; used to build the CEL environment
}

// dynMap is the open-ended map type used for tx / meta / enrichment.
var dynMap = cel.MapType(cel.StringType, cel.DynType)

// Catalog is the canonical list of CEL variables exposed to rule authors.
var Catalog = []VariableDoc{
	// Core transaction fields
	{Name: "amount", Type: "double", Group: "core", Description: "Transaction amount", celType: cel.DoubleType},
	{Name: "currency", Type: "string", Group: "core", Description: "ISO currency code (uppercase)", celType: cel.StringType},
	{Name: "tx_type", Type: "string", Group: "core", Description: "Transaction type (uppercase)", celType: cel.StringType},
	{Name: "debtor_id", Type: "string", Group: "core", Description: "Debtor (sender) entity ID", celType: cel.StringType},
	{Name: "creditor_id", Type: "string", Group: "core", Description: "Creditor (receiver) entity ID", celType: cel.StringType},
	{Name: "old_balance", Type: "double", Group: "core", Note: "from metadata.old_balance; defaults to 0.0", Description: "Debtor balance before the transaction", celType: cel.DoubleType},
	{Name: "new_balance", Type: "double", Group: "core", Note: "from metadata.new_balance; defaults to 0.0", Description: "Debtor balance after the transaction", celType: cel.DoubleType},
	{Name: "tx", Type: "map(string, dyn)", Group: "core", Access: "tx.<field>", Description: "Core transaction fields: id, type, debtor_id, creditor_id, amount, currency", celType: dynMap},

	// Velocity aggregates (engine-computed over Osprey's own transaction store)
	{Name: "velocity_count", Type: "int", Group: "velocity", Description: "Recent transaction count for the debtor in the window", celType: cel.IntType},
	{Name: "velocity_amount_sum", Type: "double", Group: "velocity", Description: "Sum of recent transaction amounts for the debtor in the window", celType: cel.DoubleType},
	{Name: "velocity_distinct_creditors", Type: "int", Group: "velocity", Description: "Distinct counterparties the debtor transacted with in the window", celType: cel.IntType},

	// Open-ended bags — no per-field declaration needed
	{Name: "meta", Type: "map(string, dyn)", Group: "metadata", Access: "meta.<key>", Note: "open-ended; guard optional keys with has(meta.<key>)", Description: "Caller-supplied request metadata (country, mcc, device, ...)", celType: dynMap},
	{Name: "enrichment", Type: "map(string, dyn)", Group: "enrichment", Access: "enrichment.<key>", Note: "externally computed; guard with has(enrichment.<key>); JSON numbers are doubles", Description: "Scores/flags from external enrichment services (ml_score, sanctions_hit, ring_risk, ...)", celType: dynMap},
}

// mapOrEmpty returns m, or a non-nil empty map when m is nil, so the meta/enrichment
// CEL variables are always present and has() guards never error on a nil map.
func mapOrEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// EnvOptions returns the CEL environment variable declarations derived from Catalog.
func EnvOptions() []cel.EnvOption {
	opts := make([]cel.EnvOption, 0, len(Catalog))
	for _, v := range Catalog {
		opts = append(opts, cel.Variable(v.Name, v.celType))
	}
	return opts
}
