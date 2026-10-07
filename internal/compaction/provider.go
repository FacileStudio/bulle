package compaction

import "github.com/FacileStudio/bulle/internal/jev"

// ProviderSpec is one judge provider's wire: which surface to speak, where to
// speak it, and the model that answers there. The provider names are the ones
// settings validates ("jev", "clef"); this package cannot import settings for
// the constants because settings already imports this one.
type ProviderSpec struct {
	Endpoint jev.Endpoint
	BaseURL  string
	Model    string
}

const (
	clefBaseURL = "https://openrouter.ai"
	clefModel   = "cloudflare/clef"
)

// providerSpecs maps each supported provider name to its wire. It is the single
// derivation point: the factory that builds a session's judge and the
// calibration harness that tunes one both read it, so a model cannot ship in a
// pass and out of a sweep.
var providerSpecs = map[string]ProviderSpec{
	"jev":  {Endpoint: jev.EndpointSystemOne, BaseURL: jev.DefaultBaseURL, Model: jev.DefaultModel},
	"clef": {Endpoint: jev.EndpointDecisions, BaseURL: clefBaseURL, Model: clefModel},
}

// SpecFor is the wire one provider derives. An empty or unknown name reads as
// jev: settings refuses unknown providers at load, so reaching here with one
// means a caller bypassed validation, and the default surface is the one that
// fails safe — the same wire the judge has always spoken.
func SpecFor(provider string) ProviderSpec {
	if spec, ok := providerSpecs[provider]; ok {
		return spec
	}
	return providerSpecs["jev"]
}
