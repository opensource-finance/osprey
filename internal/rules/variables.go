package rules

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/cel-go/cel"
)

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

	celType         *cel.Type // not serialized; used to build the CEL environment
	metadataSourced bool      // not serialized; caller metadata may override it
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
	{Name: "old_balance", Type: "double", Group: "core", Note: "from metadata.old_balance; defaults to 0.0", Description: "Debtor balance before the transaction", celType: cel.DoubleType, metadataSourced: true},
	{Name: "new_balance", Type: "double", Group: "core", Note: "from metadata.new_balance; defaults to 0.0", Description: "Debtor balance after the transaction", celType: cel.DoubleType, metadataSourced: true},
	{Name: "tx", Type: "map(string, dyn)", Group: "core", Access: "tx.<field>", Description: "Core transaction fields: id, type, debtor_id, creditor_id, amount, currency", celType: dynMap},

	// Velocity aggregates (engine-computed over Osprey's own transaction store)
	{Name: "velocity_count", Type: "int", Group: "velocity", Note: "includes the current transaction", Description: "Transactions in the window where the debtor sent or received money", celType: cel.IntType},
	{Name: "velocity_amount_sum", Type: "double", Group: "velocity", Note: "includes the current transaction", Description: "Sum of the amounts of those transactions, sent and received", celType: cel.DoubleType},
	{Name: "velocity_distinct_creditors", Type: "int", Group: "velocity", Note: "includes the current payee", Description: "Distinct payees the debtor sent money to in the window", celType: cel.IntType},
	{Name: "velocity_inbound_count", Type: "int", Group: "velocity", Description: "Transactions in the window where the debtor received money", celType: cel.IntType},
	{Name: "velocity_inbound_amount_sum", Type: "double", Group: "velocity", Description: "Sum of the amounts the debtor received in the window", celType: cel.DoubleType},

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

// metadataOverridable lists Catalog vars that caller metadata may set (old_balance/new_balance).
var metadataOverridable = func() map[string]struct{} {
	allow := make(map[string]struct{}, 2)
	for _, v := range Catalog {
		if v.metadataSourced {
			allow[v.Name] = struct{}{}
		}
	}
	return allow
}()

// ErrInvalidMetadata reports a metadata-sourced variable with a value that is
// not a number.
var ErrInvalidMetadata = errors.New("invalid metadata")

// metadataNumber converts a metadata value for a double variable such as
// old_balance. A JSON null means "not provided", the same as an absent key, so
// ok is false and the caller keeps the 0.0 default. Any other value that is not
// a number is an error. Without this check a string or null replaced the
// float64 default, and CEL's cross-type == returned false, so
// `new_balance == 0.0` did not match and a drain rule passed (issue #52).
func metadataNumber(key string, v any) (f float64, ok bool, err error) {
	switch n := v.(type) {
	case nil:
		return 0, false, nil
	case float64:
		return n, true, nil
	case float32:
		return float64(n), true, nil
	case int:
		return float64(n), true, nil
	case int32:
		return float64(n), true, nil
	case int64:
		return float64(n), true, nil
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false, fmt.Errorf("%w: metadata.%s must be a number or null", ErrInvalidMetadata, key)
		}
		return f, true, nil
	default:
		return 0, false, fmt.Errorf("%w: metadata.%s must be a number or null, got %T", ErrInvalidMetadata, key, v)
	}
}

// ValidateMetadata checks that every metadata-sourced variable in md is a
// number or null. Call it before the transaction is stored, so a bad request
// gets a 400 instead of a failed evaluation.
func ValidateMetadata(md map[string]any) error {
	for k, v := range md {
		if _, ok := metadataOverridable[k]; !ok {
			continue
		}
		if _, _, err := metadataNumber(k, v); err != nil {
			return err
		}
	}
	return nil
}
