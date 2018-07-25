package ptsv2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"google.golang.org/appengine"
)

var templates = template.Must(template.New("").Funcs(template.FuncMap{"hasField": hasField}).ParseFiles(
	"templates/index.html",
	"templates/header.html",
	"templates/footer.html",
	"templates/toilet.html",
	"templates/sidebar.html",
	"templates/dump.html",
	"templates/static/whatisthis.html",
	"templates/static/somerules.html",
	"templates/static/howitworks.html",
	"templates/static/contact.html"))

func init() {
	router := mux.NewRouter()
	router.HandleFunc("/", rootHandler)
	router.HandleFunc("/s/{page}", staticHandler)
	router.HandleFunc("/t/{toiletID}", toiletHandler)
	router.HandleFunc("/t/{toiletID}/edit", toiletEditHandler)
	router.HandleFunc("/t/{toiletID}/post", postdumpHandler)
	router.HandleFunc("/t/{toiletID}/d/{dumpID}", viewdumpHandlerDEFAULT)
	router.HandleFunc("/t/{toiletID}/d/{dumpID}/json", viewdumpHandlerJSON)
	router.HandleFunc("/t/{toiletID}/d/{dumpID}/text", viewdumpHandlerTEXT)
	router.HandleFunc("/t/{toiletID}/d/{dumpID}/flush", deletedumpHandler)
	router.HandleFunc("/a", adminHandler)

	// Static files
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))

	http.Handle("/", router)
}

// Basic landing page
func rootHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "index.html", nil)
}

// All the static content pages (this is different from /static/ because this still gets rendered by the templates)
func staticHandler(w http.ResponseWriter, r *http.Request) {
	urlVars := mux.Vars(r)
	templates.ExecuteTemplate(w, urlVars["page"], nil)
}

// Admin and overview
func adminHandler(w http.ResponseWriter, r *http.Request) {
	context := appengine.NewContext(r)
	toilets, err := getDisabledToilets(context)

	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed getting clogged toilets", err)
		return
	}

	fmt.Fprintln(w, fmt.Sprintf("There are %d blocked toilets:", len(toilets)))
	for _, v := range toilets {
		fmt.Fprintln(w, v.ID)
	}
}

/*****************
 * TOILET HANDLING
 *****************/

// Views a toilet or presents the creation screen
func toiletHandler(w http.ResponseWriter, r *http.Request) {

	urlVars := mux.Vars(r)
	context := appengine.NewContext(r)

	toilet, err := getToilet(context, urlVars["toiletID"])
	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed getting toilet: "+urlVars["toiletID"], err)
		return
	}

	// If no toilet is found create one.
	if toilet == nil {
		toilet, err = createToilet(context, urlVars["toiletID"])
		if err != nil {
			errorHandler(w, r, http.StatusInternalServerError, "Failed creating toilet: "+urlVars["toiletID"], err)
			return
		}
	}

	if isBlockedToilet(toilet) {
		fmt.Fprintf(w, "Too much dumping has blocked this toilet.")
		return
	}

	// At this point, a toilet exists. Get it's dumps
	dumps, err := getToiletDumps(context, urlVars["toiletID"])
	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Unable to get dumps for toilet: "+urlVars["toiletID"], err)
		return
	}

	values := struct {
		Toilet *Toilet
		Dumps  []Dump
		Title  string
	}{
		toilet,
		dumps,
		"Toilet: " + toilet.ID,
	}

	// The toilet was found, display it
	templates.ExecuteTemplate(w, "toilet.html", values)
}

// Edits a toilet
func toiletEditHandler(w http.ResponseWriter, r *http.Request) {
	urlVars := mux.Vars(r)
	toiletID := urlVars["toiletID"]
	context := appengine.NewContext(r)

	toilet, err := getToilet(context, toiletID)
	if err != nil || toilet == nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed getting toilet for edit: "+urlVars["toiletID"], err)
		return
	}

	if err := r.ParseForm(); err != nil {
		errorHandler(w, r, http.StatusBadRequest, "Unable to parse form", err)
		return
	}

	var errors = ""

	responseDelay, err := strconv.ParseInt(r.Form["ResponseDelay"][0], 10, 0)
	if err != nil || responseDelay > int64(MaxResponseDelay) {
		errors += "Sever delay must be a valid integer between 0 and 5"
	}

	responseCode, err := strconv.ParseInt(r.Form["ResponseCode"][0], 10, 0)
	if err != nil || responseCode > 599 || responseCode < 99 {
		errors += "Status code must be a valid integer between 100 and 599"
	}

	if errors != "" {
		fmt.Fprintln(w, "Unable to update toilet because of errors: ")
		fmt.Fprintln(w, errors)
		return
	}

	// If there were no validation errors, update the Toilet with the new values
	toilet.AuthPassword = r.Form["AuthPassword"][0]
	toilet.AuthUsername = r.Form["AuthUsername"][0]
	toilet.ResponseDelay = int(responseDelay)
	toilet.ResponseCode = int(responseCode)
	toilet.ResponseBody = r.Form["ResponseBody"][0]

	// Store this toilet
	if _, err := updateToilet(context, toilet); err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed updating toilet", err)
		return
	}

	// Display the newly stored toilet
	http.Redirect(w, r, "/t/"+toiletID, http.StatusSeeOther)
}

