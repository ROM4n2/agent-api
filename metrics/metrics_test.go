package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerExposesCounters(t *testing.T) {
	// 干净环境：只计一次提交与一次完成
	IncSubmitted()
	IncDone()

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	for _, want := range []string{
		"# TYPE agent_tasks_submitted counter",
		"agent_tasks_submitted 1",
		"agent_tasks_running 0",
		"agent_tasks_done 1",
		"agent_tasks_failed 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics body missing %q; got:\n%s", want, body)
		}
	}
}

func TestCounterMonotonic(t *testing.T) {
	before := done.Load()
	IncDone()
	if done.Load() != before+1 {
		t.Fatalf("done counter not incremented")
	}
}
