package translation

import "golang.org/x/text/language"

type str string

// Resolve returns the translation identifier as a string.
func (s str) Resolve(language.Tag) string { return string(s) }