// When an error occurs, log it and show a simple error message on screen
func errorHandler(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	w.WriteHeader(status)

	if err != nil {
		logError(appengine.NewContext(r), msg, err)
	}

	fmt.Fprintln(w, msg)
}

/****************
 * DUMP HANDLING
 ****************/

// Receives a post and dumps it.
// This is kind of the whole point of this project
func postdumpHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the URL
	urlVars := mux.Vars(r)
	context := appengine.NewContext(r)

	// Find the toilet, if it doesn't exist return a 404
	toilet, err := getToilet(context, urlVars["toiletID"])
	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed getting toilet: "+urlVars["toiletID"], err)
		return
	}
	if toilet == nil {
		errorHandler(w, r, http.StatusNotFound, "Toilet '"+urlVars["toiletID"]+"' not found", nil)
		return
	}

	if isBlockedToilet(toilet) {
		fmt.Fprintf(w, "Too much dumping has blocked this toilet.")
		return
	}

	// If the toilet has http Auth turned on, require it
	if toilet.AuthUsername != "" && toilet.AuthPassword != "" {
		if !checkAuth(w, r, toilet.AuthUsername, toilet.AuthPassword) {
			w.Header().Set("WWW-Authenticate", `Basic realm="PTSV2"`)
			w.WriteHeader(401)
			w.Write([]byte("401 Unauthorized\n"))
			return
		}
	}

	// Start storing information about the dump
	dump := new(Dump)
	dump.Timestamp = time.Now()
	dump.RemoteAddr = r.RemoteAddr

	// Handle the request method intelligently
	if r.Method == "POST" {
		dump.Method = "POST"
	} else if r.Method == "GET" {
		dump.Method = "GET"
	} else {
		errorHandler(w, r, http.StatusBadRequest, "Only GET and POST methods are supported", nil)
		return
	}

	// Capture all non Appengine headers
	for k, v := range r.Header {
		// Ignore anything added by appengine. These cause unnecessary confusion
		if strings.HasPrefix(strings.ToLower(k), "x-appengine") == false {
			dump.addHeader(k, v)
		}
	}

	// Grab the Post (or Get) parameters
	if err = r.ParseForm(); err != nil {
		errorHandler(w, r, http.StatusBadRequest, "Unable to parse form values", err)
		return
	}
	dump.FormValues = r.Form

	// Is there a file being uploaded?
	if isMultipart(r) {
		if err = r.ParseMultipartForm((1 << 10) * 24); nil != err {
			errorHandler(w, r, http.StatusInternalServerError, "Failed writing file", err)
			return
		}
		for _, fheaders := range r.MultipartForm.File {
			for _, hdr := range fheaders {

				var infile multipart.File
				if infile, err = hdr.Open(); nil != err {
					errorHandler(w, r, http.StatusInternalServerError, "Failed opening upload file", err)
					return
				}

				// TODO: MAKE A CONFIG SECTION
				buf := make([]byte, MaxFileSize) // 16K seems like a reasonable file limit
				_, err := infile.Read(buf)
				if err != nil && err != io.EOF {
					errorHandler(w, r, http.StatusInternalServerError, "Failed reading upload file", err)
					return
				}
				buf = bytes.Trim(buf, "\x00")
				dump.addFile(hdr.Filename, buf)
			}
		}
	}

	// Get the body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Couldn't read request body.", err)
		return
	}
	dump.Body = string(body)

	// Make sure there is room in this toilet.
	if deleteExtraDumps(context, toilet) != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed checking toilet size", err)
		return
	}

	// Store the dump
	dumpID, err := storeDump(context, dump, toilet)
	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed storing dump", err)
		return
	}

	// If there was a delay, wait for it. (never sleep more than 10 seconds)
	if toilet.ResponseDelay > 0 && toilet.ResponseDelay < MaxResponseDelay {
		time.Sleep(time.Duration(toilet.ResponseDelay*1000) * time.Millisecond)
	}

	// Set the Appropriate HTTP Response Code
	w.WriteHeader(toilet.ResponseCode)

	// Set the appropriate response body
	urlstr := fmt.Sprintf("http://%s/t/%s/d/%s", r.Host, toilet.ID, strconv.FormatInt(dumpID, 10))
	if toilet.ResponseBody == "{{LINK}}" {
		fmt.Fprintf(w, "Dump saved. View it <a href='"+urlstr+"'>here</a>")
	} else if toilet.ResponseBody == "{{URL-TEXT}}" {
		fmt.Fprintf(w, urlstr+"/text")
	} else if toilet.ResponseBody == "{{URL-JSON}}" {
		fmt.Fprintf(w, urlstr+"/json")
	} else {
		fmt.Fprintf(w, toilet.ResponseBody)
	}
}

