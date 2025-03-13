package locale

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/text"
	"golang.org/x/text/language"
)

// localeData represents a mapping of translation keys to their respective values for a specific language.
type localeData map[string]string

// locales is a map of registered locales keyed by language tags.
// It holds all the locale data for supported languages.
var locales = make(map[language.Tag]localeData)

// Register registers a new locale from the specified language file path.
// It reads the language file and populates the locale data for the provided language tag.
// The language file should be in the format "key=value" where each key corresponds to a translation key.
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

// Translate translates a key to the default language (English) and formats it with the provided arguments.
// It uses the TranslateL function internally for the English translation.
func Translate(key string, args ...any) string {
	return text.Colourf(TranslateL(language.English, key, args...))
}

// TranslateL translates a key to a specified language and formats it with the provided arguments.
// If the language data is unavailable, it falls back to the English translation.
// It supports placeholders in the translation string, which are replaced by the arguments.
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
