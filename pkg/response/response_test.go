package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhata/AutoDreamApplier/pkg/response"
)

// ── helpers ───────────────────────────────────────────────────────────────────

type apiResp struct {
	Success bool `json:"success"`
	Data    interface{}
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details string `json:"details,omitempty"`
	} `json:"error,omitempty"`
	Meta *struct {
		Page       int   `json:"page,omitempty"`
		PerPage    int   `json:"per_page,omitempty"`
		Total      int64 `json:"total,omitempty"`
		TotalPages int   `json:"total_pages,omitempty"`
	} `json:"meta,omitempty"`
}

func record(handler func(w http.ResponseWriter)) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handler(rr)
	return rr
}

func decode(t *testing.T, rr *httptest.ResponseRecorder) apiResp {
	t.Helper()
	var out apiResp
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func assertContentType(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/json")
	}
}

// ── JSON ──────────────────────────────────────────────────────────────────────

func TestJSON_StatusCode(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.JSON(w, http.StatusOK, map[string]string{"key": "value"})
	})
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rr.Code, http.StatusOK)
	}
}

func TestJSON_ContentType(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.JSON(w, http.StatusOK, nil)
	})
	assertContentType(t, rr)
}

func TestJSON_SuccessTrue_2xx(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.JSON(w, http.StatusCreated, "created")
	})
	out := decode(t, rr)
	if !out.Success {
		t.Error("success should be true for 2xx status")
	}
}

func TestJSON_SuccessFalse_4xx(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.JSON(w, http.StatusBadRequest, nil)
	})
	out := decode(t, rr)
	if out.Success {
		t.Error("success should be false for 4xx status")
	}
}

func TestJSON_SuccessFalse_5xx(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.JSON(w, http.StatusInternalServerError, nil)
	})
	out := decode(t, rr)
	if out.Success {
		t.Error("success should be false for 5xx status")
	}
}

func TestJSON_NilData(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.JSON(w, http.StatusOK, nil)
	})
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
}

// ── JSONWithMeta ──────────────────────────────────────────────────────────────

func TestJSONWithMeta_StatusAndContentType(t *testing.T) {
	t.Parallel()
	meta := &response.Meta{Page: 1, PerPage: 10, Total: 100, TotalPages: 10}
	rr := record(func(w http.ResponseWriter) {
		response.JSONWithMeta(w, http.StatusOK, []string{"a", "b"}, meta)
	})
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d; want 200", rr.Code)
	}
	assertContentType(t, rr)
}

func TestJSONWithMeta_SuccessAlwaysTrue(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.JSONWithMeta(w, http.StatusOK, nil, nil)
	})
	out := decode(t, rr)
	if !out.Success {
		t.Error("JSONWithMeta success should always be true")
	}
}

func TestJSONWithMeta_MetaIsPresent(t *testing.T) {
	t.Parallel()
	meta := &response.Meta{Page: 2, PerPage: 20, Total: 200, TotalPages: 10}
	rr := record(func(w http.ResponseWriter) {
		response.JSONWithMeta(w, http.StatusOK, "data", meta)
	})
	out := decode(t, rr)
	if out.Meta == nil {
		t.Fatal("meta should not be nil")
	}
	if out.Meta.Page != 2 {
		t.Errorf("meta.page = %d; want 2", out.Meta.Page)
	}
}

// ── Error ─────────────────────────────────────────────────────────────────────

func TestError_StatusCode(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid input")
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", rr.Code)
	}
}

func TestError_SuccessFalse(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.Error(w, http.StatusBadRequest, "CODE", "msg")
	})
	out := decode(t, rr)
	if out.Success {
		t.Error("Error response success should be false")
	}
}

func TestError_ErrorFields(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
	})
	out := decode(t, rr)
	if out.Error == nil {
		t.Fatal("error field should not be nil")
	}
	if out.Error.Code != "NOT_FOUND" {
		t.Errorf("error.code = %q; want NOT_FOUND", out.Error.Code)
	}
	if out.Error.Message != "resource not found" {
		t.Errorf("error.message = %q; want 'resource not found'", out.Error.Message)
	}
}

// ── ErrorWithDetails ──────────────────────────────────────────────────────────

func TestErrorWithDetails_DetailsPresent(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.ErrorWithDetails(w, http.StatusUnprocessableEntity, "VALIDATION", "invalid data", "field X must be non-empty")
	})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d; want 422", rr.Code)
	}
	out := decode(t, rr)
	if out.Error == nil {
		t.Fatal("error field should not be nil")
	}
	if out.Error.Details != "field X must be non-empty" {
		t.Errorf("error.details = %q; want 'field X must be non-empty'", out.Error.Details)
	}
}

// ── Convenience helpers ───────────────────────────────────────────────────────

func TestBadRequest(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.BadRequest(w, "bad input")
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("BadRequest status = %d; want 400", rr.Code)
	}
	out := decode(t, rr)
	if out.Error == nil || out.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected BAD_REQUEST error code, got %v", out.Error)
	}
}

func TestUnauthorized(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.Unauthorized(w, "not authenticated")
	})
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Unauthorized status = %d; want 401", rr.Code)
	}
	out := decode(t, rr)
	if out.Error == nil || out.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected UNAUTHORIZED error code")
	}
}

func TestForbidden(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.Forbidden(w, "no access")
	})
	if rr.Code != http.StatusForbidden {
		t.Errorf("Forbidden status = %d; want 403", rr.Code)
	}
	out := decode(t, rr)
	if out.Error == nil || out.Error.Code != "FORBIDDEN" {
		t.Errorf("expected FORBIDDEN error code")
	}
}

func TestNotFound(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.NotFound(w, "not found")
	})
	if rr.Code != http.StatusNotFound {
		t.Errorf("NotFound status = %d; want 404", rr.Code)
	}
	out := decode(t, rr)
	if out.Error == nil || out.Error.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND error code")
	}
}

func TestConflict(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.Conflict(w, "already exists")
	})
	if rr.Code != http.StatusConflict {
		t.Errorf("Conflict status = %d; want 409", rr.Code)
	}
	out := decode(t, rr)
	if out.Error == nil || out.Error.Code != "CONFLICT" {
		t.Errorf("expected CONFLICT error code")
	}
}

func TestInternalError(t *testing.T) {
	t.Parallel()
	rr := record(func(w http.ResponseWriter) {
		response.InternalError(w, "something broke")
	})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("InternalError status = %d; want 500", rr.Code)
	}
	out := decode(t, rr)
	if out.Error == nil || out.Error.Code != "INTERNAL_ERROR" {
		t.Errorf("expected INTERNAL_ERROR error code")
	}
}

// ── Content-Type on all helpers ───────────────────────────────────────────────

func TestAllHelpers_ContentType(t *testing.T) {
	t.Parallel()
	helpers := []func(w http.ResponseWriter){
		func(w http.ResponseWriter) { response.BadRequest(w, "msg") },
		func(w http.ResponseWriter) { response.Unauthorized(w, "msg") },
		func(w http.ResponseWriter) { response.Forbidden(w, "msg") },
		func(w http.ResponseWriter) { response.NotFound(w, "msg") },
		func(w http.ResponseWriter) { response.Conflict(w, "msg") },
		func(w http.ResponseWriter) { response.InternalError(w, "msg") },
	}
	for i, h := range helpers {
		rr := record(h)
		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("helper[%d] Content-Type = %q; want application/json", i, ct)
		}
	}
}
