package cli

import (
	"errors"

	"github.com/tamnd/codechef-cli/codechef"
)

func isNotFound(err error) bool {
	return errors.Is(err, codechef.ErrNotFound)
}
