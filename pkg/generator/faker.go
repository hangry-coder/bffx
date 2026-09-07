package generator

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var (
	firstNames = []string{"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda", "William", "Elizabeth", "David", "Barbara", "Richard", "Susan", "Joseph", "Jessica", "Thomas", "Sarah", "Charles", "Karen"}
	lastNames  = []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore", "Jackson", "Martin"}
	domains    = []string{"gmail.com", "outlook.com", "icloud.com", "hey.com", "protonmail.com"}
	words      = []string{"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit", "curabitur", "vel", "hendrerit", "libero"}
)

func FakeValue(resourceName, fieldName, fieldType string) string {
	// #nosec G404
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	
	name := strings.ToLower(fieldName)
	res := strings.ToLower(resourceName)
	
	switch {
	case strings.Contains(name, "plan") || (res == "plan" && name == "name"):
		plans := []string{"Basic Plan", "Premium Plan", "Elite Plan", "Free Tier", "Pro Monthly"}
		return plans[r.Intn(len(plans))]
	
	case strings.Contains(name, "email"):
		fn := strings.ToLower(firstNames[r.Intn(len(firstNames))])
		ln := strings.ToLower(lastNames[r.Intn(len(lastNames))])
		return fmt.Sprintf("%s.%s@%s", fn, ln, domains[r.Intn(len(domains))])
		
	case strings.Contains(name, "password"):
		return "password123" // Default for development seeds

	case strings.Contains(name, "interval"):
		intervals := []string{"monthly", "yearly", "weekly", "quarterly"}
		return intervals[r.Intn(len(intervals))]

	case strings.Contains(name, "screen"):


		screens := []string{"Home", "Settings", "Profile", "Onboarding", "Weather", "Dashboard", "Search"}
		return screens[r.Intn(len(screens))]

	case strings.Contains(name, "action"):
		actions := []string{"Click", "Submit", "Swipe", "Refresh", "Login", "Logout"}
		return actions[r.Intn(len(actions))]

	case strings.Contains(name, "name") || strings.Contains(name, "title"):
		fn := firstNames[r.Intn(len(firstNames))]
		ln := lastNames[r.Intn(len(lastNames))]
		return fmt.Sprintf("%s %s", fn, ln)


	case name == "enabled" || name == "is_verified" || name == "active" || strings.HasPrefix(name, "is_"):
		if r.Intn(2) == 0 {
			return "false"
		}
		return "true"
		
	case strings.Contains(name, "phone") || strings.Contains(name, "whatsapp"):
		return fmt.Sprintf("+1%010d", r.Int63n(10000000000))
		
	case strings.Contains(name, "price") || strings.Contains(name, "amount"):
		return fmt.Sprintf("%.2f", float64(r.Intn(10000))/100.0)
		
	case strings.Contains(name, "description") || strings.Contains(name, "body") || strings.Contains(name, "bio"):
		var s []string
		for i := 0; i < 10; i++ {
			s = append(s, words[r.Intn(len(words))])
		}
		return strings.Join(s, " ")
		
	case strings.Contains(name, "url") || strings.Contains(name, "link"):
		return fmt.Sprintf("https://example.com/%s", words[r.Intn(len(words))])

	case strings.Contains(name, "code") || strings.Contains(name, "otp"):
		return fmt.Sprintf("%06d", r.Intn(1000000))

	case strings.Contains(name, "role"):
		roles := []string{"admin", "member", "editor", "viewer"}
		return roles[r.Intn(len(roles))]

	case strings.Contains(name, "status"):
		statuses := []string{"active", "pending", "archived", "completed"}
		return statuses[r.Intn(len(statuses))]
	}


	// Fallback by type
	switch fieldType {
	case "int":
		return fmt.Sprintf("%d", r.Intn(1000))
	case "float":
		return fmt.Sprintf("%.2f", float64(r.Intn(10000))/100.0)
	case "bool":
		if r.Intn(2) == 0 {
			return "false"
		}
		return "true"
	default:
		return words[r.Intn(len(words))]
	}
}
