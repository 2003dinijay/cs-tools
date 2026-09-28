// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// mockEntityScheduleClient stands in for entity-service, recording what the
// handler forwarded so a test can assert the body reached it untouched.
type mockEntityScheduleClient struct {
	catalogueFn   func(ctx context.Context) ([]byte, error)
	assignmentsFn func(ctx context.Context, body []byte) ([]byte, error)
	absencesFn    func(ctx context.Context, body []byte) ([]byte, error)
	onDutyFn      func(ctx context.Context, at string) ([]byte, error)

	createFn   func(ctx context.Context, body []byte) ([]byte, error)
	updateFn   func(ctx context.Context, id string, body []byte) ([]byte, error)
	deleteFn   func(ctx context.Context, id, note string) ([]byte, error)
	activityFn func(ctx context.Context, teamKey, from, to string) ([]byte, error)
	leadFn     func(ctx context.Context) ([]byte, error)

	deleteAbsenceFn func(ctx context.Context, id, note string) ([]byte, error)
	createKindFn    func(ctx context.Context, body []byte) ([]byte, error)
	deleteKindFn    func(ctx context.Context, code string) ([]byte, error)
	applyFn         func(ctx context.Context, body []byte) ([]byte, error)
	absenceFn       func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockEntityScheduleClient) GetScheduleEditMarkers(context.Context, string, string) ([]byte, error) {
	return nil, nil
}

func (m *mockEntityScheduleClient) DeleteScheduleAbsence(ctx context.Context, id, note string) ([]byte, error) {
	if m.deleteAbsenceFn == nil {
		return nil, nil
	}
	return m.deleteAbsenceFn(ctx, id, note)
}

func (m *mockEntityScheduleClient) DeleteScheduleAbsenceKind(ctx context.Context, code string) ([]byte, error) {
	if m.deleteKindFn == nil {
		return nil, nil
	}
	return m.deleteKindFn(ctx, code)
}

func (m *mockEntityScheduleClient) CreateScheduleAbsenceKind(ctx context.Context, body []byte) ([]byte, error) {
	if m.createKindFn == nil {
		return nil, nil
	}
	return m.createKindFn(ctx, body)
}

func (m *mockEntityScheduleClient) ApplyScheduleAbsence(ctx context.Context, body []byte) ([]byte, error) {
	if m.absenceFn == nil {
		return nil, nil
	}
	return m.absenceFn(ctx, body)
}

func (m *mockEntityScheduleClient) ApplyScheduleRange(ctx context.Context, body []byte) ([]byte, error) {
	if m.applyFn == nil {
		return nil, nil
	}
	return m.applyFn(ctx, body)
}

func (m *mockEntityScheduleClient) GetMyLeadTeams(ctx context.Context) ([]byte, error) {
	if m.leadFn == nil {
		return nil, nil
	}
	return m.leadFn(ctx)
}

func (m *mockEntityScheduleClient) CreateScheduleAssignment(ctx context.Context, body []byte) ([]byte, error) {
	if m.createFn == nil {
		return nil, nil
	}
	return m.createFn(ctx, body)
}

func (m *mockEntityScheduleClient) UpdateScheduleAssignment(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.updateFn == nil {
		return nil, nil
	}
	return m.updateFn(ctx, id, body)
}

func (m *mockEntityScheduleClient) DeleteScheduleAssignment(ctx context.Context, id, note string) ([]byte, error) {
	if m.deleteFn == nil {
		return nil, nil
	}
	return m.deleteFn(ctx, id, note)
}

func (m *mockEntityScheduleClient) GetScheduleActivity(ctx context.Context, teamKey, from, to string) ([]byte, error) {
	if m.activityFn == nil {
		return nil, nil
	}
	return m.activityFn(ctx, teamKey, from, to)
}

func (m *mockEntityScheduleClient) GetScheduleCatalogue(ctx context.Context) ([]byte, error) {
	if m.catalogueFn != nil {
		return m.catalogueFn(ctx)
	}
	return []byte(`{"zones":[],"shifts":[],"absenceKinds":[]}`), nil
}

func (m *mockEntityScheduleClient) SearchScheduleAssignments(ctx context.Context, body []byte) ([]byte, error) {
	if m.assignmentsFn != nil {
		return m.assignmentsFn(ctx, body)
	}
	return []byte(`{"assignments":[],"count":0}`), nil
}

func (m *mockEntityScheduleClient) SearchScheduleAbsences(ctx context.Context, body []byte) ([]byte, error) {
	if m.absencesFn != nil {
		return m.absencesFn(ctx, body)
	}
	return []byte(`{"absences":[],"count":0}`), nil
}

