//go:generate go tool diogen -output config/diogen/di_gen.go $GOFILE
package di

// Service-specific wires go in InitDi() below. The set of bundled waffle
// modules is declared separately in modules.go via blank imports — diogen
// scans both files when generating the container.

// import "github.com/ribora/diogen/di" // uncomment when adding di.Wire / di.Define calls below

func InitDi() {
	// service-specific di.Wire / di.Define calls go here
}
