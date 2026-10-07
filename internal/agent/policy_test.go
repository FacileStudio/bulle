package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/FacileStudio/bulle/internal/compaction"
	"github.com/FacileStudio/bulle/internal/jev"
	"github.com/FacileStudio/bulle/internal/settings"
)

// resolve is Settings as the real callers reach it: the defaults are the base the
// resolver applies itself, and the overlay carries only what the command line
// typed (FromFlags returns a sparse Config, never a filled-in one).
func resolve(t *testing.T) settings.Config {
	t.Helper()
	cfg, err := settings.Settings("", settings.Config{})
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	return cfg
}

// writeHome puts a config file in a home of the test's own, so nothing here reads
// or writes the real one, and clears the two variables that could turn the judge
// on behind the file's back.
func writeHome(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv(settings.EnvPrefix+"COMPACTION_JUDGE", "")
	if err := os.WriteFile(filepath.Join(home, settings.ConfigFile), []byte(body), 0o600); err != nil {
		t.Fatalf("writing the config: %v", err)
	}
}

// The judge is optional: a config that never mentions it builds none, which is
// what keeps a session that did not ask for one off the network.
func TestJudgeIsOffUntilTheConfigTurnsItOn(t *testing.T) {
	writeHome(t, "provider:\n  backend: anthropic\n")

	if judge := Judge(resolve(t).Compaction); judge != nil {
		t.Errorf("judge = %v, want nil while the config leaves it off", judge)
	}
}

// And it is enableable from that file alone: one key in ~/.bulle.yml, with no flag
// and no environment variable, is all it takes to switch it on and have the
// session carry it.
func TestJudgeIsBuiltFromTheConfigFile(t *testing.T) {
	writeHome(t, "limits:\n  compaction:\n    soft_ratio: 0.5\n    judge:\n      enabled: true\n      model: jev-test\n")

	cfg := resolve(t)
	if judge := Judge(cfg.Compaction); judge == nil {
		t.Fatal("judge = nil, want the file's enabled: true to build one")
	}
	if cfg.Compaction.Judge.Model != "jev-test" {
		t.Errorf("judge model = %q, want the file's own", cfg.Compaction.Judge.Model)
	}
	if soft, _ := cfg.Compaction.Ratios(); soft != 0.5 {
		t.Errorf("soft ratio = %v, want the file's 0.5 alongside it", soft)
	}

	budget := ResolveBudget(cfg.CompactAt, cfg.Compaction, &fixedWindow{window: 200_000})
	if CompactionConfig(budget, cfg.Compaction).Judge == nil {
		t.Error("the session's compaction config carries no judge, so the file's opt-in never reaches a pass")
	}
}

// Each judge provider has a wire: a surface, a host and a model. The shared
// table is what a provider name derives when the file names no model or host
// of its own.
func TestJudgeProviderTable(t *testing.T) {
	jevSpec := compaction.SpecFor(settings.JudgeProviderJEV)
	if jevSpec.Endpoint != jev.EndpointSystemOne || jevSpec.BaseURL != jev.DefaultBaseURL || jevSpec.Model != jev.DefaultModel {
		t.Errorf("jev spec = %+v, want TypeSafe System One on jev-latest", jevSpec)
	}
	clefSpec := compaction.SpecFor(settings.JudgeProviderClef)
	if clefSpec.Endpoint != jev.EndpointDecisions || clefSpec.BaseURL != "https://openrouter.ai" || clefSpec.Model != "cloudflare/clef" {
		t.Errorf("clef spec = %+v, want the OpenRouter Decisions surface on cloudflare/clef", clefSpec)
	}
	if unknown := compaction.SpecFor("klu"); unknown != jevSpec {
		t.Errorf("unknown provider spec = %+v, want the jev default", unknown)
	}
}

// The provider decides which surface a pass speaks: a clef judge posts to the
// Decisions path with the clef model in the body, and a config that names no
// provider posts to System One with jev-latest derived from the default, now
// that the defaults no longer pre-fill either.
func TestJudgeSpeaksTheProvidersSurface(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		wantPath  string
		wantModel string
	}{
		{
			name:      "the default speaks System One",
			file:      "limits:\n  compaction:\n    judge:\n      enabled: true\n",
			wantPath:  "/v1/systemone",
			wantModel: "jev-latest",
		},
		{
			name:      "clef speaks Decisions",
			file:      "limits:\n  compaction:\n    judge:\n      enabled: true\n      provider: clef\n",
			wantPath:  "/api/alpha/decisions",
			wantModel: "cloudflare/clef",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSpeaks(t, tt.file, tt.wantPath, tt.wantModel)
		})
	}
}

// assertSpeaks runs one classification against a stub and reports which surface
// and model the judge derived from the file.
func assertSpeaks(t *testing.T, file, wantPath, wantModel string) {
	call := &recordedCall{}
	server := httptest.NewServer(surfaceStub(t, call))
	t.Cleanup(server.Close)
	writeHome(t, file+"      base_url: "+server.URL+"\n")

	verdicts, err := Judge(resolve(t).Compaction).Classify(t.Context(), "goal", []compaction.Block{{Key: "b1", Text: "block text"}})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if len(verdicts) != 1 || verdicts[0].Decision != compaction.Keep {
		t.Errorf("verdicts = %v, want one keep", verdicts)
	}
	if call.path != wantPath {
		t.Errorf("path = %q, want %q", call.path, wantPath)
	}
	if call.model != wantModel {
		t.Errorf("model = %q, want %q", call.model, wantModel)
	}
}

// recordedCall is what the stub saw of one classification request.
type recordedCall struct {
	path  string
	model string
}

// surfaceStub answers one classification with a keep verdict and records which
// path spoke and which model the body named.
func surfaceStub(t *testing.T, call *recordedCall) http.HandlerFunc {
	const answer = `{"model":"answer","answers":{"b1":{"type":"choice","choice":"keep","confidence":0.9,"probabilities":{"keep":0.9,"prune":0.05,"ledger":0.05}}}}`
	return func(w http.ResponseWriter, r *http.Request) {
		call.path = r.URL.Path
		var body struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding the request: %v", err)
		}
		call.model = body.Model
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, answer); err != nil {
			t.Errorf("writing the response: %v", err)
		}
	}
}
