package repo

import (
	"errors"
	"testing"

	"github.com/lib/pq"
	"github.com/zhanglei10281852-gif/smart-home-operations/internal/model"
)

func TestDatabaseErrorsExposeStablePublicCategory(t *testing.T) {
	cases := []struct {
		code string
		want error
	}{
		{code: "23505", want: model.ErrConflict},
		{code: "23503", want: model.ErrConflict},
		{code: "23514", want: model.ErrInvalid},
		{code: "22P02", want: model.ErrInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			cause := &pq.Error{Code: pq.ErrorCode(tc.code), Message: "sensitive database detail"}
			got := classifyWriteError(cause)
			if !errors.Is(got, tc.want) || !errors.Is(got, cause) {
				t.Fatalf("classification=%v", got)
			}
			if got.Error() != tc.want.Error() {
				t.Fatalf("public error leaked database detail: %q", got)
			}
		})
	}
}
