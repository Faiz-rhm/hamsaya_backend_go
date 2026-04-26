package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hamsaya/backend/internal/mocks"
	"github.com/stretchr/testify/mock"
)

// FuzzAuthHandler_Register sends arbitrary byte sequences to the registration
// endpoint and asserts the server never panics. Validation is expected to
// reject most inputs with 4xx; the goal is to surface crashes (nil derefs,
// unbounded allocations, infinite loops in the validator) rather than to
// verify any specific response shape.
//
// Run with: go test -run=^$ -fuzz=FuzzAuthHandler_Register ./internal/handlers/
func FuzzAuthHandler_Register(f *testing.F) {
	seeds := [][]byte{
		[]byte(``),
		[]byte(`{}`),
		[]byte(`{"email":"a@b.c","password":"Password1!","first_name":"Foo","last_name":"Bar","latitude":1,"longitude":2}`),
		[]byte(`{"email":null,"password":null}`),
		[]byte(`{"email":"  ","password":" "}`),
		[]byte(`{"latitude":"not-a-number"}`),
		[]byte("{\"x\":\"`'\\\"\"}"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	// Oversized UTF-8 payload built at runtime — string literals can't hold NUL.
	bigName := bytes.Repeat([]byte("A"), 10000)
	huge := append([]byte(`{"first_name":"`), append(bigName, []byte(`"}`)...)...)
	f.Add(huge)

	f.Fuzz(func(t *testing.T, body []byte) {
		// Cap the input — multi-MB bodies blow up fuzz runtime without finding new bugs.
		if len(body) > 64*1024 {
			t.Skip()
		}

		userRepo := &mocks.MockUserRepository{}
		// All repo methods are best-effort: a "happy-path" fuzz input may reach
		// the persistence layer; we just want to confirm no panic.
		userRepo.On("GetByEmail", mock.Anything, mock.Anything).Maybe().Return(nil, errNotFound{})
		userRepo.On("GetByEmailIncludingDeleted", mock.Anything, mock.Anything).Maybe().Return(nil, errNotFound{})
		userRepo.On("CreateUserWithProfile", mock.Anything, mock.Anything, mock.Anything).Maybe().Return(nil)
		userRepo.On("Create", mock.Anything, mock.Anything).Maybe().Return(nil)
		userRepo.On("CreateSession", mock.Anything, mock.Anything).Maybe().Return(nil)
		userRepo.On("UpdateLastLogin", mock.Anything, mock.Anything).Maybe().Return(nil)
		userRepo.On("GetProfileByUserID", mock.Anything, mock.Anything).Maybe().Return(nil, errNotFound{})

		r := newAuthTestRouter(t, userRepo)

		req, _ := http.NewRequest(http.MethodPost,
			"/api/v1/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		// Contract: never panic. Recover ➜ t.Fatal records the input under testdata/fuzz/.
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("handler panicked on input %q: %v", body, rec)
			}
		}()

		r.ServeHTTP(w, req)

		if w.Code < 100 || w.Code >= 600 {
			t.Fatalf("invalid status code %d for input %q", w.Code, body)
		}
	})
}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }
