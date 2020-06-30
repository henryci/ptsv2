package main // main package required in Go runtime 1.12+

import (
	"os"
	"net/http"
)

// main function required in Go runtime 1.12+
func main() {
	prepare()

	// [START setting_port]
	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
		// log.Printf("Defaulting to port %s", port)
	}

	// log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		//log.Fatal(err)
	}
	// [END setting_port]
}

