package trace

import (
	"errors"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.com/gorib/failure"
)

func TestARefusalIsNamedByItsCodeAndTypeNotItsText(t *testing.T) {
	onWire := func(id string) error {
		st, _ := status.New(codes.NotFound, "contract "+id+" not found").WithDetails(&errdetails.ErrorInfo{
			Reason: "CONTRACT_NOT_FOUND", Domain: failure.Kind,
		})
		return st.Err()
	}
	for _, err := range []error{onWire("55"), onWire("56")} {
		if got := refusal(err); got != "NotFound contract_not_found" {
			t.Errorf("refusal(%v) = %q", err, got)
		}
	}
	if got := refusal(errors.New("boom")); got != "failed" {
		t.Errorf("a plain error is %q, want failed", got)
	}
}
