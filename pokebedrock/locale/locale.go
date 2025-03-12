package locale

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/text"
	"golang.org/x/text/language"
)

// localeData ...
type localeData map[string]string

// locales ...
var locales = make(map[language.Tag]localeData)

// Register ...
func Register(lang language.Tag, filePath string) error {
	file, err := os.Open(fmt.Sprintf("%s/%s.lang", filePath, lang.String()))
	if err != nil {
		return fmt.Errorf("could not open lang file: %w", err)
	}
	defer file.Close()

	data := make(localeData)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		data[key] = value
	}
	if err = scanner.Err(); err != nil {
		return fmt.Errorf("error reading lang file: %w", err)
	}

	locales[lang] = data
	return nil
}

// Translate ...
func Translate(key string, args ...any) string {
	return text.Colourf(TranslateL(language.English, key, args...))
}

// TranslateL ...
func TranslateL(lang language.Tag, key string, args ...any) string {
	locale, ok := locales[lang]
	if !ok {
		locale = locales[language.English]
	}

	translation, ok := locale[key]
	if !ok {
		return fmt.Sprintf("missing translation for '%s'", key)
	}

	for i, arg := range args {
		placeholder := fmt.Sprintf("%%%d", i+1)
		translation = strings.ReplaceAll(translation, placeholder, fmt.Sprintf("%v", arg))
	}
	return translation
}
