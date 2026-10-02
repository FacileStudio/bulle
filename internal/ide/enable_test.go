package ide

import "testing"

func TestEnabledReadsTheSwitchAndTheEnvironment(t *testing.T) {
	t.Cleanup(func() { BindFlag(nil) })
	t.Setenv(EnvVar, "")
	on, off := true, false

	BindFlag(&off)
	if Enabled() {
		t.Error("the surface is on with nothing asking for it")
	}
	BindFlag(&on)
	if !Enabled() {
		t.Error("the --ide switch did not turn the surface on")
	}

	BindFlag(&off)
	t.Setenv(EnvVar, "1")
	if !Enabled() {
		t.Error("BULLE_IDE=1 did not turn the surface on")
	}
	t.Setenv(EnvVar, "0")
	if Enabled() {
		t.Error("BULLE_IDE=0 turned the surface on")
	}
}

func TestTruthyIsOffForTheWordsThatMeanNo(t *testing.T) {
	for _, value := range []string{"", " ", "0", "false", "FALSE", "no", "off"} {
		if truthy(value) {
			t.Errorf("%q was read as yes", value)
		}
	}
	for _, value := range []string{"1", "true", "yes", "on", "/tmp/anything"} {
		if !truthy(value) {
			t.Errorf("%q was read as no", value)
		}
	}
}
