package categories

// Fixture represents a predefined set of categories for testing.
// Fixtures provide consistent, reusable category sets for different test scenarios.
type Fixture interface {
	// Name returns the fixture's descriptive name.
	Name() string

	// Description returns a detailed description of the fixture's purpose.
	Description() string

	// Categories returns the category names included in this fixture.
	Categories() []CategoryName

	// Version returns the fixture version for evolution support.
	Version() int
}

// fixture implements the Fixture interface.
type fixture struct {
	name        string
	description string
	categories  []CategoryName
	version     int
}

func (f *fixture) Name() string                  { return f.name }
func (f *fixture) Description() string           { return f.description }
func (f *fixture) Categories() []CategoryName    { return f.categories }
func (f *fixture) Version() int                  { return f.version }

// Predefined fixtures for common test scenarios.
var (
	// FixtureMinimal provides the absolute minimum categories for basic tests.
	FixtureMinimal = &fixture{
		name:        "Minimal",
		description: "Minimal category set for simple unit tests",
		version:     1,
		categories: []CategoryName{
			CategoryFoodDining,
			CategoryShopping,
			CategoryTransportation,
		},
	}

	// FixtureStandard provides a standard set of categories for most tests.
	FixtureStandard = &fixture{
		name:        "Standard",
		description: "Standard category set covering common transaction types",
		version:     1,
		categories: []CategoryName{
			CategoryGroceries,
			CategoryFoodDining,
			CategoryCoffeeDining,
			CategoryShopping,
			CategoryOnlineShopping,
			CategoryTransportation,
			CategorySubscriptions,
			CategoryUtilities,
			CategoryEntertainment,
			CategoryHealthFitness,
		},
	}

	// FixtureComprehensive provides all commonly used categories.
	FixtureComprehensive = &fixture{
		name:        "Comprehensive",
		description: "Comprehensive category set for integration and complex tests",
		version:     1,
		categories: []CategoryName{
			CategoryGroceries,
			CategoryFoodDining,
			CategoryCoffeeDining,
			CategoryShopping,
			CategoryOnlineShopping,
			CategoryTransportation,
			CategorySubscriptions,
			CategoryHealthFitness,
			CategoryEntertainment,
			CategoryUtilities,
			CategoryEducation,
			CategoryTravel,
			CategoryPersonalCare,
			CategoryGiftsCharitable,
			CategoryHomeImprovement,
			CategoryInsurance,
			CategoryInvestmentsSavings,
			CategoryBankingFees,
		},
	}

	// FixtureTestingOnly provides generic categories for test-specific scenarios.
	FixtureTestingOnly = &fixture{
		name:        "TestingOnly",
		description: "Generic categories for testing state transitions and edge cases",
		version:     1,
		categories: []CategoryName{
			CategoryTest1,
			CategoryTest2,
			CategoryTest3,
			CategoryInitial,
			CategoryUserCorrected,
			CategoryFinal,
		},
	}

	// FixtureVendorTesting provides categories commonly used in vendor rule tests.
	FixtureVendorTesting = &fixture{
		name:        "VendorTesting",
		description: "Categories for testing vendor rules and classifications",
		version:     1,
		categories: []CategoryName{
			CategoryCoffeeDining,
			CategoryGroceries,
			CategoryOnlineShopping,
			CategorySubscriptions,
			"Cat1", // Legacy test categories
			"Cat2",
			"Cat3",
		},
	}
)

// FixtureRegistry provides access to all available fixtures.
type FixtureRegistry struct {
	fixtures map[string]Fixture
}

// NewFixtureRegistry creates a registry with all predefined fixtures.
func NewFixtureRegistry() *FixtureRegistry {
	registry := &FixtureRegistry{
		fixtures: make(map[string]Fixture),
	}

	// Register all predefined fixtures
	registry.Register(FixtureMinimal)
	registry.Register(FixtureStandard)
	registry.Register(FixtureComprehensive)
	registry.Register(FixtureTestingOnly)
	registry.Register(FixtureVendorTesting)

	return registry
}

// Register adds a fixture to the registry.
func (r *FixtureRegistry) Register(f Fixture) {
	r.fixtures[f.Name()] = f
}

// Get retrieves a fixture by name.
func (r *FixtureRegistry) Get(name string) (Fixture, bool) {
	f, ok := r.fixtures[name]
	return f, ok
}

// All returns all registered fixtures.
func (r *FixtureRegistry) All() []Fixture {
	var fixtures []Fixture
	for _, f := range r.fixtures {
		fixtures = append(fixtures, f)
	}
	return fixtures
}

// CompositeFixture allows combining multiple fixtures.
type CompositeFixture struct {
	name        string
	description string
	fixtures    []Fixture
}

// NewCompositeFixture creates a fixture that combines multiple fixtures.
func NewCompositeFixture(name, description string, fixtures ...Fixture) Fixture {
	return &CompositeFixture{
		name:        name,
		description: description,
		fixtures:    fixtures,
	}
}

func (c *CompositeFixture) Name() string        { return c.name }
func (c *CompositeFixture) Description() string { return c.description }
func (c *CompositeFixture) Version() int        { return 1 }

func (c *CompositeFixture) Categories() []CategoryName {
	seen := make(map[CategoryName]struct{})
	var categories []CategoryName

	for _, f := range c.fixtures {
		for _, cat := range f.Categories() {
			if _, exists := seen[cat]; !exists {
				seen[cat] = struct{}{}
				categories = append(categories, cat)
			}
		}
	}

	return categories
}