// deletes (flushes) a specific dump
func deletedumpHandler(w http.ResponseWriter, r *http.Request) {
	urlVars := mux.Vars(r)
	toiletID := urlVars["toiletID"]
	context := appengine.NewContext(r)

	// Dump IDs are int64s
	dumpID, err := strconv.ParseInt(urlVars["dumpID"], 10, 64)
	if err != nil {
		errorHandler(w, r, http.StatusBadRequest, "Invalid Dump ID.", err)
		return
	}

	// This will succeed even if the dump doesn't exist to delete
	err = deleteDump(context, dumpID, toiletID)
	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed flushing Dump.", err)
		return
	}

	http.Redirect(w, r, "/t/"+toiletID, http.StatusSeeOther)
}

// Displays a dump in the basic format
func viewdumpHandlerDEFAULT(w http.ResponseWriter, r *http.Request) {
	viewdumpHandler(w, r, "")
}

// Displays a dump in the JSON format
func viewdumpHandlerJSON(w http.ResponseWriter, r *http.Request) {
	viewdumpHandler(w, r, "json")
}

// Displays a dump in the JSON format
func viewdumpHandlerTEXT(w http.ResponseWriter, r *http.Request) {
	viewdumpHandler(w, r, "text")
}

// /views a specific dump according to the outputMethod
func viewdumpHandler(w http.ResponseWriter, r *http.Request, outputMethod string) {
	urlVars := mux.Vars(r)
	toiletID := urlVars["toiletID"]
	context := appengine.NewContext(r)

	// Dump IDs are int64s
	dumpID, err := strconv.ParseInt(urlVars["dumpID"], 10, 64)
	if err != nil {
		errorHandler(w, r, http.StatusBadRequest, "Invalid Dump ID.", err)
		return
	}

	dump, err := getDump(context, dumpID, toiletID)
	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Failed getting Dump.", err)
		return
	}

	if dump == nil {
		fmt.Fprintf(w, "Dump not found")
		return
	}

	// Output the dump according to the requested method
	if outputMethod == "json" {
		outputDumpJSON(w, r, *dump)
		return
	}

	if outputMethod == "text" {
		outputDumpText(w, r, *dump)
		return
	}

	values := struct {
		ToiletID string
		Dump     *Dump
		Title    string
	}{
		toiletID,
		dump,
		"Dump View",
	}

	templates.ExecuteTemplate(w, "dump.html", values)
}

func outputDumpText(w http.ResponseWriter, r *http.Request, dump Dump) {
	// For some reason using Fprintln causes everything to print on the screen nicely
	// Using anytihng else causes the browser to attempt to download the url as a text file
	// I'm sure there is a super simple fix for this and I simply don't care :)
	fmt.Fprintln(w, "Details")
	fmt.Fprintln(w, "-------")
	fmt.Fprintln(w, "ID:", dump.ID)
	fmt.Fprintln(w, "Timestamp:", dump.Timestamp)
	fmt.Fprintln(w, "Method:", dump.Method)
	fmt.Fprintln(w, "IP:", dump.RemoteAddr)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Headers")
	fmt.Fprintln(w, "-------")
	dumpStringMap(w, dump.Headers)
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "Parameters")
	fmt.Fprintln(w, "----------")
	if len(dump.FormValues) == 0 {
		fmt.Fprintln(w, "No Parameters")
	} else {
		dumpStringMap(w, dump.FormValues)
	}
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "Body")
	fmt.Fprintln(w, "----")
	if len(dump.Body) > 0 {
		fmt.Fprintln(w, dump.Body)
	} else {
		fmt.Fprintln(w, "No body")
	}
	fmt.Fprintln(w, "")

	fmt.Fprintln(w, "Files")
	fmt.Fprintln(w, "-----")
	if len(dump.Files) > 0 {
		for i := 0; i < len(dump.Files); i++ {
			fmt.Fprintln(w, fmt.Sprintf("File: %d", i))
			fmt.Fprintln(w, fmt.Sprintf("filename: %s", dump.Files[i].Filename))
			fmt.Fprintln(w, fmt.Sprintf("SHA1: %s", dump.Files[i].SHA1))
			fmt.Fprintln(w, string(dump.Files[i].Content))
		}
	} else {
		fmt.Fprintln(w, "No files")
	}
}

func outputDumpJSON(w http.ResponseWriter, r *http.Request, dump Dump) {
	jsonString, err := json.Marshal(dump)
	if err != nil {
		errorHandler(w, r, http.StatusInternalServerError, "Couldn't convert Dump to JSON.", err)
		return
	}
	fmt.Fprintf(w, string(jsonString))
}
