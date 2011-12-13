package polly

import (
	"fmt"
	"template"
	"os"
	"strings"
	"strconv"
	"http"

	"appengine"
	"appengine/datastore"
	"appengine/user"
)

type Poll struct {
	Name    string
	Options []*datastore.Key
	Owner   string
}

type Option struct {
	Text  string
	Image string
	Votes int
}

type Vote struct {
	Poll    Poll
	Options []*datastore.Key
}

var (
	templates = template.SetMust(template.ParseTemplateGlob("templates/*.html"))
)

func init() {
	http.HandleFunc("/poll/", pollHandler)
	http.HandleFunc("/new", newHandler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/vote", voteHandler)
	http.HandleFunc("/", listHandler)
}

func voteHandler(w http.ResponseWriter, r *http.Request) {
}
func addHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	r.ParseForm()

	title := r.FormValue("title")
	if title == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Must enter title. Hit back and try again.")
		return
	}

	options := make([]*datastore.Key, 10)
	for i := 1; ; i++ {
		name := r.FormValue(fmt.Sprintf("title%d", i))
		if name == "" {
			break
		}
		img := r.FormValue(fmt.Sprintf("img%d", i))
		option := Option{
			Text:  name,
			Image: img,
			Votes: 0,
		}
		key, err := datastore.Put(c, datastore.NewIncompleteKey(c, "option", nil), &option)
		if err != nil {
			writeError(c, w, err)
			return
		}
		options[i-1] = key
	}

	poll := Poll{
		Name:    title,
		Options: options,
		Owner:   u.Id,
	}

	key, err := datastore.Put(c, datastore.NewIncompleteKey(c, "poll", nil), &poll)
	if err != nil {
		writeError(c, w, err)
		return
	}
	url := fmt.Sprintf("/poll/%d", key.IntID())
	http.Redirect(w, r, url, http.StatusFound)
}

func newHandler(w http.ResponseWriter, r *http.Request) {
	templates.Execute(w, "new.html", nil)
}

func pollHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		http.Error(w, "Must provide a poll id.", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi64(parts[2])
	if err != nil {
		http.Error(w, "Invalid id. Must be an integer.", http.StatusBadRequest)
		return
	}
	poll := new(Poll)
	key := datastore.NewKey(c, "poll", "", id, nil)
	err = datastore.Get(c, key, poll)
	if err != nil {
		writeError(c, w, err)
		return
	}

	options := make([]interface{}, len(poll.Options))
	for i := range options {
		options[i] = new(Option)
	}
	err = datastore.GetMulti(c, poll.Options, options)
	if err != nil {
		writeError(c, w, err)
		return
	}

	u := user.Current(c)

	v := map[string]interface{}{
		"poll": poll,
		"options": options,
		"super": user.IsAdmin(c) || poll.Owner == u.Id,
	}

	templates.Execute(w, "poll.html", v)
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

func writeError(c appengine.Context, w http.ResponseWriter, err os.Error) {
	c.Errorf(err.String())
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprint(w, err.String())
}
