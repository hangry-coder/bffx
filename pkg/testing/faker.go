package testing

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/brianvoe/gofakeit/v6"
)

var (
	sequences sync.Map // map[string]*int64
)

// NextSequence returns a thread-safe incrementing integer for a given key.
// Useful for ensuring unique fields like emails in parallel tests.
func NextSequence(name string) int64 {
	val, _ := sequences.LoadOrStore(name, new(int64))
	return atomic.AddInt64(val.(*int64), 1)
}

// FakeValue generates a realistic value based on the field name.
func FakeValue(fieldName string) any {
	name := strings.ToLower(fieldName)

	switch {
	case strings.Contains(name, "email"):
		return fmt.Sprintf("test-user-%d@example.com", NextSequence("email"))
	case name == "name" || strings.Contains(name, "username"):
		return gofakeit.Name()
	case strings.Contains(name, "phone"):
		return gofakeit.Phone()
	case strings.Contains(name, "title"):
		return gofakeit.Sentence(3)
	case strings.Contains(name, "body") || strings.Contains(name, "content") || strings.Contains(name, "description"):
		return gofakeit.Paragraph(1, 3, 5, " ")
	case strings.Contains(name, "url") || strings.Contains(name, "website"):
		return gofakeit.URL()
	case strings.Contains(name, "city"):
		return gofakeit.City()
	case strings.Contains(name, "country"):
		return gofakeit.Country()
	case strings.HasSuffix(name, "_id") || name == "id":
		return gofakeit.UUID()
	default:
		return gofakeit.Word()
	}
}
