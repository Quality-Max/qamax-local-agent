package exposure

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/api/agent/5/crawl/2/snapshot", CatBehavioral},
		{"/api/agent/5/assignments/9/result", CatTestArtifact},
		{"/api/projects/1/user-data/categories/2/fields", CatAuthMaterial},
		{"/api/sast/scan", CatSourceDerived},
		{"/api/agent/5/heartbeat", CatTelemetry},
		{"/api/agent/register", CatControl},
		{"/api/ai-crawl/start", CatTestPlan},
		{"/api/something/unknown", CatUncategorized},
	}
	for _, c := range cases {
		if got := Classify("app.qualitymax.io", c.path); got != c.want {
			t.Errorf("Classify(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
