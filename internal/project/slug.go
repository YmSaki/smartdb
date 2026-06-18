package project

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
)

func slug(name string) string {
	h := fnv.New64a()

	seed := fmt.Sprintf(
		"%s-%d",
		strings.ToLower(name),
		time.Now().UnixNano(),
	)

	h.Write([]byte(seed))

	return fmt.Sprintf("%016x", h.Sum64())
}
