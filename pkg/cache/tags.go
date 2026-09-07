package cache

// ScreenTag returns the canonical cache tag for a screen response.
func ScreenTag(screen string) string {
	return "screen:" + screen
}

// SectionTag returns the canonical cache tag for a screen section response.
func SectionTag(screen, section string) string {
	return "screen:" + screen + ":section:" + section
}
