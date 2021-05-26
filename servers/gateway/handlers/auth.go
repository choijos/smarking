package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/choijos/assignments-choijos/servers/gateway/models/users"
	"github.com/choijos/assignments-choijos/servers/gateway/sessions"
)

//TODO: define HTTP handler functions as described in the
//assignment description. Remember to use your handler context
//struct as the receiver on these functions so that you have
//access to things like the session store and user store.
func (ctx *HandlerContext) UsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "request body must be in json", http.StatusUnsupportedMediaType)
			return

		}

		newUser := &users.NewUser{}
		body, _ := ioutil.ReadAll(r.Body)

		dec := json.NewDecoder(strings.NewReader(string(body)))
		if err := dec.Decode(newUser); err != nil {
			http.Error(w, "error decoding json", http.StatusBadRequest)
			return

		}

		toUser, err := newUser.ToUser()
		if err != nil {
			http.Error(w, fmt.Sprintf("cannot convert to user: %v", err), http.StatusBadRequest)
			return

		}

		// _, err = ctx.UserStore.GetByEmail(toUser.Email)
		// if err != nil {
		// 	http.Error(w, err.Error(), http.StatusBadRequest)
		// 	return

		// }

		// _, err = ctx.UserStore.GetByUserName(toUser.UserName)
		// if err != nil {
		// 	http.Error(w, err.Error(), http.StatusBadRequest)
		// 	return

		// }

		dbmsUser, err := ctx.UserStore.Insert(toUser)
		if err != nil {
			http.Error(w, fmt.Sprintf("%v %s", dbmsUser, err.Error()), http.StatusBadRequest)
			return

		}

		sess := SessionState{
			time.Now(),
			toUser,
		}

		_, err = sessions.BeginSession(ctx.SessKey, ctx.SessStore, sess, w)

		if err != nil {
			http.Error(w, fmt.Sprintf("error beginning session: %v", err), http.StatusBadRequest)
			return

		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		// if dbmsUser.ID < 1 {
		// 	http.Error(w, "user id property not set correctly", http.StatusInternalServerError)
		// 	return

		// }

		enc := json.NewEncoder(w)
		enc.Encode(dbmsUser)

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)

	}

}

func (ctx *HandlerContext) SpecificUserHandler(w http.ResponseWriter, r *http.Request) {
	sessID, err := sessions.GetSessionID(r, ctx.SessKey)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if r.Method == "GET" {
		profile := &users.User{}
		urlID := path.Base(r.URL.Path)

		if urlID == "me" {
			sess := SessionState{}
			err = ctx.SessStore.Get(sessID, &sess)
			if err != nil {
				http.Error(w, fmt.Sprintf("error getting session state: %v", err), http.StatusInternalServerError)
				return

			}

			profile = sess.AuthUser

		} else {
			userid, err := strconv.ParseInt(urlID, 10, 64)
			if err != nil {
				http.Error(w, fmt.Sprintf("error converting provided User ID from url to int64: %v", err), http.StatusNotAcceptable)
				return

			}

			profile, err = ctx.UserStore.GetByID(userid)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
				
			}

		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)
		enc.Encode(profile)

	} else if r.Method == "PATCH" {
		urlID := path.Base(r.URL.Path)

		sess := SessionState{}
		err = ctx.SessStore.Get(sessID, &sess)
		if err != nil {
			http.Error(w, fmt.Sprintf("error getting session state: %v", err), http.StatusInternalServerError)
			return
			
		}

		if urlID != "me" && urlID != strconv.FormatInt(sess.AuthUser.ID, 10) {
			http.Error(w, "request URL id no current user", http.StatusForbidden)
			return

		}

		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "request body must be in json", http.StatusUnsupportedMediaType)
			return

		}

		upd := &users.Updates{}
		body, _ := ioutil.ReadAll(r.Body)

		dec := json.NewDecoder(strings.NewReader(string(body)))
		if err := dec.Decode(upd); err != nil {
			http.Error(w, "error decoding json", http.StatusBadRequest)
			return

		}

		err = sess.AuthUser.ApplyUpdates(upd)
		if err != nil {
			http.Error(w, fmt.Sprintf("error applying updates to current user: %v", err), http.StatusInternalServerError)
			return

		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")

		currUser, err := ctx.UserStore.GetByID(sess.AuthUser.ID)
		if err != nil {
			http.Error(w, fmt.Sprintf("error getting user with id: %v", err), http.StatusInternalServerError)
			return

		}

		enc := json.NewEncoder(w)
		enc.Encode(currUser)

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)

	}

}

func (ctx *HandlerContext) SessionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "request body must be in json", http.StatusUnsupportedMediaType)
			return

		}

		userCreds := &users.Credentials{}
		body, _ := ioutil.ReadAll(r.Body)

		dec := json.NewDecoder(strings.NewReader(string(body)))
		if err := dec.Decode(userCreds); err != nil {
			http.Error(w, "error decoding json for credentials", http.StatusBadRequest)
			return

		}

		usr, err := ctx.UserStore.GetByEmail(userCreds.Email)
		if err != nil {
			time.Sleep(5 * time.Second)
			if err == users.ErrUserNotFound {
				http.Error(w, "invalid credentials", http.StatusUnauthorized)
				return

			}
			
			http.Error(w, fmt.Sprintf("error getting user with given email and credentials: %v", err), http.StatusInternalServerError)
			return

		}

		err = usr.Authenticate(userCreds.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return

		}

		ip := ""
		if r.Header.Get("X-Forwarded-For") != "" {
			ips := strings.Split(r.Header.Get("X-Forwarded-For"), ", ")
			ip = ips[0]

		} else {
			ip = r.RemoteAddr

		}

		timeNow := time.Now()

		err = ctx.UserStore.InsertSignIn(usr.ID, timeNow, ip)
		if err != nil || len(ip) == 0 {
			http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
			return
		}

		err = usr.Authenticate(userCreds.Password)
		if err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return

		}

		log.Printf("User %d attempted to log in at %v", usr.ID, timeNow)

		sess := SessionState{
			time.Now(),
			usr,
		}

		_, err = sessions.BeginSession(ctx.SessKey, ctx.SessStore, sess, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("cannot being new session: %v", err), http.StatusInternalServerError)
			return

		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		enc := json.NewEncoder(w)
		enc.Encode(usr)

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)

	}

}

func (ctx *HandlerContext) SpecificSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		if path.Base(r.URL.Path) != "mine" {
			http.Error(w, "path not 'mine', inappropriate path", http.StatusForbidden)
			return

		}

		_, err := sessions.EndSession(r, ctx.SessKey, ctx.SessStore)
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), http.StatusInternalServerError)
			return

		}

		w.Write([]byte("signed out"))

	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)

	}

}
