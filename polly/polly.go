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
	Name       string
	Owner      string
	Options    int64
	TotalVotes int64

	Id int64
}

type Option struct {
	Text  string
	Image string
	Votes int

	Poll *datastore.Key
	Id   int64
}

type Vote struct {
	Owner string

	Option *datastore.Key
}

var (
	templates = template.SetMust(template.ParseTemplateGlob("templates/*.html"))
)

func init() {
	http.HandleFunc("/poll/", pollHandler)
	http.HandleFunc("/vote/", voteHandler)
	http.HandleFunc("/new", newHandler)
	http.HandleFunc("/add", addHandler)
	http.HandleFunc("/", listHandler)
}

func voteHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Must provide a poll id and vote id.", http.StatusBadRequest)
		return
	}
	pollId, err := strconv.Atoi64(parts[2])
	if err != nil {
		writeError(c, w, err)
	}
	voteId, err := strconv.Atoi64(parts[3])
	if err != nil {
		writeError(c, w, err)
	}

	userId := user.Current(c).Id
	pollKey := datastore.NewKey(c, "poll", "", pollId, nil)
	optionKey := datastore.NewKey(c, "option", "", voteId, pollKey)

	voteKey := datastore.NewKey(c, "vote", userId, 0, pollKey)

	vote := new(Vote)
	option := new(Option)
	err = datastore.RunInTransaction(c, func(c appengine.Context) os.Error {
		// Note: this function's argument c shadows the variable c
		//       from the surrounding function.
		err := datastore.Get(c, optionKey, option)
		if err != nil {
			return err
		}
		err = datastore.Get(c, voteKey, vote)
		if err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		if vote.Option != nil {
			oldOption := new(Option)
			err := datastore.Get(c, vote.Option, oldOption)
			if err != nil {
				return err
			}
			oldOption.Votes--
			_, err = datastore.Put(c, vote.Option, oldOption)
			if err != nil {
				return err
			}
		} else {
			poll := new(Poll)
			err := datastore.Get(c, pollKey, poll)
			if err != nil {
				return err
			}
			poll.TotalVotes++
			_, err = datastore.Put(c, pollKey, poll)
			if err != nil {
				return err
			}
		}
		option.Votes++
		vote.Option = optionKey
		_, err = datastore.Put(c, optionKey, option)
		if err != nil {
			return err
		}
		_, err = datastore.Put(c, voteKey, vote)
		return err
	}, nil)
	if err != nil {
		writeError(c, w, err)
		return
	}
	fmt.Fprint(w, "OK")
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	r.ParseForm()

	title := r.FormValue("title")
	if title == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Must enter title.")
		return
	}

	poll := Poll{
		Name:       title,
		Owner:      u.Id,
		TotalVotes: 0,
	}

	pollKey, err := datastore.Put(c, datastore.NewIncompleteKey(c, "poll", nil), &poll)
	if err != nil {
		writeError(c, w, err)
		return
	}
	for i := 1; ; i++ {
		name := r.FormValue(fmt.Sprintf("title%d", i))
		if name == "" {
			if i < 3 {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "Poll must have at least two options.")
				return
			}
			break
		}
		img := r.FormValue(fmt.Sprintf("img%d", i))
		option := Option{
			Text:  name,
			Image: img,
			Votes: 0,
		}
		_, err := datastore.Put(c, datastore.NewKey(c, "option", "", int64(i), pollKey), &option)
		if err != nil {
			writeError(c, w, err)
			return
		}
		poll.Options++
	}

	pollKey, err = datastore.Put(c, pollKey, &poll)
	if err != nil {
		writeError(c, w, err)
		return
	}

	url := fmt.Sprintf("/poll/%d", pollKey.IntID())
	fmt.Fprint(w, url)
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
	poll, err := fetchPoll(c, parts[2])
	if err != nil {
		writeError(c, w, err)
		return
	}

	options, err := fetchOptions(c, poll)
	if err != nil {
		writeError(c, w, err)
		return
	}

	u := user.Current(c)

	v := map[string]interface{}{
		"poll":    poll,
		"options": options,
		"super":   user.IsAdmin(c) || poll.Owner == u.Id,
		"pollid":  parts[2],
	}

	templates.Execute(w, "poll.html", v)
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)

	iter := datastore.NewQuery("poll").
		Filter("Owner =", u.Id).
		Run(c)

	polls := make([]Poll, 0)
	for i := 0; ; i++ {
		var poll Poll
		key, err := iter.Next(&poll)
		if err == datastore.Done {
			break
		}
		if err != nil {
			writeError(c, w, err)
			return
		}
		poll.Id = key.IntID()
		polls = append(polls, poll)
	}

	templates.Execute(w, "list.html", polls)
}

func writeError(c appengine.Context, w http.ResponseWriter, err os.Error) {
	c.Errorf(err.String())
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprint(w, err.String())
}

func fetchPoll(c appengine.Context, strid string) (*Poll, os.Error) {
	id, err := strconv.Atoi64(strid)
	if err != nil {
		return nil, err
	}
	poll := new(Poll)
	key := datastore.NewKey(c, "poll", "", id, nil)
	err = datastore.Get(c, key, poll)
	if err != nil {
		return nil, err
	}
	poll.Id = id
	return poll, nil
}

func fetchOptions(c appengine.Context, poll *Poll) ([]*Option, os.Error) {
	fmt.Printf("%d\n", poll.Id)
	fmt.Printf("%d\n", poll.Options)

	dst := make([]interface{}, poll.Options)
	options := make([]*Option, poll.Options)
	keys := make([]*datastore.Key, poll.Options)

	pollKey := datastore.NewKey(c, "poll", "", poll.Id, nil)
	fmt.Printf("%v\n", pollKey)
	fmt.Printf("%v\n", keys)
	for i := range keys {
		keys[i] = datastore.NewKey(c, "option", "", int64(i+1), pollKey)
		dst[i] = new(Option)
	}
	fmt.Printf("%v\n", keys)
	err := datastore.GetMulti(c, keys, dst)
	fmt.Printf("%s %v\n", err, dst)
	if err != nil {
		return nil, err
	}
	for i := range dst {
		opt, ok := dst[i].(*Option)
		if ok {
			options[i] = opt
			opt.Poll = pollKey
			opt.Id = keys[i].IntID()
		} else {
			fmt.Printf("not ok!")
		}
	}

	return options, err
}

func fetchVote(c, poll *Poll, user string) {
}
