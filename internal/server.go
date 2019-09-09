package wtfd

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"strconv"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const (
  DefaultPort = int64(80)
)

var (
	// key must be 16, 24 or 32 bytes long (AES-128, AES-192 or AES-256)
	key                = []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	store              = sessions.NewCookieStore(key)
	ErrUserExisting    = errors.New("User with this name exists")
	ErrWrongPassword   = errors.New("Wrong Password")
	ErrUserNotExisting = errors.New("User with this name does not exist")
	users              = Users{}
	sshHost            = "localhost:2222"
	challs             = Challenges{}
)

type Users []User
type Challenges []Challenge

type Challenge struct {
	Title       string `json:"title"`
	Description string `json:"desc"`
	Flag        string `json:"flag"`
	Points      int    `json:"points"`
	Uri         string `json:"uri"`
	HasUri		bool					// This emerges from Uri != ""
}

type User struct {
	Name      string
	Hash      []byte
	Completed []*Challenge
}

type MainPageData struct {
	PageTitle  string
	Challenges []Challenge
	User       User
        IsUser     bool
}

/**
 * Fill host into each challenge's Uri field and set HasUri
 */
func (c Challenges) FillChallengeUri(host string) {
	for i, _ := range c {
		if c[i].Uri != "" {
			c[i].HasUri = true
			c[i].Uri = fmt.Sprintf(c[i].Uri, host)
		}else {
			c[i].HasUri = false
		}
	}
}

func (u *Users) Contains(username string) bool {
	_, err := u.Get(username)
	return err != nil
}

func (u *Users) Get(username string) (User, error) {
	for _, user := range *u {
		if user.Name == username {
			return user, nil
		}
	}
	return User{}, ErrUserNotExisting

}

func (u *Users) Login(username, passwd string) (User, error) {
	user, err := u.Get(username)
	if err != nil {
		return User{}, err
	}
	if pwdRight := user.ComparePassword(passwd); !pwdRight {
		return User{}, ErrWrongPassword
	}
	return user, nil

}

func (u *User) ComparePassword(password string) bool {
	return bcrypt.CompareHashAndPassword(u.Hash, []byte(password)) == nil
}

func (u *User) New(name, password string) (User, error) {
	if users.Contains(name) {
		return User{}, ErrUserExisting
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return User{}, err
	}

	return User{Name: name, Hash: hash}, nil

}

func mainpage(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "auth")
	val := session.Values["User"]
        user := &User{}
        _, ok := val.(*User)
        t, err := template.ParseFiles("html/index.html")
        if err != nil {
          fmt.Println(err)
        }
	data := MainPageData{
		PageTitle:  "foss-ag O-Phasen CTF",
		Challenges: challs,
		User:       *user,
                IsUser: ok,

	}
        t.Execute(w,data)

}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid Request")

	} else {
		session, _ := store.Get(r, "auth")
                if _, ok := session.Values["User"].(*User); ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Already logged in")
		} else {
			u, err := users.Login(r.Form.Get("username"), r.Form.Get("password"))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Server Error: %v", err)
			} else {
				session.Values["User"] = u
				session.Save(r, w)
				http.Redirect(w, r, "/", http.StatusFound)

			}

		}

	}

}

func logout(w http.ResponseWriter, r *http.Request) {

}

func Server() error {
	gob.Register(&User{})

        // Loading challs file
	challsFile, err := os.Open("challs.json")
	if err != nil {
		return err
	}
	defer challsFile.Close()
	challsFileBytes, _ := ioutil.ReadAll(challsFile)
	if err := json.Unmarshal(challsFileBytes, &challs); err != nil {
		return err
	}

		// Fill in sshHost
	challs.FillChallengeUri(sshHost)

        // Http sturf
	r := mux.NewRouter()
	r.HandleFunc("/", mainpage)
	r.HandleFunc("/login", login)
        // static
        r.PathPrefix("/static").Handler(
          http.StripPrefix("/static/",http.FileServer(http.Dir("html/static"))))
        

        Port := DefaultPort
        if portenv := os.Getenv("WTFD_PORT"); portenv != "" {
          Port , _ = strconv.ParseInt(portenv, 10, 64)
        }
        return http.ListenAndServe(fmt.Sprintf(":%d",Port), r)
}
