package rca

import (
	"testing"
)

func TestSelectRepoByHypothesis_ProductBug(t *testing.T) {
	repos := []RepoConfig{
		{Name: "linuxptp-daemon-operator", Purpose: "PTP operator: manages linuxptp-daemon DaemonSet, PtpConfig CRD, clock sync"},
		{Name: "ptp-test-framework", Purpose: "E2E test suite for PTP operator: Ginkgo specs, test helpers, fixtures"},
		{Name: "cluster-infra-config", Purpose: "CI cluster configuration: job profiles, NTP config, network templates"},
	}

	got := selectRepoByHypothesis("pb_clock_sync", repos)
	if len(got) != 1 || got[0] != "linuxptp-daemon-operator" {
		t.Errorf("product bug: got %v, want [linuxptp-daemon-operator]", got)
	}
}

func TestSelectRepoByHypothesis_AutomationBug(t *testing.T) {
	repos := []RepoConfig{
		{Name: "linuxptp-daemon-operator", Purpose: "PTP operator: manages linuxptp-daemon DaemonSet"},
		{Name: "ptp-test-framework", Purpose: "E2E test suite for PTP operator: Ginkgo specs, test helpers"},
		{Name: "cluster-infra-config", Purpose: "CI cluster configuration: job profiles"},
	}

	got := selectRepoByHypothesis("au_test_logic", repos)
	if len(got) != 1 || got[0] != "ptp-test-framework" {
		t.Errorf("automation bug: got %v, want [ptp-test-framework]", got)
	}
}

func TestSelectRepoByHypothesis_EnvironmentIssue(t *testing.T) {
	repos := []RepoConfig{
		{Name: "linuxptp-daemon-operator", Purpose: "PTP operator: manages linuxptp-daemon DaemonSet"},
		{Name: "ptp-test-framework", Purpose: "E2E test suite for PTP operator"},
		{Name: "cluster-infra-config", Purpose: "CI cluster configuration: job profiles, NTP config"},
	}

	got := selectRepoByHypothesis("en_ntp_drift", repos)
	if len(got) != 1 || got[0] != "cluster-infra-config" {
		t.Errorf("environment issue: got %v, want [cluster-infra-config]", got)
	}
}

func TestSelectRepoByHypothesis_EmptyHypothesis(t *testing.T) {
	repos := []RepoConfig{
		{Name: "some-repo", Purpose: "some purpose"},
	}
	if got := selectRepoByHypothesis("", repos); got != nil {
		t.Errorf("empty hypothesis: got %v, want nil", got)
	}
}

func TestSelectRepoByHypothesis_EmptyRepos(t *testing.T) {
	if got := selectRepoByHypothesis("pb_something", nil); got != nil {
		t.Errorf("empty repos: got %v, want nil", got)
	}
}

func TestSelectRepoByHypothesis_UnknownPrefix(t *testing.T) {
	repos := []RepoConfig{
		{Name: "some-repo", Purpose: "operator code"},
	}
	if got := selectRepoByHypothesis("xx_unknown", repos); got != nil {
		t.Errorf("unknown prefix: got %v, want nil", got)
	}
}

func TestSelectRepoByHypothesis_NoMatch(t *testing.T) {
	repos := []RepoConfig{
		{Name: "unrelated", Purpose: "documentation and notes"},
	}
	if got := selectRepoByHypothesis("pb_something", repos); got != nil {
		t.Errorf("no purpose match: got %v, want nil", got)
	}
}

func TestSelectRepoByHypothesis_SkipsRedHerring(t *testing.T) {
	repos := []RepoConfig{
		{Name: "decoy-operator", Purpose: "PTP operator fork", IsRedHerring: true},
		{Name: "real-operator", Purpose: "PTP operator: daemon management"},
	}

	got := selectRepoByHypothesis("pb_sync_issue", repos)
	if len(got) != 1 || got[0] != "real-operator" {
		t.Errorf("red herring: got %v, want [real-operator]", got)
	}
}

func TestSelectRepoByHypothesis_ExcludesDeployRepoForProductBug(t *testing.T) {
	repos := []RepoConfig{
		{Name: "linuxptp-daemon-operator", Purpose: "PTP operator: manages linuxptp-daemon DaemonSet"},
		{Name: "cnf-features-deploy", Purpose: "CNF deployment manifests and CI profiles: contains job definitions for all telco operators"},
		{Name: "ptp-test-framework", Purpose: "E2E test suite for PTP operator"},
	}

	got := selectRepoByHypothesis("pb_holdover", repos)
	if len(got) != 1 || got[0] != "linuxptp-daemon-operator" {
		t.Errorf("should exclude deploy repo: got %v, want [linuxptp-daemon-operator]", got)
	}
}

func TestSelectRepoByHypothesis_CaseInsensitive(t *testing.T) {
	repos := []RepoConfig{
		{Name: "my-operator", Purpose: "PTP Operator: DaemonSet management"},
	}

	got := selectRepoByHypothesis("PB_CLOCK", repos)
	if len(got) != 1 || got[0] != "my-operator" {
		t.Errorf("case insensitive: got %v, want [my-operator]", got)
	}
}
