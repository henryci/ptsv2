package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
)

func logMessage(context context.Context, msg string) {
	log.Printf("\x1b[31;1m"+msg+"\x1b[0m")
}

func logError(context context.Context, msg string, err error) {
	log.Printf("\x1b[31;1m%q Error: %v\x1b[0m", msg, err)
}

func logFatal(context context.Context, msg string, err error) {
	log.Fatalf("\x1b[31;1m%q Error: %v\x1b[0m", msg, err)
}

// Makes sure an ID is alphanumeric + underscore and (now) dash
func isValidID(testString string) bool {
	re := regexp.MustCompile("^[a-zA-Z0-9_-]*$")
	return re.MatchString(testString)
}

// Dumping a hash of string arrays is a repeated task so its factored out here
func dumpStringMap(w http.ResponseWriter, targetMap map[string][]string) {
	for k, v := range targetMap {
		str := k + ": "
		for i, s := range v {
			if i > 0 {
				str += ", "
			}
			str += s
		}
		fmt.Fprintln(w, str)
	}
}

// Used by templates to check if a field exists
func hasField(v interface{}, name string) bool {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	return rv.FieldByName(name).IsValid()
}

// Given an HTTP request, validates it against HTTP Auth.
// http://stackoverflow.com/questions/21936332/idiomatic-way-of-requiring-http-basic-auth-in-go
func checkAuth(w http.ResponseWriter, r *http.Request, username string, password string) bool {
	s := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(s) != 2 {
		return false
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		return false
	}

	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		return false
	}

	return pair[0] == username && pair[1] == password
}
