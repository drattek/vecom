package pricing

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	mysqlInfra "core-orchestrator/internal/infrastructure/mysql"
)

var (
	ErrInvalidPricingFormula = errors.New("invalid pricing formula expression")
	ErrNoPricingFormula      = errors.New("no pricing formula configured")
)

// pricingFormulaVariable is the only identifier a stored expression may
// reference: the product's base MXN price (ecom_product_prices, resolved
// via ProductPricesRepository.FindEffectivePrice by the caller).
const pricingFormulaVariable = "base"

// compilePricingFormula compiles expression against an env exposing only
// "base" (float64) and requires the result to be numeric — catches syntax
// errors and non-numeric results at compile time, before ever touching a
// real base price. Shared by PricingFormulaService (validating on
// create/update) and PricingFormulaCalculator (evaluating on sync).
func compilePricingFormula(expression string) (*vm.Program, error) {
	env := map[string]float64{pricingFormulaVariable: 0}
	program, err := expr.Compile(expression, expr.Env(env), expr.AsFloat64())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPricingFormula, err)
	}
	return program, nil
}

// evaluatePricingFormula compiles expression and runs it with base bound to
// basePrice, returning the (still unrounded) numeric result.
func evaluatePricingFormula(expression string, basePrice float64) (float64, error) {
	program, err := compilePricingFormula(expression)
	if err != nil {
		return 0, err
	}

	env := map[string]float64{pricingFormulaVariable: basePrice}
	output, err := expr.Run(program, env)
	if err != nil {
		return 0, fmt.Errorf("error running pricing formula: %w", err)
	}

	result, ok := output.(float64)
	if !ok {
		return 0, fmt.Errorf("%w: expression did not evaluate to a number", ErrInvalidPricingFormula)
	}

	return result, nil
}

// PricingFormulaCalculator resolves the pricing formula that applies to a
// product's brand, the channel connection being synced to, and the price
// list that won when resolving the product's effective price (falling back
// through wildcard rows down to the universal default — see
// PricingFormulaRepository.Resolve for the specificity order) and evaluates
// it against a base MXN price. It is the single place both
// MercadoLibreProductSyncService and OdooProductSyncService go through to
// compute the price they push to their respective marketplace, replacing
// what used to be a formula hardcoded identically for every product.
type PricingFormulaCalculator struct {
	repository *mysqlInfra.PricingFormulaRepository
}

func NewPricingFormulaCalculator(repository *mysqlInfra.PricingFormulaRepository) *PricingFormulaCalculator {
	return &PricingFormulaCalculator{repository: repository}
}

// CalculatePrice resolves the formula for (brandID, connectionID,
// priceListID) and evaluates it against basePrice, rounding to 2 decimals
// — MXN accepts at most 2, and expression results routinely carry far more
// precision than that.
func (c *PricingFormulaCalculator) CalculatePrice(ctx context.Context, brandID *int64, connectionID int64, priceListID int64, basePrice float64) (float64, error) {
	formula, err := c.repository.Resolve(ctx, brandID, &connectionID, &priceListID)
	if err != nil {
		if errors.Is(err, mysqlInfra.ErrPricingFormulaNotFound) {
			return 0, fmt.Errorf("%w: no formula for brand=%s connection=%d price_list=%d and no universal default exists", ErrNoPricingFormula, formatOptionalID(brandID), connectionID, priceListID)
		}
		return 0, fmt.Errorf("error loading pricing formula for brand=%s connection=%d price_list=%d: %w", formatOptionalID(brandID), connectionID, priceListID, err)
	}

	result, err := evaluatePricingFormula(formula.Expression, basePrice)
	if err != nil {
		return 0, fmt.Errorf("error evaluating pricing formula %d: %w", formula.ID, err)
	}

	return math.Round(result*100) / 100, nil
}

// formatOptionalID renders a nullable id for error messages ("none" instead
// of a pointer address).
func formatOptionalID(id *int64) string {
	if id == nil {
		return "none"
	}
	return fmt.Sprintf("%d", *id)
}
