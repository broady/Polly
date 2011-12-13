package hello

import (
	"fmt"
	"template"
	"http"

	"appengine"
	_ "appengine/datastore"
	"appengine/user"
)

type Poll struct {
	Name    string
	Options []Option
}

type Option struct {
	Text  string
	Image string
}

type Vote struct {
	Poll    Poll
	Options []Option
}

var (
	templates = template.SetMust(template.ParseTemplateGlob("templates/*.html"))
)

func init() {
	http.HandleFunc("/", listHandler)
	http.HandleFunc("/poll", pollHandler)
	http.HandleFunc("/new", newHandler)
	http.HandleFunc("/add", addHandler)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, world!")
}

func newHandler(w http.ResponseWriter, r *http.Request) {
	templates.Execute(w, "new.html", nil)
}

func pollHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, world!")
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)

	v := map[string]string{
		"foo": "bar",
		"u":   u.Email,
	}

	templates.Execute(w, "list.html", v)
}