func (m *mockEntityScheduleClient) GetScheduleOnDuty(ctx context.Context, at string) ([]byte, error) {
	if m.onDutyFn != nil {
		return m.onDutyFn(ctx, at)
	}
	return []byte(`{"assignments":[],"count":0}`), nil
}

func TestSearchScheduleAssignments(t *testing.T) {
	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects a body over the size limit", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search",
			strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
	})

	t.Run("rejects a body that is not JSON", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search",
			strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("forwards the search to entity-service unchanged", func(t *testing.T) {
		const payload = `{"from":"2026-09-21","to":"2026-09-27","family":"SRE","includeOvernight":true}`
		var captured []byte
		client := &mockEntityScheduleClient{
			assignmentsFn: func(_ context.Context, body []byte) ([]byte, error) {
				captured = body
				return []byte(`{"assignments":[{"id":"a"}],"count":1}`), nil
			},
		}
		h := NewScheduleHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(payload)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(captured) != payload {
			t.Fatalf("body was altered in transit:\n got %s\nwant %s", captured, payload)
		}
		if !strings.Contains(w.Body.String(), `"count":1`) {
			t.Fatalf("upstream response did not reach the caller: %s", w.Body.String())
		}
	})

	t.Run("surfaces an upstream failure rather than pretending it worked", func(t *testing.T) {
		client := &mockEntityScheduleClient{
			assignmentsFn: func(context.Context, []byte) ([]byte, error) {
				return nil, errors.New("upstream is down")
			},
		}
		h := NewScheduleHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/assignments/search", strings.NewReader(`{}`)))
		w := httptest.NewRecorder()
		h.SearchScheduleAssignments(w, r)
		if w.Code == http.StatusOK {
			t.Fatal("an upstream error must not come back as 200")
		}
	})
}

func TestGetScheduleCatalogue(t *testing.T) {
	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.GetScheduleCatalogue(w, httptest.NewRequest(http.MethodGet, "/team-schedule/catalogue", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("returns the catalogue", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			catalogueFn: func(context.Context) ([]byte, error) {
				return []byte(`{"zones":[{"code":"TZ1"}],"shifts":[],"absenceKinds":[]}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleCatalogue(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/catalogue", nil)))
		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), "TZ1") {
			t.Fatalf("catalogue did not reach the caller: %s", w.Body.String())
		}
	})
}

func TestGetScheduleOnDuty(t *testing.T) {
	t.Run("passes the instant through when one is asked for", func(t *testing.T) {
		var gotAt string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			onDutyFn: func(_ context.Context, at string) ([]byte, error) {
				gotAt = at
				return []byte(`{"assignments":[],"count":0}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleOnDuty(w, withUser(httptest.NewRequest(http.MethodGet,
			"/team-schedule/on-duty?at=2026-09-21T22%3A15%3A00Z", nil)))
		assertStatus(t, w, http.StatusOK)
		if gotAt != "2026-09-21T22:15:00Z" {
			t.Fatalf("want the instant forwarded verbatim, got %q", gotAt)
		}
	})

	t.Run("asks about now when no instant is given", func(t *testing.T) {
		var gotAt = "unset"
		h := NewScheduleHandler(&mockEntityScheduleClient{
			onDutyFn: func(_ context.Context, at string) ([]byte, error) {
				gotAt = at
				return []byte(`{"assignments":[],"count":0}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.GetScheduleOnDuty(w, withUser(httptest.NewRequest(http.MethodGet, "/team-schedule/on-duty", nil)))
		assertStatus(t, w, http.StatusOK)
		if gotAt != "" {
			t.Fatalf("want an empty instant so the service defaults to now, got %q", gotAt)
		}
	})
}

func TestSearchScheduleAbsences(t *testing.T) {
	t.Run("forwards the search and returns the response", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			absencesFn: func(_ context.Context, _ []byte) ([]byte, error) {
				return []byte(`{"absences":[{"id":"x"}],"count":1}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.SearchScheduleAbsences(w, withUser(httptest.NewRequest(http.MethodPost,
			"/team-schedule/absences/search", strings.NewReader(`{"from":"2026-09-21","to":"2026-09-21"}`))))
		assertStatus(t, w, http.StatusOK)
		if !strings.Contains(w.Body.String(), `"count":1`) {
			t.Fatalf("upstream response did not reach the caller: %s", w.Body.String())
		}
	})
}

func TestDeleteScheduleAbsence(t *testing.T) {
	t.Run("requires an authenticated user", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{})
		w := httptest.NewRecorder()
		h.DeleteScheduleAbsence(w, httptest.NewRequest(http.MethodDelete, "/team-schedule/absences/a1", nil))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("forwards the id and note, and answers 204", func(t *testing.T) {
		var gotID, gotNote string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			deleteAbsenceFn: func(_ context.Context, id, note string) ([]byte, error) {
				gotID, gotNote = id, note
				return nil, nil
			},
		})
		r := withUser(httptest.NewRequest(http.MethodDelete, "/team-schedule/absences/a1?note=removed", nil))
		r.SetPathValue("id", "a1")
		w := httptest.NewRecorder()
		h.DeleteScheduleAbsence(w, r)
		assertStatus(t, w, http.StatusNoContent)
		if gotID != "a1" || gotNote != "removed" {
			t.Fatalf("forwarded id %q note %q, want a1 / removed", gotID, gotNote)
		}
	})

	t.Run("a lead of another team gets the upstream 403", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			deleteAbsenceFn: func(context.Context, string, string) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusForbidden}
			},
		})
		r := withUser(httptest.NewRequest(http.MethodDelete, "/team-schedule/absences/a1", nil))
		r.SetPathValue("id", "a1")
		w := httptest.NewRecorder()
		h.DeleteScheduleAbsence(w, r)
		assertStatus(t, w, http.StatusForbidden)
	})
}

