package pricing

// A layer beside the four the framework scaffolds, reachable from anywhere in
// its own domain — shark keeps two of these.
type Tier string

const (
	TierBase    Tier = "base"
	TierPremium Tier = "premium"
)
