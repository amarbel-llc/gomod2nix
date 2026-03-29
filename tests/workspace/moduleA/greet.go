package moduleA

import "golang.org/x/text/cases"
import "golang.org/x/text/language"

func Greet(name string) string {
	c := cases.Title(language.English)
	return "Hello, " + c.String(name) + "!"
}