func TestCreateScheduleAbsenceKind(t *testing.T) {
	body := `{"shortCode":"Trn","label":"Training","bucket":"ALLOCATION","colourToken":"INT"}`

	t.Run("forwards the body untouched and answers 201", func(t *testing.T) {
		var got string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			createKindFn: func(_ context.Context, b []byte) ([]byte, error) {
				got = string(b)
				return []byte(`{"code":"TRAINING"}`), nil
			},
		})
		w := httptest.NewRecorder()
		h.CreateScheduleAbsenceKind(w, withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/absence-kinds", strings.NewReader(body))))
		assertStatus(t, w, http.StatusCreated)
		if got != body {
			t.Fatalf("forwarded %s, want %s", got, body)
		}
	})

	t.Run("a tag that already exists stays a 409", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			createKindFn: func(context.Context, []byte) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusConflict}
			},
		})
		w := httptest.NewRecorder()
		h.CreateScheduleAbsenceKind(w, withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/absence-kinds", strings.NewReader(body))))
		assertStatus(t, w, http.StatusConflict)
	})

	t.Run("rejects a body that is not JSON before calling upstream", func(t *testing.T) {
		called := false
		h := NewScheduleHandler(&mockEntityScheduleClient{
			createKindFn: func(context.Context, []byte) ([]byte, error) { called = true; return nil, nil },
		})
		w := httptest.NewRecorder()
		h.CreateScheduleAbsenceKind(w, withUser(httptest.NewRequest(http.MethodPost, "/team-schedule/absence-kinds", strings.NewReader("{not json"))))
		assertStatus(t, w, http.StatusBadRequest)
		if called {
			t.Fatal("an invalid body reached entity-service")
		}
	})
}

func TestDeleteScheduleAbsenceKind(t *testing.T) {
	t.Run("forwards the code and answers 204", func(t *testing.T) {
		var got string
		h := NewScheduleHandler(&mockEntityScheduleClient{
			deleteKindFn: func(_ context.Context, code string) ([]byte, error) { got = code; return nil, nil },
		})
		r := withUser(httptest.NewRequest(http.MethodDelete, "/team-schedule/absence-kinds/TRAINING", nil))
		r.SetPathValue("code", "TRAINING")
		w := httptest.NewRecorder()
		h.DeleteScheduleAbsenceKind(w, r)
		assertStatus(t, w, http.StatusNoContent)
		if got != "TRAINING" {
			t.Fatalf("forwarded %q, want TRAINING", got)
		}
	})

	t.Run("a tag still in use stays a 409", func(t *testing.T) {
		h := NewScheduleHandler(&mockEntityScheduleClient{
			deleteKindFn: func(context.Context, string) ([]byte, error) {
				return nil, &apierror.Error{StatusCode: http.StatusConflict}
			},
		})
		r := withUser(httptest.NewRequest(http.MethodDelete, "/team-schedule/absence-kinds/TRAINING", nil))
		r.SetPathValue("code", "TRAINING")
		w := httptest.NewRecorder()
		h.DeleteScheduleAbsenceKind(w, r)
		assertStatus(t, w, http.StatusConflict)
	})
}
