package e2e

type AuthMutation struct {
	Name     string
	Strategy string
}

func AllMutations() []AuthMutation {
	return []AuthMutation{
		{Name: "Mandatory", Strategy: "mandatory"},
		{Name: "Optional", Strategy: "optional"},
		{Name: "Anonymous", Strategy: ""},
	}
}
